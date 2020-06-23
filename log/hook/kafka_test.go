package hook

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestKafka(t *testing.T) {
	h, err := NewKafkaCore(&KafkaConfig{
		Hosts: []string{"localhost:9092"},
		Topic: "app-log",
		CoreConfig: CoreConfig{
			QueueLength: 100000,
			Level:       "info",
			Off:         false,
			Filter:      []string{"test"},
		},
		MergeData: true,
	},
		"what-gotrading-engine-",
		map[string]string{
			"services": "app",
		},
		zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.RFC3339TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	)
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		desc     string
		expected string
		data     *CoreData
	}{
		{
			desc:     "log",
			expected: `{"level":"INFO","time":"2018-06-19T16:33:42Z","logger":"bob","msg":"lob law","data":"name:user age:12 cash:1000.12 null_value:<nil> create_time:2018-06-19 16:33:42.000000099 +0000 UTC array_with_null_elements:[0x197c9f0 <nil> <nil> 2] error:user not found data:{i am a good student 10}"}`,
			data: &CoreData{
				entry: zapcore.Entry{
					Level:      zapcore.InfoLevel,
					Time:       time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
					LoggerName: "bob",
					Message:    "lob law",
				},
				fields: []zapcore.Field{
					zap.String("name", "user"),
					zap.Int("age", 12),
					zap.Float64("cash", 1000.12),
					zap.Reflect("null_value", nil),
					zap.Time("create_time", time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC)),
					zap.Reflect("array_with_null_elements", []interface{}{&struct{}{}, nil, (*struct{})(nil), 2}),
					zap.Error(errors.New("user not found")),
					zap.Reflect("data", struct {
						string
						int
					}{"i am a good student", 10}),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			value, err := h.encode(tt.data)
			if err != nil {
				t.Fatal(err)
			}
			tt.expected += string(value[len(value)-1])
			if value != tt.expected {
				t.Fatalf("\nexpected:%s get:%s\n", value, tt.expected)
			}
			//h.write(value)
		})
	}
}
