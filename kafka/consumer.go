package kafka

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/google/uuid"
)

type Subscription interface {
	Unsubscribe() error
	Pause()
	Resume()
}

// Subscribe
// 1. If there are multiple subscriptions, all of them will receive the messages.
// 2. The callback function is called once a message arrived,
//    and if its return is nil, the message will be marked as consumed automatically.
func Subscribe[T any](addrs, topics []string, cb func(t *T) error, opts ...SubOption) (Subscription, error) {
	groupID := uuid.NewString()
	return subscribe(addrs, topics, groupID, cb, opts...)
}

// SubscribeQueue
// 1. If there are multiple subscriptions, only one of them will receive the message.
// 2. The callback function is called once a message arrived,
//    and if its return is nil, the message will be marked as consumed automatically.
func SubscribeQueue[T any](addrs, topics []string, cb func(t *T) error, opts ...SubOption) (Subscription, error) {
	sort.Strings(topics)
	groupID := fmt.Sprintf("%s", md5.Sum([]byte(strings.Join(topics, "-"))))
	return subscribe(addrs, topics, groupID, cb, opts...)
}

func subscribe[T any](addrs, topics []string, groupID string, cb func(t *T) error, opts ...SubOption) (Subscription, error) {
	if len(topics) == 0 {
		return nil, errors.New("empty topics")
	}

	subOpts := GetDefaultSubOpts()
	for _, opt := range opts {
		_ = opt(subOpts)
	}
	if subOpts.conf == nil {
		subOpts.conf = sarama.NewConfig()
	}
	subOpts.conf.Consumer.Return.Errors = true

	consumerGrp, err := sarama.NewConsumerGroup(addrs, groupID, subOpts.conf)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(subOpts.ctx)
	go func() {
		for {
			select {
			case err := <-consumerGrp.Errors():
				if subOpts.asyncErrorCB != nil {
					subOpts.asyncErrorCB(err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	consumer := consumerGroupHandler[T]{handler: cb, subOpts: subOpts, codec: codec{}}
	go func() {
		defer func() {
			_ = consumerGrp.Close()
		}()

		for {
			// `Consume` should be called inside an infinite loop, when a
			// server-side rebalance happens, the consumer session will need to be
			// recreated to get the new claims.
			if err := consumerGrp.Consume(ctx, topics, &consumer); err != nil {
				if consumer.subOpts.asyncErrorCB != nil {
					consumer.subOpts.asyncErrorCB(fmt.Errorf("consume error: %w", err))
				}
			}
			// Check if context was cancelled, signaling that the consumer should stop.
			if ctx.Err() != nil {
				return
			}
		}
	}()

	return &subscription{consumerGrp: consumerGrp, cancel: cancel}, nil
}

// GORGEOUS DIVIDING LINE -------------------------------------------------

type SubOpts struct {
	ctx  context.Context
	conf *sarama.Config

	sessionStartCB func()
	sessionEndCB   func()
	asyncErrorCB   func(error)
}

func GetDefaultSubOpts() *SubOpts {
	return &SubOpts{ctx: context.Background()}
}

type SubOption func(*SubOpts) error

func WithSubContext(ctx context.Context) SubOption {
	return func(opts *SubOpts) error {
		opts.ctx = ctx
		return nil
	}
}

// WithSubConfig is used to specify the sarama.Config.
// It's best to use the function sarama.NewConfig() to create a new Config,
// which has some default value, then specifies your customized items.
// NOTE: You can't specify the value of conf.Consumer.Return.Errors which is always true.
func WithSubConfig(conf *sarama.Config) PubOption {
	return func(opts *PubOpts) error {
		sarama.NewConfig()
		opts.conf = conf
		return nil
	}
}

func SubscribeStartHandler(cb func()) SubOption {
	return func(opts *SubOpts) error {
		opts.sessionStartCB = cb
		return nil
	}
}

func SubscribeEndHandler(cb func()) SubOption {
	return func(opts *SubOpts) error {
		opts.sessionEndCB = cb
		return nil
	}
}

// SubscribeErrHandler specifies an error callback function for the subscription.
// NOTE: Does not perform time-consuming operations in SubErrorHandler.
func SubscribeErrHandler(cb func(err error)) SubOption {
	return func(opts *SubOpts) error {
		opts.asyncErrorCB = cb
		return nil
	}
}

// GORGEOUS DIVIDING LINE -------------------------------------------------

type subscription struct {
	cancel      context.CancelFunc
	consumerGrp sarama.ConsumerGroup
}

func (s *subscription) Unsubscribe() error {
	s.cancel()
	return nil
}

func (s *subscription) Pause() {
	s.consumerGrp.PauseAll()
}

func (s *subscription) Resume() {
	s.consumerGrp.ResumeAll()
}

// GORGEOUS DIVIDING LINE -------------------------------------------------

type consumerGroupHandler[T any] struct {
	handler func(t *T) error
	subOpts *SubOpts
	codec   Codec
}

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (c *consumerGroupHandler[T]) Setup(_ sarama.ConsumerGroupSession) error {
	if c.subOpts.sessionStartCB != nil {
		c.subOpts.sessionStartCB()
	}
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited.
func (c *consumerGroupHandler[T]) Cleanup(_ sarama.ConsumerGroupSession) error {
	if c.subOpts.sessionEndCB != nil {
		c.subOpts.sessionEndCB()
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
