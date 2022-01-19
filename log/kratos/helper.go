package kratos

import (
	"os"

	"github.com/go-kratos/kratos/v2/log"
)

type msgkey string

// Helper is a logger helper.
type Helper struct {
	logger log.Logger
}

// NewHelper new a logger helper.
func NewHelper(logger log.Logger) *Helper {
	return &Helper{
		logger: logger,
	}
}

func (h *Helper) log(level log.Level, keyvals ...interface{}) {
	_ = h.logger.Log(level, keyvals...)
}

// Debugw logs a message at debug level.
func (h *Helper) Debugw(msg string, keyvals ...interface{}) {
	h.log(log.LevelDebug, append(keyvals, msgkey(msg))...)
}

// Infow logs a message at info level.
func (h *Helper) Infow(msg string, keyvals ...interface{}) {
	h.log(log.LevelInfo, append(keyvals, msgkey(msg))...)
}

// Warnw logs a message at warnf level.
func (h *Helper) Warnw(msg string, keyvals ...interface{}) {
	h.log(log.LevelWarn, append(keyvals, msgkey(msg))...)
}

// Errorw logs a message at error level.
func (h *Helper) Errorw(msg string, keyvals ...interface{}) {
	h.log(log.LevelError, append(keyvals, msgkey(msg))...)
}

// Fatalw logs a message at fatal level.
func (h *Helper) Fatalw(msg string, keyvals ...interface{}) {
	h.log(log.LevelFatal, append(keyvals, msgkey(msg))...)
	os.Exit(1)
}
