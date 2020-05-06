package log

import (
	"testing"
	"time"
)

func TestKafka(t *testing.T) {
	l, _ := New(&Config{
		Format:        "json",
		DisableStdout: true,
		Fields: map[string]string{
			"services": "app",
		},
		Prefix: "what-gotrading-engine-",
		Kafka: &KafkaConfig{
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
	})

	ll := l.Sugar().With("this key", "this value")
	ll.Infow("i am msg", "integer", 1222, "float", 1.23, "timen", time.Now())
	time.Sleep(5 * time.Second)
}
