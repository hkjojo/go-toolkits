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

// NewZapLog returns a Kratos log.Logger backed by tklog (zap).
//
// Two call patterns are supported in Log() below:
//   - hkjojo Helper (odd-length kvs, tail msgkey)
//   - Kratos standard log.Helper / framework (even-length kvs, leading "msg" pair)
//
// Caller skip is applied here so that log entries report the user code line,
// not this adapter file. The skip value (3) is calibrated for the Kratos
// log.Helper.Infow path (helper.go → kratos.logger.Log → tklog.Logger.Info),
// matching the previous TradePass pkg/logger compat shim's behavior. Direct
// framework logs (no Helper wrapper) report 1-2 frames too deep — accepted
// trade-off, framework log volume is low.
func NewZapLog(cfg *tklog.Config, opts ...tklog.Option) (log.Logger, error) {
	tklogger, err := tklog.New(cfg, opts...)
	if err != nil {
		return nil, err
	}

	if cfg != nil && cfg.Caller {
		tklogger.Logger = tklogger.Logger.WithOptions(zap.AddCallerSkip(3))
	}

	return &logger{Logger: tklogger}, nil
}

var _ log.Logger = (*logger)(nil)

// Log impl kratos log, supporting both call patterns:
//   - Odd-length kvs (hkjojo Helper): tail-position msgkey-typed value carries
//     the message. Helper.Infow appends a msgkey at the end.
//   - Even-length kvs (Kratos standard log.Helper / framework): leading
//     ("msg", string) pair carries the message. log.Helper.Infow does
//     append(kvs, "msg", msg) to construct kvs, but Kratos framework code
//     also commonly emits ("msg", X, k1, v1, ...) directly via logger.Log.
//
// Without the even-length branch, the leading "msg" pair would be emitted as
// a regular zap.Field while the zap encoder also writes its own MessageKey:"msg"
// for the empty Entry.Message → produces double-msg key in JSON output.
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
		// odd-length: hkjojo Helper convention — find msgkey at tail
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
	} else if ll >= 2 {
		// even-length: Kratos framework convention — leading ("msg", X) pair
		if k, kok := kvs[0].(string); kok && k == "msg" {
			if v, vok := kvs[1].(string); vok {
				msg = msgkey(v)
				kvs = kvs[2:]
			}
		}
	}

	var data []zap.Field
	for i := 0; i < len(kvs); i += 2 {
		data = append(data, zap.Any(fmt.Sprint(kvs[i]), kvs[i+1]))
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

// Sync flushes buffered log entries.
//
// Calls the embedded tklog.Logger.Sync (which delegates to the underlying
// *zap.Logger.Sync via tklog.Logger's own embedded *zap.Logger). The previous
// implementation `return l.Sync()` recursed back into this very method —
// any caller of Sync() would hit a stack overflow. Caught during the
// system_log via Redis stream rollout (ADR-0042).
func (l *logger) Sync() error {
	return l.Logger.Sync()
}
