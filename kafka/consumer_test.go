package kafka

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/Shopify/sarama"
)

type Person struct {
	Name string
	Age  int32
}

func TestConsumer(t *testing.T) {
	addrs := []string{"localhost:9092"}
	ctx, cancel := context.WithCancel(context.Background())
	consumer := NewConsumer[Person](ctx, addrs)

	topics := []string{"hale-topic"}
	sub, err := consumer.Subscribe(topics,
		// The message handler
		func(p *Person) error {
			log.Printf("received: %v", p)
			return nil
		},
		SubscribeErrHandler(func(err error) {
			log.Printf("error: %v", err)
		}),
		SubscribeStartHandler(func(s sarama.ConsumerGroupSession) {
			log.Printf("subscribe started")
		}),
		SubscribeEndHandler(func(s sarama.ConsumerGroupSession) {
			log.Printf("subscribe ended")
		}),
	)
	if err != nil {
		log.Fatalf("subscribe failed: %v", err)
	}

	t.Run("wait....", func(t *testing.T) {
		time.Sleep(time.Hour)
	})

	t.Run("pause and resume", func(t *testing.T) {
		time.Sleep(time.Minute)
		sub.Pause()
		log.Printf("sub paused...")

		time.Sleep(time.Minute)
		sub.Resume()
		log.Printf("sub resumed...")

		time.Sleep(time.Hour)
	})

	t.Run("unsubscribe one of two subs", func(t *testing.T) {
		_, err := consumer.Subscribe(append(topics, "fake-topic"),
			// The message handler
			func(p *Person) error {
				log.Printf("received2: %v", p)
				return nil
			},
			SubscribeErrHandler(func(err error) {
				log.Printf("error2: %v", err)
			}),
			SubscribeStartHandler(func(s sarama.ConsumerGroupSession) {
				log.Printf("subscribe2 started")
			}),
			SubscribeEndHandler(func(s sarama.ConsumerGroupSession) {
				log.Printf("subscribe2 ended")
			}),
		)
		if err != nil {
			log.Fatalf("subscribe2 failed: %v", err)
		}

		time.Sleep(time.Minute)
		_ = sub.Unsubscribe()
		log.Printf("unsubscribed sub...")

		time.Sleep(time.Hour)
	})

	t.Run("consumer close", func(t *testing.T) {
		_ = consumer.Close()
	})

	t.Run("close by context", func(t *testing.T) {
		cancel()
	})
}
