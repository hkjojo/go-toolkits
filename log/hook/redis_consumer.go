package hook

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Entry is a parsed log entry consumed from a Redis stream populated by
// RedisStreamCore. It mirrors the producer-side XADD shape (per ADR-0042).
//
// StreamID is the Redis-assigned entry id (e.g., "1746523648123-0"). It is
// not currently used for idempotency at the schema level (system_log uses
// at-least-once semantics, INV-SL-003) but is exposed in case savers want
// to dedup in memory or upgrade to exactly-once at their own discretion.
type Entry struct {
	StreamID string
	Ts       time.Time
	Service  string
	Source   string
	Level    string
	Msg      string
	Payload  map[string]any
	Host     string
}

// SaverFunc is the consumer-side callback invoked once per batch read from
// the stream. The saver is responsible for persisting the batch (e.g.,
// bulk-INSERT into a PG table). Errors are logged and the batch is NOT
// XACK'd, so the entries remain in PEL and will be reclaimed by the next
// XAUTOCLAIM cycle.
//
// Under AckBySaver (default) a nil return XACKs the whole batch immediately.
// Under AckByHandler a nil return only means "accepted into the handler's
// custody" — the handler must call RedisStreamConsumer.Ack with the
// Entry.StreamID values once they are durable.
type SaverFunc func(ctx context.Context, batch []Entry) error

// AckMode selects who acknowledges entries after the saver accepts them.
type AckMode int

const (
	// AckBySaver (default): the consumer XACKs a batch as soon as the saver
	// returns nil. Right when the saver itself is the durable sink (a
	// synchronous PG INSERT).
	AckBySaver AckMode = iota

	// AckByHandler: the consumer never XACKs on its own; the handler calls
	// Ack(ctx, ids...) once the entries are durable. Right when the saver
	// only buffers (e.g. batching into a file flushed every N seconds) — the
	// Redis PEL then acts as the write-ahead log: entries accepted but not
	// yet durable stay pending and are redelivered by XAUTOCLAIM after
	// MinIdleTime if the process dies before acking. Set MinIdleTime
	// comfortably above the handler's flush cadence, otherwise healthy
	// in-flight entries get reclaimed and delivered twice.
	AckByHandler
)

// ConsumerOpts configures runtime behavior of RedisStreamConsumer. Static
// fields (StreamKey, redis connection) are pulled from the producer-side
// RedisStreamConfig so producer and consumer share a single source of truth
// for the stream contract.
type ConsumerOpts struct {
	GroupName  string // required; e.g., "system-log-persister"
	ConsumerID string // required, must be per-instance unique; e.g., "apiserver-{HOSTNAME}"

	// BatchSize is the COUNT argument to XReadGroup. Default 100.
	BatchSize int

	// FlushMs is the BLOCK timeout (ms) for XReadGroup. The consumer wakes
	// up at this cadence even when no entries are available, so callbacks
	// like context cancellation are honored promptly. Default 500.
	FlushMs int

	// MinIdleTime is the threshold for XAUTOCLAIM — entries pending in PEL
	// longer than this duration are reclaimed by other consumers. Default 30s.
	MinIdleTime time.Duration

	// AutoClaimInterval controls how often the consumer scans for stale PEL
	// entries to reclaim. Default 10s.
	AutoClaimInterval time.Duration

	// IdleConsumerTTL is the threshold for XGROUP DELCONSUMER — consumers
	// idle longer than this are removed from the group (cleanup of pods
	// that scaled down or restarted with a new ConsumerID). Default 10min.
	IdleConsumerTTL time.Duration

	// ConsumerCleanupInterval controls how often the offline-consumer
	// cleanup goroutine runs. Default 5min.
	ConsumerCleanupInterval time.Duration

	// AckMode decides who XACKs accepted entries. Default AckBySaver.
	AckMode AckMode
}

