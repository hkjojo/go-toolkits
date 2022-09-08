package kafka

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Shopify/sarama"
)

type Consumer[T any] struct {
	ctx    context.Context
	cancel context.CancelFunc
	addrs  []string
	conf   *sarama.Config

	cgs map[string]sarama.ConsumerGroup
}

func NewConsumer[T any](ctx context.Context, addrs []string) *Consumer[T] {
	conf := sarama.NewConfig()
	conf.Consumer.Return.Errors = true

	ctxChild, ctxCancel := context.WithCancel(ctx)
	return &Consumer[T]{
		ctx:    ctxChild,
		cancel: ctxCancel,
		addrs:  addrs,
		conf:   conf,
		cgs:    map[string]sarama.ConsumerGroup{},
	}
}

type Subscription struct {
	cancel context.CancelFunc
	cg     sarama.ConsumerGroup
}

// Unsubscribe TODO What do I need to do here ?
func (s *Subscription) Unsubscribe() error {
	s.cancel()
	return nil
}

func (s *Subscription) Pause() {
	s.cg.PauseAll()
}

func (s *Subscription) Resume() {
	s.cg.ResumeAll()
}

// Subscribe subscribes to the topics' messages.
// The handler function is called once a message arrived, and if the return is nil,
// the message will be marked as consumed automatically.
func (c *Consumer[T]) Subscribe(topics []string, handler func(t *T) error, opts ...SubOption) (*Subscription, error) {
	if len(topics) == 0 {
		return nil, errors.New("topics empty")
	}

	subOpts := &SubOpts{}
	for _, opt := range opts {
		_ = opt(subOpts)
	}

	groupKey := subKey(topics)
	cg, err := sarama.NewConsumerGroup(c.addrs, groupKey, c.conf)
	if err != nil {
		return nil, err
	}

	ctxChild, ctxCancel := context.WithCancel(c.ctx)
	go func() {
		for {
			select {
			case err := <-cg.Errors():
				if subOpts.asyncErrorCB != nil {
					subOpts.asyncErrorCB(err)
				}
			case <-ctxChild.Done():
				return
			}
		}
	}()

	consumer := consumerGroupHandler[T]{handler: handler, subOpts: subOpts, codec: codec{}}
	go func() {
		defer func() {
			_ = cg.Close()
		}()

		for {
			// `Consume` should be called inside an infinite loop, when a
			// server-side rebalance happens, the consumer session will need to be
			// recreated to get the new claims.
			if err := cg.Consume(ctxChild, topics, &consumer); err != nil {
				if consumer.subOpts.asyncErrorCB != nil {
					consumer.subOpts.asyncErrorCB(fmt.Errorf("consume error: %w", err))
				}
			}
			// Check if context was cancelled, signaling that the consumer should stop.
			if ctxChild.Err() != nil {
				return
			}
		}
	}()

	return &Subscription{cg: cg, cancel: ctxCancel}, nil
}

func (c *Consumer[T]) Close() error {
	c.cancel()
	return nil
}

func subKey(topics []string) string {
	sort.Strings(topics)
	return fmt.Sprintf("%s", md5.Sum([]byte(strings.Join(topics, "-"))))
}

// GORGEOUS DIVIDING LINE -------------------------------------------------

type consumerGroupHandler[T any] struct {
	handler func(t *T) error
	subOpts *SubOpts
	codec   Codec
}

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (c *consumerGroupHandler[T]) Setup(session sarama.ConsumerGroupSession) error {
	if c.subOpts.sessionStartCB != nil {
		c.subOpts.sessionStartCB(session)
	}
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited.
func (c *consumerGroupHandler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	if c.subOpts.sessionEndCB != nil {
		c.subOpts.sessionEndCB(session)
	}
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (c *consumerGroupHandler[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// NOTE: Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/main/consumer_group.go#L27-L29
	for {
		select {
		case message := <-claim.Messages():
			t := new(T)
			if err := c.codec.Decode(message.Value, t); err != nil {
				if c.subOpts.asyncErrorCB != nil {
					c.subOpts.asyncErrorCB(fmt.Errorf("decode message error: %w", err))
				}
				continue
			}

			if err := c.handler(t); err == nil {
				session.MarkMessage(message, "")
			}

		// Should return when `session.Context()` is done.
		// If not, will raise `ErrRebalanceInProgress` or `read tcp <ip>:<port>: i/o timeout` when kafka rebalance. see:
		// https://github.com/Shopify/sarama/issues/1192
		case <-session.Context().Done():
			return nil
		}
	}
}

// GORGEOUS DIVIDING LINE -------------------------------------------------

// TODO
// type ConsumePolicy int
//
// const (
// 	// ConsumeLastPolicy will start the consumer with the last sequence received.
// 	ConsumeLastPolicy ConsumePolicy = iota
//
// 	// ConsumeNewPolicy will only deliver new messages that are sent after the consumer is created.
// 	ConsumeNewPolicy
// )

type SubOpts struct {
	// consumePolicy  ConsumePolicy TODO
	sessionStartCB SubSessionHandler
	sessionEndCB   SubSessionHandler
	asyncErrorCB   SubErrorHandler
}

type SubOption func(*SubOpts) error

// TODO
// func SubConsumeLast() SubOption {
// 	return func(o *SubOpts) error {
// 		o.consumePolicy = ConsumeLastPolicy
// 		return nil
// 	}
// }
//
// func SubConsumeNew() SubOption {
// 	return func(o *SubOpts) error {
// 		o.consumePolicy = ConsumeNewPolicy
// 		return nil
// 	}
// }

type SubSessionHandler func(sarama.ConsumerGroupSession)

func SubscribeStartHandler(cb SubSessionHandler) SubOption {
	return func(opts *SubOpts) error {
		opts.sessionStartCB = cb
		return nil
	}
}

func SubscribeEndHandler(cb SubSessionHandler) SubOption {
	return func(opts *SubOpts) error {
		opts.sessionEndCB = cb
		return nil
	}
}

type SubErrorHandler func(error)

// SubscribeErrHandler specify an error handler for the Subscription.
// NOTE: Do not perform time-consuming operations in SubErrorHandler.
func SubscribeErrHandler(cb SubErrorHandler) SubOption {
	return func(opts *SubOpts) error {
		opts.asyncErrorCB = cb
		return nil
	}
}
