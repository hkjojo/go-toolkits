package hook

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// TestRedisStreamConsumer_ParseAndAck end-to-end: producer XADDs entries via
// the same on-wire format used by RedisStreamCore, the consumer reads, parses
// into Entry, calls saver, and XACKs. Validates the producer/consumer
// contract holds — any wire-format drift between sides breaks this.
func TestRedisStreamConsumer_ParseAndAck(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	streamKey := "system_logs"

	// Producer: write 3 entries directly via go-redis matching the
	// RedisStreamCore.writeData on-wire format.
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		payload, _ := json.Marshal(map[string]any{
			"lp":           "prime",
			"downtime_sec": i * 10,
		})
		_, err := rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: streamKey,
			Values: map[string]any{
				"ts":      strconv.FormatInt(now.Add(time.Duration(i)*time.Second).UnixMilli(), 10),
				"level":   "info",
				"service": "maker-adapter",
				"source":  "lp-gateway",
				"msg":     "connection_restored",
				"payload": string(payload),
				"host":    "maker-adapter-pod-1",
			},
		}).Result()
		if err != nil {
			t.Fatalf("producer xadd: %v", err)
		}
	}

	// Saver: capture each batch into a slice for assertions.
	var (
		mu      sync.Mutex
		batches [][]Entry
		seen    = make(chan struct{}, 8)
	)
	saver := func(_ context.Context, batch []Entry) error {
		mu.Lock()
		batches = append(batches, batch)
		mu.Unlock()
		seen <- struct{}{}
		return nil
	}

	consumer, err := NewRedisStreamConsumer(&RedisStreamConfig{
		Addr:      mr.Addr(),
		StreamKey: streamKey,
	}, ConsumerOpts{
		GroupName:  "system-log-persister",
		ConsumerID: "test-consumer-1",
		BatchSize:  10,
		FlushMs:    50,
	}, saver)
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}

	consumerCtx, consumerCancel := context.WithCancel(ctx)
	if err := consumer.Start(consumerCtx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// XReadGroup with start "$" only sees NEW entries — but our test wrote
	// before Start, so they're "old" relative to the group offset and will
	// not appear in XReadGroup. Trigger XAUTOCLAIM by writing a fresh entry
	// post-Start and waiting.
	//
	// Cleaner: re-emit AFTER Start.
	_, _ = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{
			"ts":      strconv.FormatInt(now.UnixMilli(), 10),
			"level":   "warn",
			"service": "decision-engine",
			"source":  "risk-engine",
			"msg":     "extreme_event_triggered",
			"payload": `{"symbol":"XAUUSD"}`,
			"host":    "decision-engine-pod-1",
		},
	}).Result()

	select {
	case <-seen:
	case <-time.After(2 * time.Second):
		consumerCancel()
		t.Fatalf("saver did not receive batch within 2s")
	}

	consumerCancel()
	if err := consumer.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(batches) == 0 {
		t.Fatalf("no batches captured")
	}
	got := batches[0]
	if len(got) == 0 {
		t.Fatalf("first batch was empty")
	}
	first := got[0]
	if first.Service != "decision-engine" {
		t.Errorf("service=%q, want decision-engine", first.Service)
	}
	if first.Source != "risk-engine" {
		t.Errorf("source=%q, want risk-engine", first.Source)
	}
	if first.Level != "warn" {
		t.Errorf("level=%q, want warn", first.Level)
	}
	if first.Msg != "extreme_event_triggered" {
		t.Errorf("msg=%q, want extreme_event_triggered", first.Msg)
	}
	if first.Host != "decision-engine-pod-1" {
		t.Errorf("host=%q, want decision-engine-pod-1", first.Host)
	}
	if first.StreamID == "" {
		t.Errorf("StreamID should be populated")
	}
	if first.Payload["symbol"] != "XAUUSD" {
		t.Errorf("payload.symbol=%v, want XAUUSD", first.Payload["symbol"])
	}
}

// TestRedisStreamConsumer_BadEntryDoesNotBlockBatch validates that a single
// malformed entry doesn't poison the batch — well-formed siblings still
// reach the saver.
func TestRedisStreamConsumer_BadEntryDoesNotBlockBatch(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	streamKey := "system_logs"

	var saverCount int
	var mu sync.Mutex
	gotEntry := make(chan struct{}, 1)

	consumer, err := NewRedisStreamConsumer(&RedisStreamConfig{
		Addr:      mr.Addr(),
		StreamKey: streamKey,
	}, ConsumerOpts{
		GroupName:  "test-group",
		ConsumerID: "test-consumer-2",
		BatchSize:  10,
		FlushMs:    50,
	}, func(_ context.Context, batch []Entry) error {
		mu.Lock()
		saverCount += len(batch)
		mu.Unlock()
		select {
		case gotEntry <- struct{}{}:
		default:
		}
		return nil
	})
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = consumer.Stop() }()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	// Mix a malformed (bad ts) entry with a good one.
	_, _ = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{
			"ts":      "not-a-number",
			"level":   "info",
			"msg":     "bad",
			"payload": `{}`,
		},
	}).Result()
	_, _ = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{
			"ts":      strconv.FormatInt(time.Now().UnixMilli(), 10),
			"level":   "info",
			"service": "api-server",
			"msg":     "good",
			"payload": `{}`,
			"host":    "h1",
		},
	}).Result()

	select {
	case <-gotEntry:
	case <-time.After(2 * time.Second):
		t.Fatalf("saver did not receive entry within 2s")
	}

	mu.Lock()
	if saverCount < 1 {
		t.Errorf("saver should see at least 1 valid entry, got %d", saverCount)
	}
	mu.Unlock()
}

