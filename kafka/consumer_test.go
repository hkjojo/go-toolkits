package kafka

import (
	"log"
	"testing"
	"time"
)

type Person struct {
	Name string
	Age  int32
}

func TestConsumer(t *testing.T) {
	addrs := []string{"localhost:9092"}
	topics := []string{"hale-topic"}
	sub, err := Subscribe[Person](addrs, topics,
		// messages handler
		func(p *Person) error {
			return nil
		},
		SubscribeErrHandler(func(err error) {
			log.Printf("error: %v", err)
		}),
		SubscribeStartHandler(func() {
			log.Printf("subscribe started")
		}),
		SubscribeEndHandler(func() {
			log.Printf("subscribe ended")
		}),
	)
	if err != nil {
		log.Fatalf("subscribe failed: %v", err)
	}
	defer func() {
		_ = sub.Unsubscribe()
	}()

	t.Run("wait...", func(t *testing.T) {
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

}