// RedisStreamConsumer reads log entries written by RedisStreamCore from a
// Redis stream, parses them into typed Entry, calls a user-provided
// SaverFunc for each batch, and XACKs on success (AckBySaver) or leaves the
// XACK to the handler (AckByHandler, see Ack).
//
// Three goroutines run concurrently after Start:
//  1. main loop — XReadGroup + saver + XACK
//  2. autoclaim loop — periodic XAUTOCLAIM for stale PEL entries
//  3. cleanup loop — periodic XGROUP DELCONSUMER for idle consumers
//
// All three honor context cancellation from Start and exit cleanly on Stop.
type RedisStreamConsumer struct {
	streamCfg *RedisStreamConfig
	opts      ConsumerOpts
	saver     SaverFunc

	client redis.UniversalClient

	mu      sync.Mutex
	cancel  context.CancelFunc
	stopped chan struct{}
	wg      sync.WaitGroup
}

// NewRedisStreamConsumer constructs a consumer. The redis client is built
// internally from streamCfg (matching producer-side ownership) so consumer
// and producer share the same connection contract — connect to the same
// addr/db/auth as wherever the producer XADDs.
func NewRedisStreamConsumer(streamCfg *RedisStreamConfig, opts ConsumerOpts, saver SaverFunc) (*RedisStreamConsumer, error) {
	if streamCfg == nil {
		return nil, errors.New("redis stream consumer: nil stream config")
	}
	if streamCfg.Addr == "" {
		return nil, errors.New("redis stream consumer: addr is required")
	}
	if opts.GroupName == "" {
		return nil, errors.New("redis stream consumer: group name is required")
	}
	if opts.ConsumerID == "" {
		return nil, errors.New("redis stream consumer: consumer id is required")
	}
	if saver == nil {
		return nil, errors.New("redis stream consumer: saver is required")
	}
	if opts.AckMode != AckBySaver && opts.AckMode != AckByHandler {
		return nil, fmt.Errorf("redis stream consumer: unknown ack mode %d", opts.AckMode)
	}

	applyRedisStreamDefaults(streamCfg)
	applyConsumerOptDefaults(&opts)

	clientOpts := &redis.UniversalOptions{
		Addrs:    []string{streamCfg.Addr},
		Password: streamCfg.Password,
		DB:       streamCfg.DB,
	}
	if streamCfg.UseTLS {
		clientOpts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	c := &RedisStreamConsumer{
		streamCfg: streamCfg,
		opts:      opts,
		saver:     saver,
		client:    redis.NewUniversalClient(clientOpts),
		stopped:   make(chan struct{}),
	}
	return c, nil
}

// Start launches the three internal goroutines. The provided ctx is the
// parent for all of them; cancelling ctx (or calling Stop) terminates the
// consumer.
//
// Start is idempotent in the sense that repeated calls return the prior
// error if construction succeeded but startup failed; concurrent calls are
// not supported.
//
// XGroupCreateMkStream is invoked once at startup with idempotent semantics
// (BUSYGROUP error is treated as success — the group already exists from a
// previous run). The starting ID is "$" so this consumer instance only sees
// new entries — historical entries already ACK'd by a prior consumer run
// are not re-fed. (XAUTOCLAIM still picks up unacked entries from the PEL.)
func (c *RedisStreamConsumer) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		return errors.New("redis stream consumer: already started")
	}

	if err := c.ensureGroup(ctx); err != nil {
		return fmt.Errorf("redis stream consumer: ensure group: %w", err)
	}

	loopCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(3)
	go c.runMainLoop(loopCtx)
	go c.runAutoClaimLoop(loopCtx)
	go c.runConsumerCleanupLoop(loopCtx)

	go func() {
		c.wg.Wait()
		close(c.stopped)
	}()

	return nil
}

