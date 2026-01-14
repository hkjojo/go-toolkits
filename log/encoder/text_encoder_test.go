package encoder

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"testing"
	"time"
)

func TestTextEncodeEntry(t *testing.T) {

	tests := []struct {
		desc     string
		expected string
		ent      zapcore.Entry
		fields   []zapcore.Field
	}{
		{
			desc: "info entry with some fields",
			expected: `{
				"L": "info",
				"T": "2025-02-20 20:28:42.000",
				"N": "bob",
				"M": "m1 tick chart",
			}`,
			ent: zapcore.Entry{
				Level:      zapcore.InfoLevel,
				Time:       time.Date(2025, 2, 20, 20, 28, 42, 99, time.UTC),
				LoggerName: "bob",
				Message:    "m1 tick chart",
			},
			fields: []zapcore.Field{
				zap.String("System", "Monitor"),
			},
		},
	}

	enc := NewTextEncoder(zapcore.EncoderConfig{
		MessageKey:     "M",
		LevelKey:       "L",
		TimeKey:        "T",
		NameKey:        "N",
		CallerKey:      "C",
		StacktraceKey:  "S",
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			buf, err := enc.EncodeEntry(tt.ent, tt.fields)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s", buf.String())
			buf.Free()
		})
	}
}
