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

	endpoints []func() proto.Message
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

// WithKafkaAddr ...
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
		cfg.endpoints = append(cfg.endpoints, f)
	}
}

// WithMsgCodec ...
func WithMsgCodec(codec codec.Marshaler) Option {
	return func(cfg *config) {
		cfg.codec = codec
	}
}
