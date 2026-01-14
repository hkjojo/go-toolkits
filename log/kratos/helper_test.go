package kratos

import (
	"testing"
	"time"

	"github.com/hkjojo/go-toolkits/apptools"
	"go.uber.org/zap/zapcore"

	tklog "github.com/hkjojo/go-toolkits/log/v2"
	"github.com/hkjojo/go-toolkits/log/v2/encoder"
)

func TestConsoleLog(t *testing.T) {
	kfg := &tklog.Config{
		Path:   "./demo.log",
		Level:  "info",
		Format: "console",
	}

	zLog, err := NewZapLog(kfg)
	if err != nil {
		t.Fatal(err)
	}
	finalLogger := apptools.WithMetaKeys(zLog)
	testLogger := NewHelper(finalLogger)
	//testLogger = testLogger.WithSource("DB")
	testLogger.Infow("this is a test", "abcd", "efg", "ab", "cd")
	testLogger.Infof("this is a test, %s:%s, %s:%s", "abcd", "efg", "ab", "cd")
	testLogger.Warnf("this is a test")
}

func TestTextLog(t *testing.T) {
	kfg := &tklog.Config{
		Path:   "./demo.log",
		Level:  "info",
		Format: "text",
	}

	zLog, err := NewZapLog(kfg)
	if err != nil {
		t.Fatal(err)
	}
	finalLogger := apptools.WithMetaKeys(zLog)
	testLogger := NewHelper(finalLogger)

	//testLogger = testLogger.WithSource("DB")
	testLogger.Infow("this is a test", "abcd", "efg", "ab", "cd")
	testLogger.Infof("this is a test, %s:%s, %s:%s", "abcd", "efg", "ab", "cd")
	testLogger.Warnf("this is a test")
}

func TestHeaderTextLog(t *testing.T) {
	kfg := &tklog.Config{
		Path:   "./demo.log",
		Level:  "info",
		Format: "text",
	}
	withTextHeader := func(f encoder.HeaderFunc) tklog.Option {
		return func() *tklog.EncoderOptions {
			return &tklog.EncoderOptions{Text: []encoder.TextOption{encoder.WithHeader(f)}}
		}
	}
	// header time->|level-|>module->|source->|msg
	zLog, err := NewZapLog(kfg, withTextHeader(func(enc zapcore.PrimitiveArrayEncoder, ent zapcore.Entry, fields map[string]zapcore.Field) {
		enc.AppendString(ent.Time.UTC().Format(encoder.TextTimeLayout))
		enc.AppendString(encoder.SPLIT)
		enc.AppendString(ent.Level.CapitalString())
		enc.AppendString(encoder.SPLIT)

		module := fields["module"].String
		if module == "" {
			module = "System"
		}

		source := fields["ip"].String
		if source == "" {
			source = "-"
		}

		// avoid to duplicated print key-val
		delete(fields, "module")
		delete(fields, "ip")

		enc.AppendString(module)
		enc.AppendString(encoder.SPLIT)
		enc.AppendString(source)
		enc.AppendString(encoder.SPLIT)
	}))

	if err != nil {
		t.Fatal(err)
	}
	//finalLogger := apptools.WithMetaKeys(zLog)
	testLogger := NewHelper(zLog)

	//testLogger = testLogger.WithSource("DB")
	testLogger.With("ip", "127.0.0.1").Infow("this is a test", "abcd", "efg", "ab", "cd")
	testLogger.Infof("this is a test, %s:%s, %s:%s", "abcd", "efg", "ab", "cd")
	testLogger.Warnf("this is a test")
}

func TestQueryLog(t *testing.T) {
	startTime := time.Now()
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)
	from := dayStart.Format(encoder.TextTimeLayout)
	to := dayEnd.Format(encoder.TextTimeLayout)
	resp, err := encoder.QueryLogs(&encoder.ListLogReq{
		From:       from,
		To:         to,
		Level:      nil,
		Message:    nil,
		Separator:  nil,
		FieldNames: []string{"time", "level", "message"},
		Filters:    nil,
	}, "./history.log")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("elapsed:", time.Since(startTime).String())

	t.Logf("count: %d", len(resp.Logs))
	/*for _, re := range resp.Logs {
		t.Logf("%+v", re)
	}*/
}
