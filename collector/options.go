package collector

import (
	"time"

	"github.com/micro/go-micro/v2/broker"
	"github.com/micro/go-micro/v2/codec"
	mproto "github.com/micro/go-micro/v2/codec/proto"

	"github.com/golang/protobuf/proto"
)

type config struct {
	codec           codec.Marshaler
	broker          broker.Broker
	collectInterval time.Duration
	topic           string
	queueSize       int
	errorFunc       func(err error)
	endpoints       []*endpoint
}

type endpoint struct {
	f               func() proto.Message
	collectInterval time.Duration
}

// Option ...
type Option func(*config)

func defaultConfig() *config {
	return &config{
		collectInterval: time.Minute,
		queueSize:       2048,
		codec:           mproto.Marshaler{},
	}
}

// WithTopic ...
func WithTopic(topic string) Option {
	return func(cfg *config) {
		cfg.topic = topic
	}
}

// WithBroker ...
func WithBroker(broker broker.Broker) Option {
	return func(cfg *config) {
		cfg.broker = broker
	}
}

// WithInterval ...
func WithInterval(d time.Duration) Option {
	return func(cfg *config) {
		cfg.collectInterval = d
	}
}

// WithEndpoint ...
func WithEndpoint(f func() proto.Message) Option {
	return func(cfg *config) {
		cfg.endpoints = append(cfg.endpoints, &endpoint{f, 0})
	}
}

// WithIntervalEndpoint ...
func WithIntervalEndpoint(interval time.Duration, f func() proto.Message) Option {
	return func(cfg *config) {
		cfg.endpoints = append(cfg.endpoints, &endpoint{f, interval})
	}
}

// WithMsgCodec ...
func WithMsgCodec(codec codec.Marshaler) Option {
	return func(cfg *config) {
		cfg.codec = codec
	}
}

// WithErrorCallback ...
func WithErrorCallback(f func(error)) Option {
	return func(cfg *config) {
		cfg.errorFunc = f
	}
}
