package kratos

import (
	"testing"
	"time"

	pbc "git.gonit.codes/dealer/actshub/protocol/go/common/v1"
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
		enc.AppendString(ent.Time.UTC().Format(encoder.TextTimeFormat))
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
	//status := "INFO"
	//msg := "1633261"
	startTime := time.Now()
	resp, err := encoder.QueryLogs(&pbc.ListLogReq{
		From: "2025-05-27T00:00:00.000Z",
		To:   "2025-05-27T23:00:00.000Z",
		//Status: &status,
		//Message: &msg,
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