// Stop signals all goroutines to exit, waits for them to drain, and closes
// the underlying redis client. Safe to call once; subsequent calls return
// nil immediately.
func (c *RedisStreamConsumer) Stop() error {
	c.mu.Lock()
	if c.cancel == nil {
		c.mu.Unlock()
		return nil
	}
	cancel := c.cancel
	c.cancel = nil
	c.mu.Unlock()

	cancel()
	<-c.stopped

	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// Ack acknowledges entries by stream id on behalf of the handler. It is the
// only way entries get XACK'd under AckByHandler; under AckBySaver it is a
// redundant no-harm call (already-acked ids are ignored by Redis).
//
// Call it only after the entries are durable in the handler's sink. An
// error means the XACK did not happen: the ids stay in the PEL and will be
// redelivered via XAUTOCLAIM, so the handler must tolerate duplicates
// (at-least-once) — it must NOT retry blindly in a loop. Ack is safe to
// call concurrently with the consumer loops and after Stop (it then fails
// with the closed-client error).
func (c *RedisStreamConsumer) Ack(ctx context.Context, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	return c.client.XAck(ctx, c.streamCfg.StreamKey, c.opts.GroupName, ids...).Err()
}

func (c *RedisStreamConsumer) ensureGroup(ctx context.Context) error {
	err := c.client.XGroupCreateMkStream(ctx, c.streamCfg.StreamKey, c.opts.GroupName, "$").Err()
	if err == nil {
		return nil
	}
	// BUSYGROUP signals "group already exists" — idempotent success.
	if isBusyGroup(err) {
		return nil
	}
	return err
}

func (c *RedisStreamConsumer) runMainLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		if ctx.Err() != nil {
			return
		}

		res, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.opts.GroupName,
			Consumer: c.opts.ConsumerID,
			Streams:  []string{c.streamCfg.StreamKey, ">"},
			Count:    int64(c.opts.BatchSize),
			Block:    time.Duration(c.opts.FlushMs) * time.Millisecond,
		}).Result()

		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			fmt.Fprintf(os.Stderr,
				"[log] redis stream consumer xreadgroup err: %v\n", err)
			// Brief sleep so a persistent error (auth failure, server down)
			// doesn't hot-loop the CPU.
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}

		c.handleStreams(ctx, res)
	}
}

func (c *RedisStreamConsumer) handleStreams(ctx context.Context, streams []redis.XStream) {
	for _, stream := range streams {
		if len(stream.Messages) == 0 {
			continue
		}
		batch := make([]Entry, 0, len(stream.Messages))
		ids := make([]string, 0, len(stream.Messages))
		for _, msg := range stream.Messages {
			entry, err := parseEntry(msg)
			if err != nil {
				// Malformed entries are dropped (NOT ACK'd intentionally —
				// they will reach PEL and need manual intervention). A
				// single bad entry should not block the rest of the batch.
				fmt.Fprintf(os.Stderr,
					"[log] redis stream consumer parse err id=%s: %v\n", msg.ID, err)
				continue
			}
			batch = append(batch, entry)
			ids = append(ids, msg.ID)
		}
		if len(batch) == 0 {
			continue
		}
		if err := c.saver(ctx, batch); err != nil {
			fmt.Fprintf(os.Stderr,
				"[log] redis stream consumer saver err: %v (batch=%d, leaving in PEL)\n",
				err, len(batch))
			continue // leave in PEL for autoclaim
		}
		if c.opts.AckMode == AckByHandler {
			// Handler owns durability; it acks via Ack() once the entries
			// are persisted. Until then the PEL is the WAL.
			continue
		}
		if err := c.client.XAck(ctx, c.streamCfg.StreamKey, c.opts.GroupName, ids...).Err(); err != nil {
			// XAck failure is non-fatal — entries will be redelivered via
			// XAUTOCLAIM and the saver will land them again (at-least-once,
			// per ADR-0042 §INV-SL-003).
			fmt.Fprintf(os.Stderr,
				"[log] redis stream consumer xack err: %v\n", err)
		}
	}
}

func (c *RedisStreamConsumer) runAutoClaimLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.opts.AutoClaimInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.runAutoClaimOnce(ctx)
		}
	}
}