// TestRedisStreamConsumer_AckByHandler_PELUntilAck: under AckByHandler the
// consumer must not XACK on saver success — entries stay pending until the
// handler calls Ack with their stream ids, after which the PEL is empty.
func TestRedisStreamConsumer_AckByHandler_PELUntilAck(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	const streamKey, group = "system_logs", "system-log-store"
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	var (
		mu  sync.Mutex
		ids []string
		got = make(chan struct{}, 8)
	)
	consumer, err := NewRedisStreamConsumer(&RedisStreamConfig{
		Addr:      mr.Addr(),
		StreamKey: streamKey,
	}, ConsumerOpts{
		GroupName:   group,
		ConsumerID:  "logstore-1",
		BatchSize:   10,
		FlushMs:     50,
		MinIdleTime: time.Hour, // healthy in-flight entries must not be reclaimed mid-test
		AckMode:     AckByHandler,
	}, func(_ context.Context, batch []Entry) error {
		mu.Lock()
		for _, e := range batch {
			ids = append(ids, e.StreamID)
		}
		mu.Unlock()
		got <- struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = consumer.Stop() })

	for i := 0; i < 2; i++ {
		if _, err := rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: streamKey,
			Values: map[string]any{
				"ts":      strconv.FormatInt(time.Now().UnixMilli(), 10),
				"level":   "info",
				"service": "api-server",
				"msg":     "buffered",
				"payload": `{}`,
				"host":    "h1",
			},
		}).Result(); err != nil {
			t.Fatalf("xadd: %v", err)
		}
	}

	// Wait until both entries reached the saver (one or two batches).
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(ids)
		mu.Unlock()
		if n == 2 {
			break
		}
		select {
		case <-got:
		case <-deadline:
			t.Fatalf("saver saw %d entries within 2s, want 2", n)
		}
	}

	pending := func() int64 {
		p, err := rdb.XPending(ctx, streamKey, group).Result()
		if err != nil {
			t.Fatalf("xpending: %v", err)
		}
		return p.Count
	}
	if n := pending(); n != 2 {
		t.Fatalf("saver success must NOT ack under AckByHandler: pending=%d, want 2", n)
	}

	// Ack the first only — the second stays pending (per-id granularity).
	mu.Lock()
	first, second := ids[0], ids[1]
	mu.Unlock()
	if err := consumer.Ack(ctx, first); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if n := pending(); n != 1 {
		t.Fatalf("after acking 1 of 2: pending=%d, want 1", n)
	}
	if err := consumer.Ack(ctx); err != nil {
		t.Fatalf("empty ack must be a no-op, got %v", err)
	}
	if err := consumer.Ack(ctx, second, first); err != nil { // re-acking an acked id is harmless
		t.Fatalf("ack: %v", err)
	}
	if n := pending(); n != 0 {
		t.Fatalf("after acking all: pending=%d, want 0", n)
	}
}

// TestRedisStreamConsumer_AckBySaverDefaultStillAcks pins the default: with
// AckMode unset, saver success acks the batch (existing callers unchanged).
func TestRedisStreamConsumer_AckBySaverDefaultStillAcks(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	const streamKey, group = "system_logs", "system-log-persister"
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	got := make(chan struct{}, 8)
	consumer, err := NewRedisStreamConsumer(&RedisStreamConfig{
		Addr:      mr.Addr(),
		StreamKey: streamKey,
	}, ConsumerOpts{
		GroupName:  group,
		ConsumerID: "apiserver-1",
		BatchSize:  10,
		FlushMs:    50,
	}, func(_ context.Context, batch []Entry) error {
		got <- struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = consumer.Stop() })

	if _, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{
			"ts":      strconv.FormatInt(time.Now().UnixMilli(), 10),
			"level":   "info",
			"msg":     "direct",
			"payload": `{}`,
		},
	}).Result(); err != nil {
		t.Fatalf("xadd: %v", err)
	}
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("saver did not receive entry within 2s")
	}
	// XACK happens right after the saver returns; poll briefly for it.
	deadline := time.After(2 * time.Second)
	for {
		p, err := rdb.XPending(ctx, streamKey, group).Result()
		if err != nil {
			t.Fatalf("xpending: %v", err)
		}
		if p.Count == 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("default mode must ack on saver success, pending=%d", p.Count)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestNewRedisStreamConsumer_RejectsUnknownAckMode(t *testing.T) {
	_, err := NewRedisStreamConsumer(&RedisStreamConfig{Addr: "localhost:6379", StreamKey: "s"}, ConsumerOpts{
		GroupName: "g", ConsumerID: "c", AckMode: AckMode(42),
	}, func(context.Context, []Entry) error { return nil })
	if err == nil {
		t.Fatal("unknown ack mode must be rejected at construction")
	}
}
