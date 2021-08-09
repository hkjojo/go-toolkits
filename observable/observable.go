package observable

import (
	"reflect"
	"strings"
	"sync"
)

// ALL_EVENTS_NAMESPACE event key uset to listen and remove all the events
const ALL_EVENTS_NAMESPACE = "*"

var (
	obsrv = New()
)

// private struct
type callback struct {
	event     string
	fn        reflect.Value
	isUnique  bool
	isTyped   bool
	wasCalled bool
}

// Observable struct
type Observable struct {
	callbacks map[string][]*callback
	mux       *sync.Mutex
}

// Public API

// New - returns a new observable reference
func New() *Observable {
	return &Observable{
		make(map[string][]*callback),
		&sync.Mutex{},
	}
}

// On - adds a callback function
func (o *Observable) On(event string, cb interface{}) *Observable {
	o.addCallback(event, cb, false)
	return o
}

// Trigger - a particular event passing custom arguments
func (o *Observable) Trigger(event string, params ...interface{}) *Observable {
	// get the args we want to pass to our listeners callbaks
	args := make([]reflect.Value, 0, len(params))

	// get all the arguments
	for _, param := range params {
		args = append(args, reflect.ValueOf(param))
	}

	// get all the list of events space separated
	events := strings.Fields(event)
	for _, s := range events {
		o.dispatchEvents(s, args)
	}

	// trigger the all events callback whenever this event was defined
	if event != ALL_EVENTS_NAMESPACE {
		o.dispatchEvents(ALL_EVENTS_NAMESPACE, append([]reflect.Value{reflect.ValueOf(event)}, args...))
	}
	return o
}

// Off - stop listening a particular event
func (o *Observable) Off(event string, args ...interface{}) *Observable {
	if event == ALL_EVENTS_NAMESPACE {
		// wipe all the event listeners
		o.cleanEvent(event)
		return o
	}
	events := strings.Fields(event)
	for _, s := range events {
		if len(args) == 0 {
			o.cleanEvent(s)
		} else if len(args) == 1 {
			fn := reflect.ValueOf(args[0])
			o.removeEvent(s, fn)
		} else {
			panic("Multiple off callbacks are not supported")
		}
	}
	return o
}

// Once - call the callback only once
func (o *Observable) Once(event string, cb interface{}) *Observable {
	o.addCallback(event, cb, true)
	return o
}
