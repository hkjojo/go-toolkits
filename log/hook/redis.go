package hook

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// RedisStreamConfig configures a zap core that XADDs each log entry into a
// Redis stream. Used for cross-service runtime event aggregation (system_log).
//
// The XADD entry shape is fixed per the ADR-0042 contract — consumer side
// (NewRedisStreamConsumer / RedisStreamConsumer) parses these exact fields:
//
//	ts        unix milliseconds (string)
//	level     zap raw level string (debug/info/warn/error/dpanic/panic/fatal)
//	service   process identity (from CoreConfig.Fields["service"])
//	source    sub-component name (from log.With("source", ...) sub-logger)
//	msg       zap entry.Message (machine-readable, snake_case)
//	payload   remaining structured fields, JSON-encoded
//	host      os.Hostname() (auto-injected at core construction; multi-replica dedup)
type RedisStreamConfig struct {
	CoreConfig `json:",inline"`

	// Redis connection — hkjojo internally builds a redis.UniversalClient from these.
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	UseTLS   bool   `json:"use_tls"`

	// Stream behavior.
	StreamKey       string `json:"stream_key"`        // default "system_logs"
	MaxLen          int64  `json:"max_len"`           // default 200_000 (approximate)
	BatchSize       int    `json:"batch_size"`        // reserved for future batching; current impl is single-write
	FlushMs         int    `json:"flush_ms"`          // reserved for future batching
	QPS             int    `json:"qps"`               // 0 = unlimited; default 50
	MaxPayloadBytes int    `json:"max_payload_bytes"` // default 4096; oversized payload replaced with {"_truncated":true}
}

// RedisStreamCore is a zapcore.Core that ships log entries to a Redis stream.
//
// It composes hkjojo BaseCore (queue / filter / fields / level / off) and
// implements the writeData callback to serialize a CoreData into an XADD.
// The redis.UniversalClient is built internally from RedisStreamConfig and is
// owned by this core (independent from any business-side redis client).
//
// Failure modes (matching hook/kafka.go semantics — non-blocking, log-and-drop):
//   - Constructor: redis.Ping failure prints to stderr but does not error;
//     the core continues and individual XADDs may also fail at write time.
//   - writeData: XADD failure prints to stderr; the entry is dropped.
//     stdout/file paths are unaffected (zap tee runs cores independently).
//   - Producer storm: token bucket (RedisStreamConfig.QPS) drops over-budget
//     entries and increments droppedTotal — observable via DroppedTotal().
type RedisStreamCore struct {
	*BaseCore

	cfg    *RedisStreamConfig
	client redis.UniversalClient
	host   string

	// Token bucket state — atomic to avoid taking a lock in writeData hot path.
	bucketTokens   atomic.Int64
	bucketLastRefill atomic.Int64 // unix nanoseconds
	bucketCap      int64
	droppedTotal   atomic.Uint64
}

// NewRedisStreamCore constructs a RedisStreamCore. The returned core is ready
// to be tee'd onto a zap.Logger via zapcore.NewTee, or appended to the cores
// list inside tklog.New (which RedisStreamConfig is wired into via the main
// Config.Redis field).
//
// Defaults applied for unset fields:
//   - StreamKey:       "system_logs"
//   - MaxLen:          200_000
//   - QPS:             50 (0 explicitly disables throttle)
//   - MaxPayloadBytes: 4096
//   - QueueLength:     1024 (BaseCore async buffer)
//
// `prefix` and `fields` follow the same convention as NewKafkaCore — fields
// are merged into BaseCore.fields and emitted as always-on tags on every
// entry. The "service" tag is read from this map at writeData time.
func NewRedisStreamCore(cfg *RedisStreamConfig, prefix string, fields map[string]string, encode zapcore.EncoderConfig) (*RedisStreamCore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis stream core: nil config")
	}
	if cfg.Addr == "" {
		return nil, fmt.Errorf("redis stream core: addr is required")
	}

	applyRedisStreamDefaults(cfg)

	host, _ := os.Hostname()

	core := &RedisStreamCore{
		BaseCore: &BaseCore{
			queue:        make(chan *CoreData, cfg.QueueLength),
			LevelEnabler: zap.NewAtomicLevelAt(ParseLevel(cfg.Level)),
			enc:          zapcore.NewJSONEncoder(encode),
			out:          zapcore.AddSync(io.Discard),
			filters:      getfilters(cfg.Filter),
			fields:       CombineFields(fields, cfg.Fields),
			off:          cfg.Off,
		},
		cfg:       cfg,
		host:      host,
		bucketCap: int64(cfg.QPS),
	}
	core.BaseCore.core = core

	if cfg.QPS > 0 {
		core.bucketTokens.Store(int64(cfg.QPS))
		core.bucketLastRefill.Store(time.Now().UnixNano())
	}

	opts := &redis.UniversalOptions{
		Addrs:    []string{cfg.Addr},
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	if cfg.UseTLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	core.client = redis.NewUniversalClient(opts)

	// Best-effort startup ping. Do not fail core construction — core continues
	// to the goroutine and writeData errors will print to stderr at runtime.
	// This matches hook/kafka.go semantics where producer construction errors
	// also degrade to stderr without aborting the logger.
	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := core.client.Ping(pingCtx).Err(); err != nil {
		fmt.Fprintf(os.Stderr,
			"[log] redis stream core: ping err (sink may be unavailable): %v\n", err)
	}

	core.start()
	return core, nil
}

