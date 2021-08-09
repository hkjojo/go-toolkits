package observable

import (
	"reflect"
	"strings"
)

// Helpers
func (o *Observable) addCallback(event string, cb interface{}, isUnique bool) {
	events := strings.Fields(event)
	isTyped := len(events) > 1

	fn := reflect.ValueOf(cb)
	if fn.Kind() != reflect.Func {
		panic("callback not a function")
	}

	o.mux.Lock()
	defer o.mux.Unlock()
	for _, s := range events {
		o.callbacks[s] = append(o.callbacks[s], &callback{s, fn, isUnique, isTyped, false})
	}
}

// remove the events bound to the callback
func (o *Observable) removeEvent(s string, fns ...reflect.Value) {
	// lock the struct
	o.mux.Lock()
	defer o.mux.Unlock()

	for _, fn := range fns {
		o.remove(s, fn)
	}
}

func (o *Observable) remove(s string, fn reflect.Value) {
	// loop all the callbacks registered under the event namespace
	for i, cb := range o.callbacks[s] {
		if fn == cb.fn {
			o.callbacks[s] = append(o.callbacks[s][:i], o.callbacks[s][i+1:]...)
		}
	}
	// if there are no more callbacks using this namespace
	// delete the key from the map
	if len(o.callbacks[s]) == 0 {
		delete(o.callbacks, s)
	}
}

func (o *Observable) cleanEvent(s string) {
	o.mux.Lock()
	defer o.mux.Unlock()

	if s == ALL_EVENTS_NAMESPACE {
		// wipe all the event listeners
		o.callbacks = make(map[string][]*callback)
		return
	}
	delete(o.callbacks, s)
}

// get the callbacks from event
func (o *Observable) getCallbacks(s string) []*callback {
	o.mux.Lock()
	defer o.mux.Unlock()

	cbs := make([]*callback, 0, len(o.callbacks[s]))
	for _, cb := range o.callbacks[s] {
		// check once event
		if cb.isUnique {
			if cb.wasCalled {
				continue
			}
			cb.wasCalled = true
		}
		cbs = append(cbs, cb)
	}
	return cbs
}

// dispatch the events using custom arguments
func (o *Observable) dispatchEvents(s string, args []reflect.Value) *Observable {
	cbs := o.getCallbacks(s)
	for _, cb := range cbs {
		// if the callback was registered with multiple events
		// we prepend the event namespace to the function arguments
		if cb.isTyped {
			args = append([]reflect.Value{reflect.ValueOf(cb.event)}, args...)
		}

		cb.fn.Call(args)
		if cb.isUnique && cb.wasCalled {
			o.removeEvent(cb.event, cb.fn)
		}
	}
	return o
}
