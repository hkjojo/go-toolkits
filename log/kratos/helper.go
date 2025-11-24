package kratos

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"
	"go.opentelemetry.io/otel/trace"
)

type msgkey string

// Helper is a logger helper.
type Helper struct {
	logger log.Logger
	kvs    []interface{}
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
	h.log(log.LevelDebug, append(keyvals, append(h.kvs, msgkey(msg))...)...)
}

// Infow logs a message at info level.
func (h *Helper) Infow(msg string, keyvals ...interface{}) {
	h.log(log.LevelInfo, append(keyvals, append(h.kvs, msgkey(msg))...)...)
}

// Warnw logs a message at warnf level.
func (h *Helper) Warnw(msg string, keyvals ...interface{}) {
	h.log(log.LevelWarn, append(keyvals, append(h.kvs, msgkey(msg))...)...)
}

// Errorw logs a message at error level.
func (h *Helper) Errorw(msg string, keyvals ...interface{}) {
	h.log(log.LevelError, append(keyvals, append(h.kvs, msgkey(msg))...)...)
}

// Fatalw logs a message at fatal level.
func (h *Helper) Fatalw(msg string, keyvals ...interface{}) {
	h.log(log.LevelFatal, append(keyvals, append(h.kvs, msgkey(msg))...)...)
	os.Exit(1)
}

// Debugf logs a message at debug level.
func (h *Helper) Debugf(format string, arg ...interface{}) {
	if len(arg) > 0 {
		format = fmt.Sprintf(format, arg...)
	}

	h.log(log.LevelDebug, append(h.kvs, msgkey(format))...)
}

// Infof logs a message at info level.
func (h *Helper) Infof(format string, arg ...interface{}) {
	if len(arg) > 0 {
		format = fmt.Sprintf(format, arg...)
	}

	h.log(log.LevelInfo, append(h.kvs, msgkey(format))...)
}

// Warnf logs a message at warnf level.
func (h *Helper) Warnf(format string, arg ...interface{}) {
	if len(arg) > 0 {
		format = fmt.Sprintf(format, arg...)
	}

	h.log(log.LevelWarn, append(h.kvs, msgkey(format))...)
}

// Errorf logs a message at error level.
func (h *Helper) Errorf(format string, arg ...interface{}) {
	if len(arg) > 0 {
		format = fmt.Sprintf(format, arg...)
	}

	h.log(log.LevelError, append(h.kvs, msgkey(format))...)
}

// Fatalf logs a message at fatal level.
func (h *Helper) Fatalf(format string, arg ...interface{}) {
	if len(arg) > 0 {
		format = fmt.Sprintf(format, arg...)
	}

	h.log(log.LevelFatal, append(h.kvs, msgkey(format))...)
	os.Exit(1)
}

// WithDDOtelTrace convenient way to connect datadog logs and traces
// through otel standard
func (h *Helper) WithDDOtelTrace(ctx context.Context) *Helper {
	var (
		span    = trace.SpanFromContext(ctx)
		spanID  = span.SpanContext().SpanID().String()
		traceID = span.SpanContext().TraceID().String()
	)

	return &Helper{
		logger: h.logger,
		kvs: append(h.kvs,
			"dd.span_id", convertTraceID(spanID),
			"dd.trace_id", convertTraceID(traceID),
		),
	}
}

// With ...
func (h *Helper) With(args ...interface{}) *Helper {
	return &Helper{
		logger: h.logger,
		kvs:    append(h.kvs, args...),
	}
}

func convertTraceID(id string) string {
	if len(id) < 16 {
		return ""
	}
	if len(id) > 16 {
		id = id[16:]
	}
	intValue, err := strconv.ParseUint(id, 16, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(intValue, 10)
}