// Close releases the underlying redis client. Use during process shutdown
// after the BaseCore queue has drained.
func (c *RedisStreamCore) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// DroppedTotal returns the cumulative count of entries dropped by the token
// bucket (over-QPS) since core construction. Useful for the
// system_log_dropped_total metric (consumer can scrape via callback).
func (c *RedisStreamCore) DroppedTotal() uint64 {
	return c.droppedTotal.Load()
}

// writeData runs in the BaseCore goroutine (single consumer of c.queue).
// It serializes the entry per the ADR-0042 contract and XADDs to the stream.
// On QPS overrun, it drops the entry and increments droppedTotal — this is
// the only loss path under healthy redis (the rest is reactive to redis
// failures, observable in stderr).
func (c *RedisStreamCore) writeData(data *CoreData) {
	if !c.allow() {
		c.droppedTotal.Add(1)
		return
	}

	// Extract source from fields (zap.With("source", ...) sub-logger derives
	// add this field). Hoist to top-level XADD k/v so the consumer can use
	// it as a primary filter without parsing the payload JSON.
	source := ""
	remaining := make([]zapcore.Field, 0, len(data.fields))
	for _, f := range data.fields {
		if f.Key == "source" {
			source = c.getFieldString(f)
			continue
		}
		remaining = append(remaining, f)
	}

	// Reduce remaining fields into a typed map → JSON. getField preserves
	// Go-side types (int → int64, float → float64, etc.) so consumers see
	// unmarshaled JSON numbers, not stringified ints.
	payloadMap := make(map[string]any, len(remaining))
	for _, f := range remaining {
		payloadMap[f.Key] = c.getField(f)
	}
	payloadJSON, err := json.Marshal(payloadMap)
	if err != nil {
		// JSON encoding failure is rare (only with cyclic graphs / chan / func types).
		// Replace with truncation marker — keeping the entry in the stream is more
		// useful than dropping silently.
		payloadJSON = []byte(`{"_encode_error":true}`)
	}

	// Hard cap on payload size — defends against accidental request-body or
	// orderbook dumps in log fields. Replaced (not truncated) because partial
	// JSON is invalid and would fail consumer-side unmarshal.
	if c.cfg.MaxPayloadBytes > 0 && len(payloadJSON) > c.cfg.MaxPayloadBytes {
		payloadJSON = []byte(`{"_truncated":true}`)
	}

	service := ""
	if c.fields != nil {
		service = c.fields["service"]
	}

	values := map[string]any{
		"ts":      strconv.FormatInt(data.entry.Time.UnixMilli(), 10),
		"level":   data.entry.Level.String(),
		"service": service,
		"source":  source,
		"msg":     data.entry.Message,
		"payload": string(payloadJSON),
		"host":    c.host,
	}

	if err := c.client.XAdd(context.Background(), &redis.XAddArgs{
		Stream: c.cfg.StreamKey,
		MaxLen: c.cfg.MaxLen,
		Approx: true, // MAXLEN ~ N keeps trim O(1)
		Values: values,
	}).Err(); err != nil {
		fmt.Fprintf(os.Stderr,
			"[log] redis stream xadd err: %v\n", err)
	}
}

// allow consumes one token from the QPS bucket. Returns true if granted,
// false if over budget. Refills lazily based on elapsed nanoseconds — no
// background ticker, no lock contention beyond two atomic CAS-equivalents.
//
// QPS=0 disables throttling entirely (always returns true).
func (c *RedisStreamCore) allow() bool {
	if c.cfg.QPS <= 0 {
		return true
	}

	now := time.Now().UnixNano()
	last := c.bucketLastRefill.Load()
	elapsed := now - last

	// Compute tokens to refill since last call. One second = full QPS budget.
	if elapsed > 0 {
		refill := int64(math.Floor(float64(elapsed) * float64(c.bucketCap) / float64(time.Second)))
		if refill > 0 && c.bucketLastRefill.CompareAndSwap(last, now) {
			// Add refill but cap at bucketCap so a long idle window doesn't
			// allow a single burst > QPS.
			tokens := c.bucketTokens.Add(refill)
			if tokens > c.bucketCap {
				c.bucketTokens.Store(c.bucketCap)
			}
		}
	}

	// Try to consume one token.
	for {
		current := c.bucketTokens.Load()
		if current <= 0 {
			return false
		}
		if c.bucketTokens.CompareAndSwap(current, current-1) {
			return true
		}
	}
}

func applyRedisStreamDefaults(cfg *RedisStreamConfig) {
	if cfg.StreamKey == "" {
		cfg.StreamKey = "system_logs"
	}
	if cfg.MaxLen <= 0 {
		cfg.MaxLen = 200_000
	}
	if cfg.QueueLength == 0 {
		cfg.QueueLength = 1024
	}
	if cfg.QPS == 0 {
		cfg.QPS = 50
	}
	if cfg.MaxPayloadBytes <= 0 {
		cfg.MaxPayloadBytes = 4096
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushMs <= 0 {
		cfg.FlushMs = 500
	}
}
