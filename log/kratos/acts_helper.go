package kratos

import (
	"context"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"go.opentelemetry.io/otel/trace"
)

// ActsHelper is a logger helper.
type ActsHelper struct {
	logger log.Logger
	kvs    []interface{}
}

// NewActsHelper new a logger helper.
func NewActsHelper(logger log.Logger) *ActsHelper {
	return &ActsHelper{
		logger: logger,
	}
}

func (h *ActsHelper) log(level log.Level, keyvals ...interface{}) {
	_ = h.logger.Log(level, keyvals...)
}

// Debugw logs a message at debug level.
func (h *ActsHelper) Debugw(logType int32, source, msg string, keyvals ...interface{}) {
	h.log(log.LevelDebug, append(keyvals, append(h.kvs, logType, source, msgkey(msg))...)...)
}

// Infow logs a message at info level.
func (h *ActsHelper) Infow(logType int32, source, msg string, keyvals ...interface{}) {
	h.log(log.LevelInfo, append(keyvals, append(h.kvs, logType, source, msgkey(msg))...)...)
}

// Warnw logs a message at warnf level.
func (h *ActsHelper) Warnw(logType int32, source, msg string, keyvals ...interface{}) {
	h.log(log.LevelWarn, append(keyvals, append(h.kvs, logType, source, msgkey(msg))...)...)
}

// Errorw logs a message at error level.
func (h *ActsHelper) Errorw(logType int32, source, msg string, keyvals ...interface{}) {
	h.log(log.LevelError, append(keyvals, append(h.kvs, logType, source, msgkey(msg))...)...)
}

// Fatalw logs a message at fatal level.
func (h *ActsHelper) Fatalw(logType int32, source, msg string, keyvals ...interface{}) {
	h.log(log.LevelFatal, append(keyvals, append(h.kvs, logType, source, msgkey(msg))...)...)
	os.Exit(1)
}

// WithDDOtelTrace convenient way to connect datadog logs and traces
// through otel standard
func (h *ActsHelper) WithDDOtelTrace(ctx context.Context) *ActsHelper {
	var (
		span    = trace.SpanFromContext(ctx)
		spanID  = span.SpanContext().SpanID().String()
		traceID = span.SpanContext().TraceID().String()
	)

	return &ActsHelper{
		logger: h.logger,
		kvs: append(h.kvs,
			"dd.span_id", convertTraceID(spanID),
			"dd.trace_id", convertTraceID(traceID),
		),
	}
}

// With ...
func (h *ActsHelper) With(args ...interface{}) *ActsHelper {
	return &ActsHelper{
		logger: h.logger,
		kvs:    append(h.kvs, args...),
	}
}
