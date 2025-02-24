package kratos

import (
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"

	tklog "github.com/hkjojo/go-toolkits/log/v2"
)

type logger struct {
	*tklog.Logger
}

func NewZapLog(cfg *tklog.Config) (log.Logger, error) {
	tklogger, err := tklog.New(cfg)
	if err != nil {
		return nil, err
	}

	return &logger{Logger: tklogger}, nil
}

var _ log.Logger = (*logger)(nil)

// Log impl kratos log, but try to find a msg key
func (l *logger) Log(level log.Level, kvs ...interface{}) error {
	ll := len(kvs)
	if ll == 0 {
		l.Warn(fmt.Sprint("keyvals not found: ", kvs))
		return nil
	}

	var (
		msg msgkey
		ok  bool
	)

	if (ll & 1) == 1 {
		// find msgkey from reverse sort
		for i := ll - 1; i >= 0; i-- {
			msg, ok = kvs[i].(msgkey)
			if ok {
				kvs = kvs[0 : ll-1]
				break
			}
		}

		if msg == "" {
			kvs = append(kvs, "invalid keyvals")
		}
	}

	var data []zap.Field

	switch kvs[0].(type) {
	case LogType:
		if len(kvs) == 2 {
			data = append(data, zap.Any(TypeKey, kvs[0]))
			data = append(data, zap.Any(SourceKey, kvs[1]))
		} else {
			for i := 0; i < len(kvs); i += 2 {
				data = append(data, zap.Any(fmt.Sprint(kvs[i]), kvs[i+1]))
			}
		}
	default:
		for i := 0; i < len(kvs); i += 2 {
			data = append(data, zap.Any(fmt.Sprint(kvs[i]), kvs[i+1]))
		}
	}

	m := string(msg)
	switch level {
	case log.LevelDebug:
		l.Debug(m, data...)
	case log.LevelInfo:
		l.Info(m, data...)
	case log.LevelWarn:
		l.Warn(m, data...)
	case log.LevelError:
		l.Error(m, data...)
	case log.LevelFatal:
		l.Fatal(m, data...)
	}
	return nil
}

func (l *logger) Sync() error {
	return l.Sync()
}
