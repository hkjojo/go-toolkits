// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wsrpc

import (
	"encoding/json"
	"errors"
	"io"
	"sync"
)

var errMissingParams = errors.New("jsonrpc: request body missing params")

type serverCodec struct {
	dec *json.Decoder // for reading JSON values
	enc *json.Encoder // for writing JSON values
	c   io.Closer

	// temporary work space
	req serverRequest

	// JSON-RPC clients can use arbitrary json values as request IDs.
	// Package rpc expects uint64 request IDs.
	// We assign uint64 sequence numbers to incoming requests
	// but save the original request ID in the pending map.
	// When rpc responds, we use the sequence number in
	// the response to find the original request ID.
	mutex   sync.Mutex // protects seq, pending
	seq     uint64
	pending map[uint64]*json.RawMessage
}

// NewServerCodec returns a new ServerCodec using JSON-RPC on conn.
func NewServerCodec(conn io.ReadWriteCloser) ServerCodec {
	return &serverCodec{
		dec:     json.NewDecoder(conn),
		enc:     json.NewEncoder(conn),
		c:       conn,
		pending: make(map[uint64]*json.RawMessage),
	}
}

type serverRequest struct {
	Version string           `json:"jsonrpc"`
	Method  string           `json:"method"`
	Params  *json.RawMessage `json:"params"`
	ID      *json.RawMessage `json:"id"`
}

type notification struct {
	Version      string      `json:"jsonrpc"`
	Method       string      `json:"method"`
	Notification string      `json:"notification"` //Field for rpc-websockets
	Params       interface{} `json:"params"`
}

func (r *serverRequest) reset() {
	r.Method = ""
	r.Params = nil
	r.ID = nil
}

type serverResponse struct {
	Version string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  interface{}      `json:"result"`
	Error   interface{}      `json:"error"`
}

func (c *serverCodec) ReadRequestHeader(r *Request) error {
	c.req.reset()
	if err := c.dec.Decode(&c.req); err != nil {
		return err
	}
	r.ServiceMethod = c.req.Method

	// JSON request id can be any JSON value;
	// RPC package expects uint64.  Translate to
	// internal uint64 and save JSON on the side.
	c.mutex.Lock()
	c.seq++
	c.pending[c.seq] = c.req.ID
	c.req.ID = nil
	r.Seq = c.seq
	c.mutex.Unlock()

	return nil
}

func (c *serverCodec) ReadRequestBody(x interface{}) error {
	if c.req.Params == nil {
		return errMissingParams
	}

	if x == nil {
		return nil
	}

	// JSON params structured object. Unmarshal to the args object.
	if err := json.Unmarshal(*c.req.Params, x); err != nil {
		// Clearly JSON params is not a structured object,
		// fallback and attempt an unmarshal with JSON params as
		// array value and RPC params is struct. Unmarshal into
		// array containing the request struct.
		params := [1]interface{}{x}
		if err = json.Unmarshal(*c.req.Params, &params); err != nil {
			return err
		}
	}

	return nil
}

// GetParams ...
func (c *serverCodec) GetParams() json.RawMessage {
	return *c.req.Params
}

// GetMethod
func (c *serverCodec) GetMethod() string {
	return c.req.Method
}

var null = json.RawMessage([]byte("null"))

func (c *serverCodec) WriteResponse(r *Response, x interface{}) error {
	c.mutex.Lock()
	b, ok := c.pending[r.Seq]
	if !ok {
		c.mutex.Unlock()
		return errors.New("invalid sequence number in response")
	}
	delete(c.pending, r.Seq)
	c.mutex.Unlock()

	if b == nil {
		// Invalid request so no id. Use JSON null.
		b = &null
	}
	resp := serverResponse{ID: b, Version: "2.0"}
	if r.Error == "" {
		resp.Result = x
	} else {
		resp.Error = r.Error
	}
	return c.enc.Encode(resp)
}

func (c *serverCodec) WriteNotification(method string, x interface{}) error {
	return c.enc.Encode(&notification{
		Version:      "2.0",
		Method:       method,
		Notification: method,
		Params:       []interface{}{x},
	})
}

func (c *serverCodec) WriteNotificationEx(method string, x interface{}) error {
	return c.enc.Encode(&notification{
		Version:      "2.0",
		Method:       method,
		Notification: method,
		Params:       x,
	})
}

func (c *serverCodec) Close() error {
	return c.c.Close()
}
