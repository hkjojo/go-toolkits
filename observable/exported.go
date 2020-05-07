package observable

import (
	"reflect"
)

// On default On
func On(event string, cb interface{}) *Observable {
	fn := reflect.ValueOf(cb)
	return on(observer, event, fn)
}

// Trigger default Trigger
func Trigger(event string, params ...interface{}) *Observable {
	// get the arguments we want to pass to our listeners callbaks
	arguments := make([]reflect.Value, len(params))

	// get all the arguments
	for i, param := range params {
		arguments[i] = reflect.ValueOf(param)
	}
	return trigger(observer, event, arguments)
}

// Off default Off
func Off(event string, args ...interface{}) *Observable {
	return off(observer, event, args...)
}

// One default One
func One(event string, cb interface{}) *Observable {
	fn := reflect.ValueOf(cb)
	return one(observer, event, fn)
}
