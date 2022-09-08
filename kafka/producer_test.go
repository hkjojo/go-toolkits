package kafka

import (
	"context"
	"log"
	"testing"
	"time"
)

func TestProducer(t *testing.T) {
	addrs := []string{"localhost:9092"}
	ctx, cancel := context.WithCancel(context.Background())
	producer, err := NewProducer(ctx, addrs,
		PublishErrHandler(func(err error) {
			log.Printf("error: %v", err)
		}),
	)
	if err != nil {
		log.Fatalf("new producer error: %v", err)
	}

	err = producer.PublishAsync("hale-topic", &Person{
		Name: "aaa",
		Age:  100,
	})
	if err != nil {
		log.Fatalf("publish failed: %v", err)
	}

	time.Sleep(time.Second)
	cancel()
}
