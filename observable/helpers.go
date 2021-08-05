package observable

import (
	"reflect"
)

// Helpers

// Add a callback under a certain event namespace
func (o *Observable) addCallback(s string, fn reflect.Value, isUnique bool, isTyped bool) {
	// lock the struct
	o.Lock()
	defer o.Unlock()

	o.Callbacks[s] = append(o.Callbacks[s], callback{fn, isUnique, isTyped, false})
}

// remove the events bound to the callback
func (o *Observable) removeEvent(s string, fn reflect.Value) {
	// lock the struct
	o.Lock()
	defer o.Unlock()

	o.remove(s, fn)
}

func (o *Observable) remove(s string, fn reflect.Value) {
	// loop all the callbacks registered under the event namespace
	for i, cb := range o.Callbacks[s] {
		if fn == cb.fn {
			o.Callbacks[s] = append(o.Callbacks[s][:i], o.Callbacks[s][i+1:]...)
		}
	}
	// if there are no more callbacks using this namespace
	// delete the key from the map
	if len(o.Callbacks[s]) == 0 {
		delete(o.Callbacks, s)
	}
}

func (o *Observable) cleanEvent(event string) {
	// lock the struct
	o.Lock()
	defer o.Unlock()

	if event == ALL_EVENTS_NAMESPACE {
		// wipe all the event listeners
		o.Callbacks = make(map[string][]callback)
		return
	}
	delete(o.Callbacks, event)
}

// dispatch the events using custom arguments
func (o *Observable) dispatchEvent(s string, arguments []reflect.Value) *Observable {
	// lock the struct
	o.RLock()
	callbacks := o.Callbacks[s]
	o.RUnlock()

	// loop all the callbacks
	// avoiding to call twice the ones registered with Observable.One
	for i, cb := range callbacks {
		if !cb.isUnique || (cb.isUnique && !cb.wasCalled) {
			// if the callback was registered with multiple events
			// we prepend the event namespace to the function arguments
			if cb.isTyped {
				cb.fn.Call(append([]reflect.Value{reflect.ValueOf(s)}, arguments...))
			} else {
				cb.fn.Call(arguments)
			}
		}

		callbacks[i].wasCalled = true
		if cb.isUnique {
			o.removeEvent(s, cb.fn)
		}
	}

	return o
}
