package wsrpc

import (
	"io"
	"time"

	"github.com/gorilla/websocket"
)

// ReadWriteCloser ...
type ReadWriteCloser struct {
	WS   *websocket.Conn
	r    io.Reader
	done chan struct{}
}

// NewReadWriteCloser ...
func NewReadWriteCloser(ws *websocket.Conn) *ReadWriteCloser {
	rwc := &ReadWriteCloser{
		WS:   ws,
		done: make(chan struct{}),
	}
	go rwc.ping()
	return rwc
}

func (rwc *ReadWriteCloser) ping() {
	t := time.NewTicker(time.Second * 5)
	defer t.Stop()

	for range t.C {
		select {
		case <-rwc.done:
			return
		default:
			if err := rwc.WS.WriteControl(websocket.PingMessage,
				[]byte{}, time.Now().Add(time.Second*5),
			); err != nil {
				rwc.WS.Close()
				return
			}
		}
	}
}

func (rwc *ReadWriteCloser) Read(p []byte) (n int, err error) {
	if rwc.r == nil {
		_, rwc.r, err = rwc.WS.NextReader()
		if err != nil {
			return 0, err
		}
	}
	for n = 0; n < len(p); {
		var m int
		m, err = rwc.r.Read(p[n:])
		n += m
		if err == io.EOF {
			rwc.r = nil
			break
		}
		if err != nil {
			break
		}
	}

	return
}

func (rwc *ReadWriteCloser) Write(p []byte) (n int, err error) {
	var w io.WriteCloser
	w, err = rwc.WS.NextWriter(websocket.TextMessage)
	if err != nil {
		return 0, err
	}

	for n = 0; n < len(p); {
		var m int
		m, err = w.Write(p)
		n += m
		if err != nil {
			break
		}
	}

	if err != nil {
		err = rwc.Close()
		return
	}

	w.Close()
	return
}

// Close ...
func (rwc *ReadWriteCloser) Close() (err error) {
	err = rwc.WS.Close()
	select {
	case rwc.done <- struct{}{}:
	default:
	}
	return
}
