package wsrpc

import "fmt"

type Logger interface {
	Errorf(template string, args ...interface{})
}

type log struct{}

func (l *log) Errorf(template string, args ...interface{}) {
	fmt.Printf(template, args...)
}
