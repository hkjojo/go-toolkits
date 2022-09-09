package kafka

import (
	"log"
	"testing"
	"time"
)

func TestProducer(t *testing.T) {
	addrs := []string{"localhost:9092"}
	producer, err := NewProducer(addrs,
		PublishErrHandler(func(err error) {
			log.Printf("error: %v", err)
		}),
	)
	if err != nil {
		log.Fatalf("new producer error: %v", err)
	}

	t.Run("", func(t *testing.T) {
		err = producer.PublishAsync("hale-topic", &Person{
			Name: "aaa",
			Age:  100,
		})
		if err != nil {
			log.Fatalf("async publish failed: %v", err)
		}

		time.Sleep(time.Second)
		_ = producer.Close()
	})

	t.Run("", func(t *testing.T) {
		err = producer.Publish("hale-topic", &Person{
			Name: "aaa",
			Age:  100,
		})
		if err != nil {
			log.Fatalf("publish failed: %v", err)
		}
	})

}
