package hook

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newTestRedisStreamCore(t *testing.T, cfg *RedisStreamConfig, fields map[string]string) (*RedisStreamCore, *miniredis.Miniredis) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	if cfg == nil {
		cfg = &RedisStreamConfig{}
	}
	cfg.Addr = mr.Addr()

	encConfig := zapcore.EncoderConfig{
		MessageKey:   "msg",
		LevelKey:     "level",
		TimeKey:      "time",
		EncodeLevel:  zapcore.LowercaseLevelEncoder,
		EncodeTime:   zapcore.RFC3339NanoTimeEncoder,
		LineEnding:   zapcore.DefaultLineEnding,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}
	core, err := NewRedisStreamCore(cfg, "", fields, encConfig)
	if err != nil {
		t.Fatalf("new core: %v", err)
	}
	t.Cleanup(func() { _ = core.Close() })

	return core, mr
}

func readStreamEntries(t *testing.T, mr *miniredis.Miniredis, streamKey string) []map[string]string {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	res, err := rdb.XRange(context.Background(), streamKey, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	out := make([]map[string]string, 0, len(res))
	for _, msg := range res {
		m := make(map[string]string, len(msg.Values))
		for k, v := range msg.Values {
			m[k] = toString(v)
		}
		out = append(out, m)
	}
	return out
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if s, ok := v.([]byte); ok {
		return string(s)
	}
	return ""
}

func waitForStreamLen(t *testing.T, mr *miniredis.Miniredis, streamKey string, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		got, err := rdb.XLen(context.Background(), streamKey).Result()
		_ = rdb.Close()
		if err == nil && int(got) >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("stream %s did not reach len=%d in time", streamKey, want)
}

// TestRedisStreamCore_XAddShape validates the producer side wire format
// (per ADR-0042). Consumer-side parseEntry depends on these exact field
// names — drift breaks the system_log pipeline.
func TestRedisStreamCore_XAddShape(t *testing.T) {
	core, mr := newTestRedisStreamCore(t, &RedisStreamConfig{
		StreamKey: "system_logs",
		QPS:       0, // disable throttle for shape test
	}, map[string]string{
		"service": "maker-adapter",
	})

	entry := zapcore.Entry{
		Time:    time.Date(2026, 5, 6, 12, 34, 56, 0, time.UTC),
		Level:   zapcore.InfoLevel,
		Message: "connection_restored",
	}
	core.writeData(&CoreData{
		entry: entry,
		fields: []zapcore.Field{
			zap.String("source", "lp-gateway"),
			zap.String("lp", "prime"),
			zap.Int("downtime_sec", 192),
		},
	})

	waitForStreamLen(t, mr, "system_logs", 1)
	entries := readStreamEntries(t, mr, "system_logs")
	if len(entries) != 1 {
		t.Fatalf("entries=%d, want 1", len(entries))
	}

	got := entries[0]
	if got["service"] != "maker-adapter" {
		t.Errorf("service=%q, want maker-adapter", got["service"])
	}
	if got["source"] != "lp-gateway" {
		t.Errorf("source=%q, want lp-gateway", got["source"])
	}
	if got["level"] != "info" {
		t.Errorf("level=%q, want info (zap raw)", got["level"])
	}
	if got["msg"] != "connection_restored" {
		t.Errorf("msg=%q, want connection_restored", got["msg"])
	}
	if got["host"] == "" {
		t.Errorf("host should be auto-injected from os.Hostname()")
	}
	wantTs := strconv.FormatInt(entry.Time.UnixMilli(), 10)
	if got["ts"] != wantTs {
		t.Errorf("ts=%q, want %q", got["ts"], wantTs)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(got["payload"]), &payload); err != nil {
		t.Fatalf("payload unmarshal: %v\npayload=%s", err, got["payload"])
	}
	if payload["lp"] != "prime" {
		t.Errorf("payload.lp=%v, want prime", payload["lp"])
	}
	// JSON numbers decode to float64 by default; assert equivalent value.
	if got, want := payload["downtime_sec"].(float64), float64(192); got != want {
		t.Errorf("payload.downtime_sec=%v, want %v", got, want)
	}
	// source must NOT appear in payload — it's hoisted to top-level.
	if _, ok := payload["source"]; ok {
		t.Errorf("source should not be in payload; got %v", payload)
	}
}

// TestRedisStreamCore_PayloadTruncation guards the MaxPayloadBytes hard cap.
// Oversized payload is REPLACED (not partially truncated) to keep JSON valid.
func TestRedisStreamCore_PayloadTruncation(t *testing.T) {
	core, mr := newTestRedisStreamCore(t, &RedisStreamConfig{
		StreamKey:       "system_logs",
		MaxPayloadBytes: 100, // tight bound to force truncation
		QPS:             0,
	}, nil)

	core.writeData(&CoreData{
		entry: zapcore.Entry{
			Time:    time.Now(),
			Level:   zapcore.WarnLevel,
			Message: "big_payload",
		},
		fields: []zapcore.Field{
			zap.String("dump", strings.Repeat("x", 500)),
		},
	})

	waitForStreamLen(t, mr, "system_logs", 1)
	entries := readStreamEntries(t, mr, "system_logs")
	if entries[0]["payload"] != `{"_truncated":true}` {
		t.Errorf("payload=%q, want {\"_truncated\":true}", entries[0]["payload"])
	}
}

type jsonBadError struct {
	Fn  func()
	msg string
}

func (e jsonBadError) Error() string {
	if e.msg == "" {
		return "json bad error"
	}
	return e.msg
}

var _ error = jsonBadError{}

func TestRedisStreamCore_SanitizesUnmarshalablePayloadFields(t *testing.T) {
	core, mr := newTestRedisStreamCore(t, &RedisStreamConfig{
		StreamKey: "system_logs",
		QPS:       0,
	}, nil)

	core.writeData(&CoreData{
		entry: zapcore.Entry{
			Time:    time.Now(),
			Level:   zapcore.ErrorLevel,
			Message: "sweeper_failed",
		},
		fields: []zapcore.Field{
			zap.Any("err", jsonBadError{Fn: func() {}, msg: "db exploded"}),
			zap.String("runner", "market_override"),
		},
	})

	waitForStreamLen(t, mr, "system_logs", 1)
	entries := readStreamEntries(t, mr, "system_logs")

	var payload map[string]any
	if err := json.Unmarshal([]byte(entries[0]["payload"]), &payload); err != nil {
		t.Fatalf("payload unmarshal: %v\npayload=%s", err, entries[0]["payload"])
	}
	if _, ok := payload["_encode_error"]; ok {
		t.Fatalf("payload should not be replaced by _encode_error: %v", payload)
	}
	if payload["err"] != "db exploded" {
		t.Fatalf("payload.err=%v, want db exploded", payload["err"])
	}
	if payload["runner"] != "market_override" {
		t.Fatalf("payload.runner=%v, want market_override", payload["runner"])
	}
}

func TestRedisStreamCore_SanitizesUnsupportedPayloadValuesIndividually(t *testing.T) {
	core, mr := newTestRedisStreamCore(t, &RedisStreamConfig{
		StreamKey: "system_logs",
		QPS:       0,
	}, nil)

	core.writeData(&CoreData{
		entry: zapcore.Entry{
			Time:    time.Now(),
			Level:   zapcore.ErrorLevel,
			Message: "panic_recovered",
		},
		fields: []zapcore.Field{
			zap.Any("panic", func() {}),
			zap.String("runner", "activation"),
		},
	})

	waitForStreamLen(t, mr, "system_logs", 1)
	entries := readStreamEntries(t, mr, "system_logs")

	var payload map[string]any
	if err := json.Unmarshal([]byte(entries[0]["payload"]), &payload); err != nil {
		t.Fatalf("payload unmarshal: %v\npayload=%s", err, entries[0]["payload"])
	}
	if _, ok := payload["_encode_error"]; ok {
		t.Fatalf("payload should not be replaced by _encode_error: %v", payload)
	}
	if payload["panic"] != "<func>" {
		t.Fatalf("payload.panic=%v, want <func>", payload["panic"])
	}
	if payload["runner"] != "activation" {
		t.Fatalf("payload.runner=%v, want activation", payload["runner"])
	}
}

func TestRedisStreamCore_WriteHoistsMetadataAndStringifiesError(t *testing.T) {
	core, mr := newTestRedisStreamCore(t, &RedisStreamConfig{
		StreamKey: "system_logs",
		QPS:       0,
	}, map[string]string{
		"service": "api-server",
	})

	if err := core.Write(zapcore.Entry{
		Time:    time.Now(),
		Level:   zapcore.WarnLevel,
		Message: "outbox poller XADD failed",
	}, []zapcore.Field{
		zap.String("source", "background"),
		zap.Int64("event_id", 9951),
		zap.String("event_type", "config.updated"),
		zap.String("entity_type", "client"),
		zap.Any("err", errors.New("redis_stream: xadd config_changes: WRONGTYPE wrong kind")),
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	waitForStreamLen(t, mr, "system_logs", 1)
	entries := readStreamEntries(t, mr, "system_logs")
	got := entries[0]
	if got["service"] != "api-server" {
		t.Fatalf("service=%q, want api-server", got["service"])
	}
	if got["source"] != "background" {
		t.Fatalf("source=%q, want background", got["source"])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(got["payload"]), &payload); err != nil {
		t.Fatalf("payload unmarshal: %v\npayload=%s", err, got["payload"])
	}
	if _, ok := payload["service"]; ok {
		t.Fatalf("service should be hoisted out of payload: %v", payload)
	}
	if _, ok := payload["source"]; ok {
		t.Fatalf("source should be hoisted out of payload: %v", payload)
	}
	if payload["err"] != "redis_stream: xadd config_changes: WRONGTYPE wrong kind" {
		t.Fatalf("payload.err=%v", payload["err"])
	}
}

// TestRedisStreamCore_FilterDropsKeys guards the CoreConfig.Filter blacklist —
// listed field keys are silently dropped before serialization.
func TestRedisStreamCore_FilterDropsKeys(t *testing.T) {
	core, mr := newTestRedisStreamCore(t, &RedisStreamConfig{
		StreamKey: "system_logs",
		CoreConfig: CoreConfig{
			Filter: []string{"password", "api_key"},
		},
		QPS: 0,
	}, nil)

	// BaseCore.filterFields is invoked from BaseCore.Write before queueing.
	// We simulate the same path by calling Write directly.
	if err := core.Write(zapcore.Entry{
		Time:    time.Now(),
		Level:   zapcore.InfoLevel,
		Message: "login",
	}, []zapcore.Field{
		zap.String("user", "alice"),
		zap.String("password", "supersecret"),
		zap.String("api_key", "AKIA..."),
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	waitForStreamLen(t, mr, "system_logs", 1)
	entries := readStreamEntries(t, mr, "system_logs")

	var payload map[string]any
	_ = json.Unmarshal([]byte(entries[0]["payload"]), &payload)
	if _, ok := payload["password"]; ok {
		t.Errorf("password should be filtered, got %v", payload)
	}
	if _, ok := payload["api_key"]; ok {
		t.Errorf("api_key should be filtered, got %v", payload)
	}
	if payload["user"] != "alice" {
		t.Errorf("user=%v, want alice", payload["user"])
	}
}

// TestRedisStreamCore_ThrottleDrops guards the QPS token-bucket — over-budget
// entries are dropped silently (with droppedTotal incremented).
func TestRedisStreamCore_ThrottleDrops(t *testing.T) {
	core, mr := newTestRedisStreamCore(t, &RedisStreamConfig{
		StreamKey: "system_logs",
		QPS:       2, // very tight budget
	}, nil)

	for i := 0; i < 10; i++ {
		core.writeData(&CoreData{
			entry: zapcore.Entry{
				Time:    time.Now(),
				Level:   zapcore.InfoLevel,
				Message: "burst",
			},
		})
	}

	// Wait for whatever entries pass the bucket to land in the stream.
	time.Sleep(150 * time.Millisecond)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	got, err := rdb.XLen(context.Background(), "system_logs").Result()
	if err != nil {
		t.Fatalf("xlen: %v", err)
	}
	if got > 3 {
		t.Errorf("expected <=3 entries through bucket cap=2, got %d", got)
	}
	if core.DroppedTotal() == 0 {
		t.Errorf("expected DroppedTotal > 0, got 0")
	}
}
