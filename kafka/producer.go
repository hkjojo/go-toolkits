package kafka

import (
	"context"
	"fmt"

	"github.com/Shopify/sarama"
)

type Producer struct {
	codec Codec

	sp sarama.SyncProducer
	ap sarama.AsyncProducer
}

func NewProducer(ctx context.Context, addrs []string, opts ...PubOption) (*Producer, error) {
	pubOpts := &PubOpts{}
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

	return &Producer{codec: codec{}, ap: ap, sp: sp}, nil
}

func (p *Producer) Publish(topic string, msg interface{}) (err error) {
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

func (p *Producer) PublishAsync(topic string, msg interface{}) error {
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

// GORGEOUS DIVIDING LINE -------------------------------------------------

type PubOpts struct {
	asyncErrorCB PubErrorHandler
}

type PubOption func(*PubOpts) error

type PubErrorHandler func(error)

// PublishErrHandler specify an error handler for the Producer.
// NOTE: Do not perform time-consuming operations in PubErrorHandler.
func PublishErrHandler(cb PubErrorHandler) PubOption {
	return func(opts *PubOpts) error {
		opts.asyncErrorCB = cb
		return nil
	}
}
