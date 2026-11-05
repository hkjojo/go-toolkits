package hook

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
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
	//
	// Mode selects the server topology: "" or "single" (default, unchanged behavior) or
	// "cluster". In cluster mode Addr may be a single seed (e.g. an ElastiCache
	// configuration endpoint) or a comma-separated seed list, and DB must be 0 — Redis
	// Cluster has no logical databases. A single-node client pointed at a cluster gets
	// MOVED for every key that lives on another node, so the mode has to be explicit
	// rather than guessed from the address.
	Mode     string `json:"mode"`
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	UseTLS   bool   `json:"use_tls"`

	// Stream behavior.
	StreamKey       string `json:"stream_key"`        // default "system_logs"
	MaxLen          int64  `json:"max_len"`           // default 200_000 (approximate)
	BatchSize       int    `json:"batch_size"`        // reserved for future batching; current impl is single-write
	FlushMs         int    `json:"flush_ms"`          // reserved for future batching
	QPS             int    `json:"qps"`               // info/debug budget per second; 0 = default 50, < 0 = unlimited
	WarnQPS         int    `json:"warn_qps"`          // warn budget per second; 0 = default 50, < 0 = unlimited
	ErrorQPS        int    `json:"error_qps"`         // error-and-above budget per second; 0 = default 50, < 0 = unlimited
	MaxPayloadBytes int    `json:"max_payload_bytes"` // default 4096; oversized payload replaced with {"_truncated":true}
}

// Redis topologies accepted by RedisStreamConfig.Mode.
const (
	RedisModeSingle  = "single"
	RedisModeCluster = "cluster"
)

// redisStreamClientOptions builds the client options shared by the producer core and
// the consumer, so both sides always connect to the same topology.
//
// The stream is a single key, so every command the hook and consumer issue (XADD,
// XREADGROUP, XACK, XAUTOCLAIM, XINFO, XGROUP) is single-key and cluster-safe; in
// cluster mode the whole stream lives on the shard that owns its slot.
func redisStreamClientOptions(cfg *RedisStreamConfig) (*redis.UniversalOptions, error) {
	opts := &redis.UniversalOptions{
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case "", RedisModeSingle:
		if strings.Contains(cfg.Addr, ",") {
			return nil, fmt.Errorf("redis stream: addr %q lists several nodes; set mode: cluster for a cluster", cfg.Addr)
		}
		opts.Addrs = []string{strings.TrimSpace(cfg.Addr)}
	case RedisModeCluster:
		if cfg.DB != 0 {
			return nil, fmt.Errorf("redis stream: db must be 0 in cluster mode, got %d", cfg.DB)
		}
		for _, a := range strings.Split(cfg.Addr, ",") {
			if a = strings.TrimSpace(a); a != "" {
				opts.Addrs = append(opts.Addrs, a)
			}
		}
		if len(opts.Addrs) == 0 {
			return nil, fmt.Errorf("redis stream: addr is required")
		}
		// Without this, go-redis builds a single-node client when only one seed is given.
		opts.IsClusterMode = true
	default:
		return nil, fmt.Errorf("redis stream: unknown mode %q (want single or cluster)", cfg.Mode)
	}
	if cfg.UseTLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return opts, nil
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
//   - writeData: XADD failure is counted and the entry is dropped; stderr
//     reports the first failure and then every stderrEvery-th.
//     stdout/file paths are unaffected (zap tee runs cores independently).
//   - Producer storm: one token bucket per level class (QPS / WarnQPS /
//     ErrorQPS), checked on the caller goroutine before any field work, so
//     info volume can never use up the warn or error budget.
//   - Queue full: only the incoming entry is dropped.
//
// Every drop is counted by reason and level — see DropStats / DroppedTotal.
type RedisStreamCore struct {
	*BaseCore

	cfg    *RedisStreamConfig
	client redis.UniversalClient
	host   string

	buckets [tierCount]*tokenBucket
	drops   dropCounters
}

var xaddErrReport everyN

// NewRedisStreamCore constructs a RedisStreamCore. The returned core is ready
// to be tee'd onto a zap.Logger via zapcore.NewTee, or appended to the cores
// list inside tklog.New (which RedisStreamConfig is wired into via the main
// Config.Redis field).
//
// Defaults applied for unset fields:
//   - StreamKey:       "system_logs"
//   - MaxLen:          200_000
//   - QPS / WarnQPS / ErrorQPS: 50 each (negative disables that throttle)
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
	opts, err := redisStreamClientOptions(cfg)
	if err != nil {
		return nil, fmt.Errorf("redis stream core: %w", err)
	}

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
		cfg:  cfg,
		host: host,
		buckets: [tierCount]*tokenBucket{
			tierInfo:  newTokenBucket(cfg.QPS),
			tierWarn:  newTokenBucket(cfg.WarnQPS),
			tierError: newTokenBucket(cfg.ErrorQPS),
		},
	}
	core.BaseCore.core = core

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

