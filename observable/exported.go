package observable

// On default On
func On(event string, cb interface{}) *Observable {
	return obsrv.On(event, cb)
}

// Trigger default Trigger
func Trigger(event string, params ...interface{}) *Observable {
	return obsrv.Trigger(event, params...)
}

// Off default Off
func Off(event string, args ...interface{}) *Observable {
	return obsrv.Off(event, args...)
}

// Once default Once
func Once(event string, cb interface{}) *Observable {
	return obsrv.Once(event, cb)
}
