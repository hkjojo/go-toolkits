package kafka

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Shopify/sarama"
)

// errors defined
var (
	ErrAlreadyClosed = errors.New("producer already closed")
)

type Option func(*sarama.Config)

// Producer ...
type Producer struct {
	ap     sarama.AsyncProducer
	closed bool
}

// NewProducer ...
func NewProducer(hosts []string, options ...Option) (*Producer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Partitioner = sarama.NewRandomPartitioner
	cfg.Producer.Return.Successes = true
	cfg.Producer.Timeout = time.Microsecond * 100
	cfg.Version = sarama.V2_4_0_0
	for _, o := range options {
		o(cfg)
	}

	p, err := sarama.NewAsyncProducer(hosts, cfg)
	if err != nil {
		return nil, err
	}

	producer := &Producer{ap: p}
	go producer.run()
	return producer, nil
}

// Run ...
func (p *Producer) run() {
	success := p.ap.Successes()
	errors := p.ap.Errors()
	defer fmt.Println("producer loop stop")

	for {
		select {
		case _, ok := <-success:
			if !ok {
				return
			}
		case err, ok := <-errors:
			if !ok {
				return
			}

			log.Printf("produce message fail, error: %s\n", err.Error())
		}
	}
}

// Publish ...
func (p *Producer) Publish(topic string, data interface{}) error {
	if p.closed {
		return ErrAlreadyClosed
	}

	encodeData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{}
	msg.Topic = topic
	msg.Value = sarama.ByteEncoder(encodeData)

	p.ap.Input() <- msg
	return nil
}

// PublishString ...
func (p *Producer) PublishString(topic, message string) error {
	if p.closed {
		return ErrAlreadyClosed
	}

	msg := &sarama.ProducerMessage{}
	msg.Topic = topic
	msg.Value = sarama.StringEncoder(message)

	p.ap.Input() <- msg
	return nil
}

// Close ...
func (p *Producer) Close() error {
	p.closed = true
	return p.ap.Close()
}
