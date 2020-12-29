package wsrpc

import (
	"errors"
	"sync"
)

type notify struct {
	method string
	data   interface{}
	isArr  bool
}

// Notifier ...
type Notifier struct {
	mux     sync.Mutex
	conn    *Conn
	sending chan *notify
	isopen  bool
}

// NewNotifier ...
func NewNotifier(conn *Conn) *Notifier {
	sending := make(chan *notify, 1e3)

	notifier := &Notifier{
		conn:    conn,
		sending: sending,
		isopen:  true,
	}
	conn.OnClose(func() {
		notifier.Close()
	})

	go notifier.loop()

	return notifier
}

func (n *Notifier) loop() {
	for n.isopen {
		notify, ok := <-n.sending
		if !ok {
			break
		}

		if notify.isArr {
			n.conn.NotifyEx(notify.method, notify.data)
		} else {
			n.conn.Notify(notify.method, notify.data)
		}
	}
}

// Close ...
func (n *Notifier) Close() {
	n.mux.Lock()
	defer n.mux.Unlock()

	n.closeLocked()

}

func (n *Notifier) closeLocked() {
	if n.isopen {
		close(n.sending)
		n.isopen = false
	}
}

// Notify ...
func (n *Notifier) Notify(method string, data interface{}) error {
	if !n.isopen {
		return errors.New("notifier closed")
	}

	select {
	case n.sending <- &notify{
		method: method,
		data:   data,
	}:
	default:
		return errors.New("sending channel is full, conn close")
	}

	return nil
}

// NotifyArr ...
func (n *Notifier) NotifyArr(method string, data interface{}) error {
	if !n.isopen {
		return errors.New("notifier closed")
	}

	select {
	case n.sending <- &notify{
		method: method,
		data:   data,
		isArr:  true,
	}:
	default:
		return errors.New("sending channel is full, conn close")
	}

	return nil
}

// Clean ...
func (n *Notifier) Clean() {
	l := len(n.sending)
	for n.isopen {
		if l <= 1 {
			return
		}

		select {
		case _, ok := <-n.sending:
			if !ok {
				return
			}
			l--
		default:
			return
		}
	}
}
