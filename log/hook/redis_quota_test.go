package hook

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func countStreamLevel(t *testing.T, addr, streamKey, level string) int {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer func() { _ = rdb.Close() }()
	res, err := rdb.XRange(context.Background(), streamKey, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	n := 0
	for _, msg := range res {
		if toString(msg.Values["level"]) == level {
			n++
		}
	}
	return n
}

// TestRedisStreamCore_LevelBudgetsIndependent: info and warn floods cannot use
// up the error budget.
func TestRedisStreamCore_LevelBudgetsIndependent(t *testing.T) {
	core, mr := newTestRedisStreamCore(t, &RedisStreamConfig{
		StreamKey: "system_logs",
		QPS:       2,
		WarnQPS:   2,
		ErrorQPS:  5,
	}, nil)
	before := DropStats()

	logger := zap.New(core)
	for i := 0; i < 20; i++ {
		logger.Info("info flood")
		logger.Warn("warn flood")
	}
	for i := 0; i < 5; i++ {
		logger.Error("must arrive")
	}

	deadline := time.Now().Add(2 * time.Second)
	for countStreamLevel(t, mr.Addr(), "system_logs", "error") < 5 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := countStreamLevel(t, mr.Addr(), "system_logs", "error"); got != 5 {
		t.Fatalf("error entries in stream = %d, want 5", got)
	}

	got := core.DropStats()
	if got.Rate.Error != 0 {
		t.Errorf("Rate.Error = %d, want 0", got.Rate.Error)
	}
	if got.Rate.Info == 0 || got.Rate.Warn == 0 {
		t.Errorf("expected info and warn rate drops, got %+v", got.Rate)
	}

	after := DropStats()
	if after.Rate.Info-before.Rate.Info != got.Rate.Info {
		t.Errorf("process-wide Rate.Info delta = %d, want %d", after.Rate.Info-before.Rate.Info, got.Rate.Info)
	}
}

// TestRedisStreamCore_OverBudgetSkipsFieldWork: a rejected entry never reaches
// the queue.
func TestRedisStreamCore_OverBudgetSkipsFieldWork(t *testing.T) {
	core, _ := newTestRedisStreamCore(t, &RedisStreamConfig{StreamKey: "system_logs", QPS: 1}, nil)

	if !core.admit(zapcore.InfoLevel) {
		t.Fatal("first info entry should be admitted")
	}
	if core.admit(zapcore.InfoLevel) {
		t.Fatal("second info entry should exceed qps=1")
	}
	if got := core.DropStats().Rate.Info; got != 1 {
		t.Fatalf("Rate.Info = %d, want 1", got)
	}
}

type recordingCore struct {
	mu      sync.Mutex
	dropped []zapcore.Level
}

func (r *recordingCore) writeData(*CoreData) {}

func (r *recordingCore) recordQueueDrop(lvl zapcore.Level) {
	r.mu.Lock()
	r.dropped = append(r.dropped, lvl)
	r.mu.Unlock()
}

// TestBaseCore_QueueFullDropsOnlyIncoming: a full queue keeps what is already
// queued (the old behaviour drained the whole queue, errors included).
func TestBaseCore_QueueFullDropsOnlyIncoming(t *testing.T) {
	rec := &recordingCore{}
	base := &BaseCore{
		LevelEnabler: zap.NewAtomicLevelAt(zapcore.DebugLevel),
		queue:        make(chan *CoreData, 2), // no consumer started
		core:         rec,
		enc:          zapcore.NewJSONEncoder(zapcore.EncoderConfig{}),
		out:          zapcore.AddSync(io.Discard),
	}

	_ = base.Write(zapcore.Entry{Level: zapcore.ErrorLevel, Message: "queued error"}, nil)
	_ = base.Write(zapcore.Entry{Level: zapcore.InfoLevel, Message: "queued info"}, nil)
	_ = base.Write(zapcore.Entry{Level: zapcore.InfoLevel, Message: "overflow"}, nil)

	if got := len(base.queue); got != 2 {
		t.Fatalf("queue len = %d, want 2 (queued entries kept)", got)
	}
	if first := <-base.queue; first.entry.Message != "queued error" {
		t.Fatalf("head of queue = %q, want queued error", first.entry.Message)
	}
	if len(rec.dropped) != 1 || rec.dropped[0] != zapcore.InfoLevel {
		t.Fatalf("recorded drops = %v, want [info]", rec.dropped)
	}
}

// TestRedisStreamCore_XAddErrorCounted: XADD failures are counted per level
// instead of only printing to stderr.
func TestRedisStreamCore_XAddErrorCounted(t *testing.T) {
	core, mr := newTestRedisStreamCore(t, &RedisStreamConfig{StreamKey: "system_logs"}, nil)
	mr.SetError("NOAUTH Authentication required.")

	zap.New(core).Error("lost")

	deadline := time.Now().Add(2 * time.Second)
	for core.DropStats().XAddError.Error == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := core.DropStats().XAddError.Error; got != 1 {
		t.Fatalf("XAddError.Error = %d, want 1", got)
	}
}

func TestTierOf(t *testing.T) {
	cases := map[zapcore.Level]levelTier{
		zapcore.DebugLevel:  tierInfo,
		zapcore.InfoLevel:   tierInfo,
		zapcore.WarnLevel:   tierWarn,
		zapcore.ErrorLevel:  tierError,
		zapcore.DPanicLevel: tierError,
		zapcore.FatalLevel:  tierError,
	}
	for lvl, want := range cases {
		if got := tierOf(lvl); got != want {
			t.Errorf("tierOf(%s) = %d, want %d", lvl, got, want)
		}
	}
}

func TestApplyRedisStreamDefaults_LevelBudgets(t *testing.T) {
	cfg := &RedisStreamConfig{}
	applyRedisStreamDefaults(cfg)
	if cfg.QPS != 50 || cfg.WarnQPS != 50 || cfg.ErrorQPS != 50 {
		t.Fatalf("defaults qps/warn/error = %d/%d/%d, want 50/50/50", cfg.QPS, cfg.WarnQPS, cfg.ErrorQPS)
	}

	cfg = &RedisStreamConfig{ErrorQPS: -1}
	applyRedisStreamDefaults(cfg)
	if b := newTokenBucket(cfg.ErrorQPS); !b.allow() || !b.allow() {
		t.Fatal("negative error_qps should be unlimited")
	}
}