// DroppedTotal returns the cumulative count of entries this core dropped
// (over budget, queue full or XADD error) since construction. DropStats gives
// the process-wide breakdown by reason and level.
func (c *RedisStreamCore) DroppedTotal() uint64 {
	return c.drops.snapshot().Total()
}

// DropStats returns this core's drop counts by reason and level.
func (c *RedisStreamCore) DropStats() DropCounts {
	return c.drops.snapshot()
}

func (c *RedisStreamCore) recordDrop(r dropReason, lvl zapcore.Level) {
	c.drops.add(r, lvl)
	processDrops.add(r, lvl)
}

// admit runs on the caller goroutine before BaseCore does any field work, so
// an over-budget entry costs one atomic check.
func (c *RedisStreamCore) admit(lvl zapcore.Level) bool {
	if c.buckets[tierOf(lvl)].allow() {
		return true
	}
	c.recordDrop(dropRate, lvl)
	return false
}

func (c *RedisStreamCore) recordQueueDrop(lvl zapcore.Level) {
	c.recordDrop(dropQueue, lvl)
}

// writeData runs in the BaseCore goroutine (single consumer of c.queue).
// It serializes the entry per the ADR-0042 contract and XADDs to the stream.
// Budget checks already happened in admit.
func (c *RedisStreamCore) writeData(data *CoreData) {

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
		payloadMap[f.Key] = c.getPayloadField(f)
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
		c.recordDrop(dropXAddError, data.entry.Level)
		if n, ok := xaddErrReport.hit(); ok {
			fmt.Fprintf(os.Stderr,
				"[log] redis stream xadd err (failures_total=%d): %v\n", n, err)
		}
	}
}

func (c *RedisStreamCore) getPayloadField(field zapcore.Field) any {
	value := c.getField(field)
	switch field.Type {
	case zapcore.ErrorType:
		if err, ok := value.(error); ok {
			if err == nil {
				return nil
			}
			return err.Error()
		}
	case zapcore.StringerType:
		if stringer, ok := value.(fmt.Stringer); ok {
			return safeString(stringer)
		}
	}
	return safeJSONValue(value)
}

func safeJSONValue(value any) any {
	if value == nil {
		return nil
	}
	if err, ok := value.(error); ok {
		return err.Error()
	}
	if _, err := json.Marshal(value); err == nil {
		return value
	}
	return sanitizeReflectValue(reflect.ValueOf(value), make(map[uintptr]struct{}), 0)
}

func sanitizeReflectValue(value reflect.Value, seen map[uintptr]struct{}, depth int) any {
	if !value.IsValid() {
		return nil
	}
	if depth > 8 {
		if value.CanInterface() {
			return fmt.Sprint(value.Interface())
		}
		return fmt.Sprint(value)
	}
	if value.CanInterface() {
		if err, ok := value.Interface().(error); ok {
			return err.Error()
		}
	}

	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		return sanitizeReflectValue(value.Elem(), seen, depth+1)
	case reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		ptr := value.Pointer()
		if _, ok := seen[ptr]; ok {
			return "<cycle>"
		}
		seen[ptr] = struct{}{}
		return sanitizeReflectValue(value.Elem(), seen, depth+1)
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		out := make(map[string]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			out[fmt.Sprint(iter.Key().Interface())] = sanitizeReflectValue(iter.Value(), seen, depth+1)
		}
		return out
	case reflect.Slice, reflect.Array:
		out := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			out[i] = sanitizeReflectValue(value.Index(i), seen, depth+1)
		}
		return out
	case reflect.Struct:
		t := value.Type()
		out := make(map[string]any, value.NumField())
		for i := 0; i < value.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" {
				continue
			}
			out[field.Name] = sanitizeReflectValue(value.Field(i), seen, depth+1)
		}
		return out
	case reflect.Func:
		if value.IsNil() {
			return nil
		}
		return "<func>"
	case reflect.Chan:
		if value.IsNil() {
			return nil
		}
		return "<chan>"
	case reflect.Complex64, reflect.Complex128, reflect.UnsafePointer:
		return fmt.Sprint(value.Interface())
	default:
		if value.CanInterface() {
			return value.Interface()
		}
		return fmt.Sprint(value)
	}
}

func safeString(stringer fmt.Stringer) (out string) {
	defer func() {
		if rec := recover(); rec != nil {
			out = fmt.Sprint(rec)
		}
	}()
	if stringer == nil {
		return ""
	}
	return stringer.String()
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
	if cfg.WarnQPS == 0 {
		cfg.WarnQPS = 50
	}
	if cfg.ErrorQPS == 0 {
		cfg.ErrorQPS = 50
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
