package k8sprobe

// Option defines a function to configure the Manager.
type Option func(*Manager)

// WithAddress specifies the address to listen on (e.g., ":8080").
// If set, a new http.Server will be created and started automatically.
func WithAddress(addr string) Option {
	return func(m *Manager) {
		m.addr = addr
	}
}
