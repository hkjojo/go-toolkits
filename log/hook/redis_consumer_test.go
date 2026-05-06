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
