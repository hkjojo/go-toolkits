package redis

import (
	"fmt"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
)

type CallBack func(string)

// PubSubClient represents the Redis Pub/Sub client structure.
type PubSubClient struct {
	pool *Pool
	psc  redis.PubSubConn
	ch   chan redis.Message

	subMu         sync.RWMutex
	subscriptions map[string]CallBack
	reSubCallBack func()
	startPing     sync.Once
	doneCh        chan struct{}
}

// NewPubSubClient creates a new Pub/Sub client.
func NewPubSubClient(pool *Pool, reSubCallBack func()) (*PubSubClient, error) {
	client := &PubSubClient{
		pool:          pool,
		psc:           redis.PubSubConn{Conn: pool.Conn()},
		ch:            make(chan redis.Message, 4096),
		subscriptions: make(map[string]CallBack),
		doneCh:        make(chan struct{}),
		reSubCallBack: reSubCallBack,
	}

	go client.pushMessages()
	go client.receiveMessages()
	return client, nil
}

// receiveMessages listens for incoming messages and broadcasts them to the channel.
func (c *PubSubClient) receiveMessages() {
	for {
		switch msg := c.psc.ReceiveWithTimeout(10 * time.Second).(type) {
		case redis.Message:
			select {
			case c.ch <- msg:
			default:
				_, ok := <-c.ch
				if !ok {
					return
				}
				// TODO solution channel is full
			}
		case redis.Subscription:
			fmt.Printf("Subscription message: %s: %s %d\n", msg.Channel, msg.Kind, msg.Count)
		case error:
			fmt.Printf("Error: %s", msg)
			time.Sleep(5 * time.Second)
			c.reSubscribe()
		}
	}
}

// receiveMessages listens for incoming messages and broadcasts them to the channel.
func (c *PubSubClient) pushMessages() {
	defer close(c.ch)
	for {
		select {
		case msg, ok := <-c.ch:
			if !ok {
				return
			}

			c.subMu.RLock()
			cb, ok := c.subscriptions[msg.Channel]
			c.subMu.RUnlock()
			if ok {
				cb(string(msg.Data))
			}
		case <-c.doneCh:
			return
		}
	}
}

// receiveMessages listens for incoming messages and broadcasts them to the channel.
func (c *PubSubClient) loopPing() {
	c.startPing.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			for range ticker.C {
				c.psc.Ping("PING")
			}
		}()
	})
}

func (c *PubSubClient) reSubscribe() {
	c.psc.Close()
	c.psc = redis.PubSubConn{Conn: c.pool.Conn()}
	// re-subscribe to channels after an error occurs

	c.subMu.RLock()
	for channel := range c.subscriptions {
		err := c.psc.Subscribe(channel)
		if err != nil {
			fmt.Printf("reSubscribe Error: %s", err)
		}
	}
	c.subMu.RUnlock()
}

func (c *PubSubClient) subscribe(channel string, cb CallBack) error {
	err := c.psc.Subscribe(channel)
	if err != nil {
		return err
	}

	c.subMu.Lock()
	c.subscriptions[channel] = cb
	c.subMu.Unlock()
	c.loopPing()
	return nil
}

func (c *PubSubClient) unsubscribe(channel string) {
	c.psc.Unsubscribe(channel)
	c.subMu.Lock()
	defer c.subMu.Unlock()
	delete(c.subscriptions, channel)
}

// Close closes the client connection.
func (c *PubSubClient) Close() error {
	close(c.doneCh)
	err := c.psc.Close()
	if err != nil {
		return err
	}
	return nil
}
