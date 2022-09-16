package kafka

import (
	"context"
	"fmt"

	"github.com/Shopify/sarama"
)

type Producer struct {
	codec  Codec
	cancel context.CancelFunc

	sp sarama.SyncProducer
	ap sarama.AsyncProducer
}

func NewProducer(addrs []string, opts ...PubOption) (*Producer, error) {
	pubOpts := GetDefaultPubOpts()
	for _, opt := range opts {
		_ = opt(pubOpts)
	}
	if pubOpts.conf == nil {
		pubOpts.conf = sarama.NewConfig()
	}
	pubOpts.conf.Producer.Return.Successes = true

	pc, err := sarama.NewClient(addrs, pubOpts.conf)
	if err != nil {
		return nil, err
	}
	ap, err := sarama.NewAsyncProducerFromClient(pc)
	if err != nil {
		return nil, err
	}
	sp, err := sarama.NewSyncProducerFromClient(pc)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(pubOpts.ctx)
	go func() {
		for {
			select {
			case err := <-ap.Errors():
				if pubOpts.asyncErrorCB != nil {
					pubOpts.asyncErrorCB(err.Msg, err.Err)
				}
			case msg := <-ap.Successes():
				if pubOpts.asyncSuccessCB != nil {
					pubOpts.asyncSuccessCB(msg)
				}

			case <-ctx.Done():
				_ = sp.Close()
				_ = ap.Close()
				_ = pc.Close()
				return
			}
		}
	}()

	return &Producer{ap: ap, sp: sp, codec: codec{}, cancel: cancel}, nil
}

func (p *Producer) Publish(topic string, msg any) (err error) {
	data, err := p.codec.Encode(msg)
	if err != nil {
		return fmt.Errorf("encode msg failed: %w", err)
	}

	_, _, err = p.sp.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
	})
	return
}

func (p *Producer) PublishAsync(topic string, msg any) error {
	data, err := p.codec.Encode(msg)
	if err != nil {
		return fmt.Errorf("encode msg failed: %w", err)
	}

	p.ap.Input() <- &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
	}
	return nil
}

func (p *Producer) Close() error {
	p.cancel()
	return nil
}

// GORGEOUS DIVIDING LINE -------------------------------------------------

type PubOpts struct {
	ctx  context.Context
	conf *sarama.Config

	asyncErrorCB   func(*sarama.ProducerMessage, error)
	asyncSuccessCB func(*sarama.ProducerMessage)
}

func GetDefaultPubOpts() *PubOpts {
	return &PubOpts{ctx: context.Background()}
}

type PubOption func(*PubOpts) error

func WithPubContext(ctx context.Context) PubOption {
	return func(opts *PubOpts) error {
		opts.ctx = ctx
		return nil
	}
}

// WithPubConfig is used to specify the sarama.Config.
// It's best to use the function sarama.NewConfig() to create a new Config,
// which has some default value, then specifies your customized items.
// NOTE: You can't specify the value of conf.Producer.Return.Successes which is always true.
func WithPubConfig(conf *sarama.Config) PubOption {
	return func(opts *PubOpts) error {
		sarama.NewConfig()
		opts.conf = conf
		return nil
	}
}

// AsyncPublishErrHandler specifies an error callback function for the Producer.
// NOTE: It just works for PublishAsync, And does not perform time-consuming operations in it.
func AsyncPublishErrHandler(cb func(*sarama.ProducerMessage, error)) PubOption {
	return func(opts *PubOpts) error {
		opts.asyncErrorCB = cb
		return nil
	}
}

// AsyncPublishSuccessHandler specifies a success callback function for the Producer.
// NOTE: It just works for PublishAsync, And does not perform time-consuming operations in it.
func AsyncPublishSuccessHandler(cb func(message *sarama.ProducerMessage)) PubOption {
	return func(opts *PubOpts) error {
		opts.asyncSuccessCB = cb
		return nil
	}
}
