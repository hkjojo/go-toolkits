package kafka

import (
	"testing"
	"time"
)

const testTopic = "test-topic"

var (
	testAddrs = []string{
		"localhost:9092",
	}
	testTopics = []string{
		testTopic,
	}
)

func Test_Producer(t *testing.T) {
	producer, err := NewProducer(testAddrs)
	if err != nil {
		t.Fatal(err)
	}

	timer := time.NewTimer(time.Second * 30)
	defer func() {
		timer.Stop()
		producer.Close()
		time.Sleep(time.Second)
	}()

	for {
		select {
		case <-timer.C:
			return
		default:
		}

		err := producer.PublishString(testTopic, "test producer")
		if err != nil {
			t.Log(err)
		}
		time.Sleep(time.Second * 1)
	}
}

func Test_Consumer(t *testing.T) {
	consumer, err := NewConsumer(testAddrs, testTopics, "testGroup")
	if err != nil {
		t.Fatal(err)
	}

	timer := time.NewTimer(time.Second * 30)
	defer func() {
		timer.Stop()
		consumer.Close()
		time.Sleep(time.Second)
	}()

	consumer.Run()
	<-timer.C
}
