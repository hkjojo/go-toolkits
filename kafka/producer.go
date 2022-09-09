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

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	pc, err := sarama.NewClient(addrs, config)
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
					pubOpts.asyncErrorCB(err)
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
	ctx context.Context

	asyncErrorCB func(error)
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

// PublishErrHandler specify an error handler for the Producer.
// NOTE: Do not perform time-consuming operations in PubErrorHandler.
func PublishErrHandler(cb func(error)) PubOption {
	return func(opts *PubOpts) error {
		opts.asyncErrorCB = cb
		return nil
	}
}
