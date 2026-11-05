package log

import (
	"io"
	"os"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"
)

// stdoutFlushInterval bounds how long a buffered stdout line can wait.
const stdoutFlushInterval = time.Second

var stdoutBuffers struct {
	mu   sync.Mutex
	list []*zapcore.BufferedWriteSyncer
}

// newStdoutBuffer wraps os.Stdout in a BufferedWriteSyncer and registers it
// for FlushStdout.
func newStdoutBuffer(size int) *zapcore.BufferedWriteSyncer {
	ws := &zapcore.BufferedWriteSyncer{
		WS:            noFsync{os.Stdout},
		Size:          size,
		FlushInterval: stdoutFlushInterval,
	}
	stdoutBuffers.mu.Lock()
	stdoutBuffers.list = append(stdoutBuffers.list, ws)
	stdoutBuffers.mu.Unlock()
	return ws
}

// FlushStdout writes out every buffered stdout created by New. Call it before
// the process exits (including paths that call os.Exit). It works no matter
// how the logger has been wrapped since, and is a no-op when no buffer exists.
// The buffers keep working afterwards.
func FlushStdout() error {
	stdoutBuffers.mu.Lock()
	list := append([]*zapcore.BufferedWriteSyncer(nil), stdoutBuffers.list...)
	stdoutBuffers.mu.Unlock()

	var first error
	for _, ws := range list {
		if err := ws.Sync(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// noFsync makes BufferedWriteSyncer.Sync only write the buffer out. Without it
// every Sync (each error entry, see syncOnErrorCore) would also fsync stdout,
// which costs a disk flush per entry whenever stdout is redirected to a file.
type noFsync struct{ io.Writer }

func (noFsync) Sync() error { return nil }

// syncOnErrorCore flushes the buffered stdout right after an error-and-above
// entry, so errors are never held back by the buffer. zap's ioCore only syncs
// for levels above error.
//
// Check and With must be overridden: the embedded ioCore's Check would add the
// inner core to the CheckedEntry and bypass this Write.
type syncOnErrorCore struct{ zapcore.Core }

func (c syncOnErrorCore) With(fields []zapcore.Field) zapcore.Core {
	return syncOnErrorCore{Core: c.Core.With(fields)}
}

func (c syncOnErrorCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c syncOnErrorCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	if err := c.Core.Write(ent, fields); err != nil {
		return err
	}
	if ent.Level >= zapcore.ErrorLevel {
		return c.Core.Sync()
	}
	return nil
}
