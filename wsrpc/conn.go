package wsrpc

import (
	"sync"

	"net/http"
)

type notifyEvent struct {
	method string
	params interface{}
}

// ConnCloseHandler ...
type ConnCloseHandler func()

// Conn ...
type Conn struct {
	Request       *http.Request
	rwc           *ReadWriteCloser
	codec         ServerCodec
	sending       *sync.Mutex
	closed        bool
	mu            sync.RWMutex
	closeHandlers []ConnCloseHandler
	extraData     map[string]interface{}
}

// NewConn ...
func NewConn(req *http.Request, sending *sync.Mutex, codec ServerCodec) *Conn {
	conn := &Conn{
		Request:   req,
		sending:   sending,
		codec:     codec,
		extraData: make(map[string]interface{}),
	}

	return conn
}

func (c *Conn) ternimating() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	for _, handler := range c.closeHandlers {
		handler()
	}

	c.closeHandlers = []ConnCloseHandler{}
}

// Notify ...
func (c *Conn) Notify(method string, params interface{}) error {
	c.sending.Lock()
	defer c.sending.Unlock()

	return c.codec.WriteNotification(method, params)
}

// NotifyEx ...
func (c *Conn) NotifyEx(method string, params interface{}) error {
	c.sending.Lock()
	defer c.sending.Unlock()

	return c.codec.WriteNotificationEx(method, params)
}

// Close ...
func (c *Conn) Close() error {
	return c.codec.Close()
}

// OnClose ...
func (c *Conn) OnClose(f ConnCloseHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		go f()
	}
	c.closeHandlers = append(c.closeHandlers, f)
}

// GetData ...
func (c *Conn) GetData(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.extraData[key]
}

// SetData ...
func (c *Conn) SetData(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.extraData[key] = value
}

// DelData ...
func (c *Conn) DelData(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.extraData, key)
}
