package hook

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestTelegram(t *testing.T) {
	h := NewWebHookCore(&WebHookConfig{
		Host:        "https://api.telegram.org/botxxxx/sendMessage",
		Message:     "{\"parse_mode\":\"Markdown\",\"chat_id\":xxxx,\"text\": \"{{content}}\"}",
		Method:      "POST",
		ContentType: "application/json",
		KVMessage:   "***{{key}}***: {{value}}\n",
		CoreConfig: CoreConfig{
			QueueLength: 10,
			Level:       "warn",
			Off:         false,
			Filter:      []string{"test"},
			Fields: map[string]string{
				"services": "user",
			},
		},
	}, zapcore.EncoderConfig{
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
	})

	tests := []struct {
		desc     string
		expected string
		data     *CoreData
	}{
		{
			desc: "log",
			expected: `{"parse_mode":"Markdown","chat_id":1067874870,"text": "***name***: user
			***age***: 12
			***cash***: 1000.12
			***null_value***: <nil>
			***create_time***: 2018-06-19 16:33:42.000000099 +0000 UTC
			***array_with_null_elements***: [0x197b9f0 <nil> <nil> 2]
			***error***: user not found
			***data***: {i am a good student 10}
			"}`,
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
			value := h.encode(tt.data)
			// if value != tt.expected {
			// 	t.Fatal(len(value), len(tt.expected))
			// }
			t.Log(value)
			//h.write(value)
		})
	}
}