func (c *RedisStreamConsumer) runAutoClaimOnce(ctx context.Context) {
	start := "0-0"
	for {
		if ctx.Err() != nil {
			return
		}
		messages, next, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   c.streamCfg.StreamKey,
			Group:    c.opts.GroupName,
			Consumer: c.opts.ConsumerID,
			MinIdle:  c.opts.MinIdleTime,
			Start:    start,
			Count:    int64(c.opts.BatchSize),
		}).Result()
		if err != nil {
			if !errors.Is(err, redis.Nil) {
				fmt.Fprintf(os.Stderr,
					"[log] redis stream consumer xautoclaim err: %v\n", err)
			}
			return
		}
		if len(messages) > 0 {
			c.handleStreams(ctx, []redis.XStream{{
				Stream:   c.streamCfg.StreamKey,
				Messages: messages,
			}})
		}
		// Continue scanning until next == "0-0" (XAutoClaim's end-of-PEL signal).
		if next == "0-0" {
			return
		}
		start = next
	}
}

func (c *RedisStreamConsumer) runConsumerCleanupLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.opts.ConsumerCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.runConsumerCleanupOnce(ctx)
		}
	}
}

func (c *RedisStreamConsumer) runConsumerCleanupOnce(ctx context.Context) {
	consumers, err := c.client.XInfoConsumers(ctx, c.streamCfg.StreamKey, c.opts.GroupName).Result()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"[log] redis stream consumer xinfo err: %v\n", err)
		return
	}
	for _, info := range consumers {
		// Skip self — we are not idle (we're the one running the cleanup).
		if info.Name == c.opts.ConsumerID {
			continue
		}
		if info.Idle < c.opts.IdleConsumerTTL {
			continue
		}
		// Delete consumers that are both idle AND have no pending entries.
		// Keeping idle consumers with pending entries lets XAUTOCLAIM
		// recover them; deleting would lose those PEL records.
		if info.Pending > 0 {
			continue
		}
		if err := c.client.XGroupDelConsumer(ctx, c.streamCfg.StreamKey, c.opts.GroupName, info.Name).Err(); err != nil {
			fmt.Fprintf(os.Stderr,
				"[log] redis stream consumer xgroup delconsumer err name=%s: %v\n", info.Name, err)
		}
	}
}

// parseEntry converts a Redis stream message (raw map[string]any from
// go-redis) into a typed Entry. Mirrors the producer-side serialization in
// RedisStreamCore.writeData — any drift between these two functions breaks
// the system_log pipeline.
func parseEntry(msg redis.XMessage) (Entry, error) {
	e := Entry{StreamID: msg.ID}

	if v, ok := msg.Values["ts"].(string); ok && v != "" {
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return Entry{}, fmt.Errorf("ts parse: %w", err)
		}
		e.Ts = time.UnixMilli(ms)
	}
	e.Level, _ = msg.Values["level"].(string)
	e.Service, _ = msg.Values["service"].(string)
	e.Source, _ = msg.Values["source"].(string)
	e.Msg, _ = msg.Values["msg"].(string)
	e.Host, _ = msg.Values["host"].(string)

	if pl, ok := msg.Values["payload"].(string); ok && pl != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(pl), &m); err == nil {
			e.Payload = m
		}
		// JSON decode failures are tolerated — Payload stays nil. Common
		// case is the truncation marker `{"_truncated":true}` which decodes
		// fine; cyclic / corrupt JSON would be observed via missing Payload
		// and the saver can decide how to handle.
	}
	return e, nil
}

func applyConsumerOptDefaults(opts *ConsumerOpts) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}
	if opts.FlushMs <= 0 {
		opts.FlushMs = 500
	}
	if opts.MinIdleTime <= 0 {
		opts.MinIdleTime = 30 * time.Second
	}
	if opts.AutoClaimInterval <= 0 {
		opts.AutoClaimInterval = 10 * time.Second
	}
	if opts.IdleConsumerTTL <= 0 {
		opts.IdleConsumerTTL = 10 * time.Minute
	}
	if opts.ConsumerCleanupInterval <= 0 {
		opts.ConsumerCleanupInterval = 5 * time.Minute
	}
}

func isBusyGroup(err error) bool {
	if err == nil {
		return false
	}
	// go-redis returns a string error for BUSYGROUP; match by substring to
	// avoid coupling to the redis package's internal error types.
	return contains(err.Error(), "BUSYGROUP")
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
