package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// withStdoutFile points os.Stdout at a temp file for the duration of the test
// and returns a reader of its current content.
func withStdoutFile(t *testing.T) func() string {
	t.Helper()

	f, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	orig := os.Stdout
	os.Stdout = f
	t.Cleanup(func() {
		os.Stdout = orig
		_ = f.Close()
	})
	return func() string {
		b, err := os.ReadFile(f.Name())
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		return string(b)
	}
}

func TestStdoutBuffer_ErrorFlushesImmediatelyInfoWaits(t *testing.T) {
	read := withStdoutFile(t)
	l, err := New(&Config{Level: "info", Format: "json", StdoutBufferSize: 64 * 1024})
	if err != nil {
		t.Fatal(err)
	}

	l.Info("buffered info")
	if got := read(); got != "" {
		t.Fatalf("info written before flush: %q", got)
	}

	l.Error("urgent error")
	got := read()
	if !strings.Contains(got, "buffered info") || !strings.Contains(got, "urgent error") {
		t.Fatalf("error should flush the buffer in order, got %q", got)
	}
	if strings.Index(got, "buffered info") > strings.Index(got, "urgent error") {
		t.Fatalf("order changed: %q", got)
	}

	l.With(zap.String("k", "v")).Info("after error via With")
	if strings.Contains(read(), "after error") {
		t.Fatal("info after the error should be buffered again")
	}
	l.With(zap.String("k", "v")).Error("error via With")
	if !strings.Contains(read(), "error via With") {
		t.Fatal("With-derived logger must keep the flush-on-error behaviour")
	}

	l.Info("tail info")
	if err := FlushStdout(); err != nil {
		t.Fatalf("FlushStdout: %v", err)
	}
	if !strings.Contains(read(), "tail info") {
		t.Fatal("FlushStdout did not write the buffer out")
	}
}

func TestStdoutBuffer_DisabledWritesImmediately(t *testing.T) {
	read := withStdoutFile(t)
	l, err := New(&Config{Level: "info", Format: "json"})
	if err != nil {
		t.Fatal(err)
	}

	l.Info("direct info")
	if !strings.Contains(read(), "direct info") {
		t.Fatal("unbuffered info should be written immediately")
	}
}
