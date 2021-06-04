package kafka

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/Shopify/sarama"
)

// Consumer ...
type Consumer struct {
	cg        sarama.ConsumerGroup
	cancel    context.CancelFunc
	topics    []string
	Reconnect time.Duration
}

// NewConsumer ...
func NewConsumer(hosts, topics []string, groupName string, options ...Option) (*Consumer, error) {
	cfg := sarama.NewConfig()
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	cfg.Consumer.Group.Session.Timeout = 10 * time.Second
	cfg.Consumer.Group.Heartbeat.Interval = 5 * time.Second
	cfg.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
	cfg.Consumer.Group.Rebalance.Timeout = 30 * time.Second
	cfg.Consumer.Group.Rebalance.Retry.Max = 5
	cfg.Consumer.Group.Rebalance.Retry.Backoff = 2 * time.Second
	cfg.Version = sarama.V2_4_0_0
	for _, o := range options {
		o(cfg)
	}

	client, err := sarama.NewClient(hosts, cfg)
	if err != nil {
		return nil, err
	}

	cg, err := sarama.NewConsumerGroupFromClient(groupName, client)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		topics:    topics,
		cg:        cg,
		Reconnect: cfg.Consumer.Group.Heartbeat.Interval,
	}, nil
}

// Run ...
func (c *Consumer) Run(handler sarama.ConsumerGroupHandler) {
	if c.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go func() {
		fmt.Println("kafka consume start...")
		for {
			if err := c.cg.Consume(
				context.Background(),
				c.topics,
				handler,
			); err != nil && !errors.Is(err, io.EOF) {
				log.Printf("kafka consume fail, error: %s\n", err.Error())
				time.Sleep(c.Reconnect)
			}

			select {
			case <-ctx.Done():
				fmt.Println("kafka consume stop...")
				return
			default:
				fmt.Println("kafka consume retry...")
			}

		}
	}()
}

// Close ...
func (c *Consumer) Close() {
	c.cancel()
	c.cancel = nil
	c.cg.Close()
}

// ConsumHandler ...
type ConsumHandler struct {
}

// ConsumeClaim ..
func (h *ConsumHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	fmt.Println("*start consume*")
	for msg := range claim.Messages() {
		fmt.Printf(
			"===================================================\ntopic:%s\nmessage:%s\ntime:%s\n\n",
			msg.Topic,
			msg.Value,
			msg.Timestamp.String(),
		)
		sess.MarkMessage(msg, "")
	}
	return nil
}

// Setup ..
func (h *ConsumHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup ..
func (h *ConsumHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}
