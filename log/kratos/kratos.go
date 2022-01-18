package kratos

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	tklog "github.com/hkjojo/go-toolkits/log"
	"go.uber.org/zap"
)

type logger struct {
	*tklog.Logger

	hasValuer bool
	ctx       context.Context
	suffix    []interface{}
}

func NewZapLog(cfg *tklog.Config) (log.Logger, error) {
	tklogger, err := tklog.New(cfg)
	if err != nil {
		return nil, err
	}

	return &logger{Logger: tklogger}, nil
}

var _ log.Logger = (*logger)(nil)

func (l *logger) Log(level log.Level, keyvals ...interface{}) error {
	kvs := append(keyvals, l.suffix...)
	if len(kvs) == 0 {
		l.Warn(fmt.Sprint("keyvals not found: ", kvs))
		return nil
	}

	var (
		msg string
		ok  bool
	)
	if (len(kvs) & 1) == 1 {
		msg, ok = kvs[0].(string)
		if !ok {
			kvs = append(kvs, "msg not string or keyvals not paired")
		} else {
			kvs = kvs[1:]
		}
	}

	if l.hasValuer {
		bindValues(l.ctx, kvs)
	}

	var data []zap.Field
	for i := 0; i < len(kvs); i += 2 {
		data = append(data, zap.Any(fmt.Sprint(kvs[i]), kvs[i+1]))
	}

	switch level {
	case log.LevelDebug:
		l.Debug(msg, data...)
	case log.LevelInfo:
		l.Info(msg, data...)
	case log.LevelWarn:
		l.Warn(msg, data...)
	case log.LevelError:
		l.Error(msg, data...)
	case log.LevelFatal:
		l.Fatal(msg, data...)
	}
	return nil
}

// With with logger fields.
func With(l log.Logger, kv ...interface{}) log.Logger {
	if c, ok := l.(*logger); ok {
		kvs := make([]interface{}, 0, len(c.suffix)+len(kv))
		kvs = append(kvs, kv...)
		kvs = append(kvs, c.suffix...)
		return &logger{
			Logger:    c.Logger,
			suffix:    kvs,
			hasValuer: containsValuer(kvs),
			ctx:       c.ctx,
		}
	}
	return log.With(l)
}

func (l *logger) Sync() error {
	return l.Sync()
}

func bindValues(ctx context.Context, keyvals []interface{}) {
	for i := 1; i < len(keyvals); i += 2 {
		if v, ok := keyvals[i].(log.Valuer); ok {
			keyvals[i] = v(ctx)
		}
	}
}

func containsValuer(keyvals []interface{}) bool {
	for i := 1; i < len(keyvals); i += 2 {
		if _, ok := keyvals[i].(log.Valuer); ok {
			return true
		}
	}
	return false
}
