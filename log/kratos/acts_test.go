package kratos

import (
	"github.com/hkjojo/go-toolkits/apptools"
	tklog "github.com/hkjojo/go-toolkits/log/v2"
	"testing"
)

func TestTextLog(t *testing.T) {
	kfg := &tklog.Config{
		Path:   "./demo",
		Level:  "debug",
		Format: "text",
	}

	zLog, err := NewZapLog(kfg)
	if err != nil {
		t.Fatal(err)
	}
	finalLogger := apptools.WithMetaKeys(zLog)
	testLogger := NewActsHelper(finalLogger)
	testLogger.Infow("System", "DB", "this is a test")
}
