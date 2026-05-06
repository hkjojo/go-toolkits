package kratos

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	klog "github.com/go-kratos/kratos/v2/log"

	tklog "github.com/hkjojo/go-toolkits/log/v2"
)

// TestLog_KratosFrameworkEvenKvs_NoDoubleMsg guards against the cosmetic bug
// where even-length kvs starting with ("msg", X) emitted a double "msg" key
// in JSON output. Before the H1 fix, framework log of the form
//
//	logger.Log(InfoLevel, "msg", "[HTTP] server listening", "k", "v")
//
// produced {"msg":"","msg":"[HTTP] server listening","k":"v"}.
func TestLog_KratosFrameworkEvenKvs_NoDoubleMsg(t *testing.T) {
	line := captureSingleLogLine(t, func(logger klog.Logger) {
		_ = logger.Log(klog.LevelInfo,
			"msg", "[HTTP] server listening on: [::]:8080",
			"k", "v",
		)
	})

	if got := strings.Count(line, `"msg"`); got != 1 {
		t.Fatalf("expected single msg key, got %d in line: %s", got, line)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["msg"] != "[HTTP] server listening on: [::]:8080" {
		t.Errorf("msg=%q, want HTTP listening message", m["msg"])
	}
	if m["k"] != "v" {
		t.Errorf("k=%v, want v", m["k"])
	}
}

// TestLog_HkjojoHelperOddKvs_TailMsgkey covers the original odd-length kvs
// path (Helper.Infow appends a msgkey at the tail) — must continue to work
// after the H1 even-kvs branch was added.
func TestLog_HkjojoHelperOddKvs_TailMsgkey(t *testing.T) {
	line := captureSingleLogLine(t, func(logger klog.Logger) {
		_ = logger.Log(klog.LevelInfo,
			"k", "v",
			msgkey("api-server started"),
		)
	})

	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("unmarshal: %v\nline=%s", err, line)
	}
	if m["msg"] != "api-server started" {
		t.Errorf("msg=%q, want api-server started", m["msg"])
	}
	if m["k"] != "v" {
		t.Errorf("k=%v, want v", m["k"])
	}
}

// TestSync_NoRecursion guards against the Sync infinite-recursion bug. Before
// H1 fix, `func (l *logger) Sync() error { return l.Sync() }` would recurse
// into itself forever; any caller of Sync hit a stack-overflow panic.
func TestSync_NoRecursion(t *testing.T) {
	logger, err := NewZapLog(&tklog.Config{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	syncer, ok := logger.(klog.Logger)
	if !ok {
		t.Fatalf("logger does not implement klog.Logger")
	}
	type syncable interface {
		Sync() error
	}
	if s, ok := syncer.(syncable); ok {
		// If the recursion is back, this call stack-overflows; the panic is
		// caught by the test framework. We don't actually care about the
		// return value (zap stdout sync sometimes errors with EINVAL),
		// only that the call returns within reasonable time/depth.
		_ = s.Sync()
	}
}

// captureSingleLogLine builds a logger that writes to an in-memory buffer,
// invokes the provided func, and returns the (single) JSON log line emitted.
func captureSingleLogLine(t *testing.T, emit func(logger klog.Logger)) string {
	t.Helper()

	// Redirect stdout to a pipe; tklog Config.DisableStdout=false is the
	// default path, so this works without modifying tklog internals.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	logger, err := NewZapLog(&tklog.Config{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	emit(logger)
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read: %v", err)
	}

	line := strings.TrimSpace(buf.String())
	// In case of multi-line output, take the first JSON-shaped line.
	if idx := strings.Index(line, "\n"); idx >= 0 {
		line = line[:idx]
	}
	return line
}
