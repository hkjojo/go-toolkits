package log

import (
	"errors"
	"testing"
	"time"
)

func TestTelegram(t *testing.T) {
	l, _ := New(&Config{
		Format:        "json",
		DisableStdout: false,
		Caller:        true,
		WebHook: []*WebHookConfig{
			&WebHookConfig{
				Host:        "https://api.telegram.org/botXXX/sendMessage",
				Message:     "{\"parse_mode\":\"Markdown\",\"chat_id\":1067874870,\"text\": \"{{content}}\"}",
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
			},
		}})

	l.Sugar().DPanicw("quote error", "err", errors.New("test erro"))
	l.Sugar().Warnw("i am a robot", "name", "rot")
	time.Sleep(4 * time.Second)
}

func TestDingDing(t *testing.T) {
	l, _ := New(&Config{
		Format:        "",
		DisableStdout: false,
		Caller:        true,
		WebHook: []*WebHookConfig{
			&WebHookConfig{
				Host:        "https://oapi.dingtalk.com/robot/send?access_token=",
				Message:     "{\"msgtype\":\"markdown\",\"markdown\":{\"title\":\"log\",\"text\":\"{{content}}\"}}",
				Method:      "POST",
				ContentType: "application/json",
				KVMessage:   "- **{{key}}**: {{value}}\n",
				CoreConfig: CoreConfig{
					QueueLength: 10,
					Level:       "warn",
					Off:         false,
					Filter:      []string{"test"},
					Fields: map[string]string{
						"services": "user",
					},
				},
			},
		}})

	l.Sugar().DPanicw("quote error", "err", errors.New("test erro"))
	l.Sugar().Warnw("i am a robot", "name", "rot")
	time.Sleep(4 * time.Second)
}
