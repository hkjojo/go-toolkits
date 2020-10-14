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
	sync.Mutex
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
	n.Lock()
	defer n.Unlock()

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
	n.Lock()
	defer n.Unlock()

	if n.isopen {
		select {
		case n.sending <- &notify{
			method: method,
			data:   data,
		}:
		default:
			n.closeLocked()
			go n.conn.Close()
			return errors.New("Sending channel is full, conn close")
		}
	}

	return nil
}

// NotifyArr ...
func (n *Notifier) NotifyArr(method string, data interface{}) error {
	n.Lock()
	defer n.Unlock()

	if n.isopen {
		select {
		case n.sending <- &notify{
			method: method,
			data:   data,
			isArr:  true,
		}:
		default:
			n.closeLocked()
			go n.conn.Close()
			return errors.New("Sending channel is full, conn close")
		}
	}

	return nil
}
