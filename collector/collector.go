package collector

import (
	"errors"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/micro/go-micro/v2/broker"
)

var defaultCollector *collector

type collector struct {
	cfg *config

	queue chan proto.Message

	quitOnce sync.Once
	quit     chan interface{}
}

// Start ...
func Start(opts ...Option) error {
	c, err := newCollector(opts...)
	if err != nil {
		return err
	}

	go c.run()
	defaultCollector = c
	return nil
}

// Stop ...
func Stop() {
	if defaultCollector != nil {
		defaultCollector.stop()
	}
}

// Push ...
func Push(msg proto.Message) error {
	return defaultCollector.push(msg)
}

func newCollector(opts ...Option) (*collector, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return &collector{
		cfg:   cfg,
		quit:  make(chan interface{}, 1),
		queue: make(chan proto.Message, cfg.queueSize),
	}, nil
}

func (c *collector) run() {
	c.runTicker()
	for {
		select {
		case msg := <-c.queue:
			c.publish(msg)
		case <-c.quit:
			return
		}
	}
}

func (c *collector) runTicker() {
	for _, ep := range c.cfg.endpoints {
		go func(ep *endpoint) {
			var interval = ep.collectInterval
			if interval == 0 {
				interval = c.cfg.collectInterval
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					go c.collect(ep)
				case <-c.quit:
					return
				}
			}
		}(ep)
	}

}

func (c *collector) push(msg proto.Message) error {
	select {
	case c.queue <- msg:
	default:
		return errors.New("queue channel full")
	}
	return nil
}

func (c *collector) stop() {
	c.quitOnce.Do(func() {
		close(c.quit)
	})
}

func (c *collector) collect(ep *endpoint) {
	for _, msg := range ep.f() {
		if err := c.push(msg); err != nil && c.cfg.errorFunc != nil {
			c.cfg.errorFunc(err)
		}
	}
}

func (c *collector) publish(msgs ...proto.Message) {
	if c.cfg.broker != nil {
		for _, msg := range msgs {
			data, _ := c.cfg.codec.Marshal(msg)
			if err := c.cfg.broker.Publish(c.cfg.topic, &broker.Message{
				Header: map[string]string{}, // TODO:
				Body:   data,
			}); err != nil && c.cfg.errorFunc != nil {
				c.cfg.errorFunc(err)
			}
		}
	}
}
