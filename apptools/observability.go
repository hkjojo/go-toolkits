package apptools

import (
	"os"
	"strings"
)

// traceOptions holds runtime-injectable settings for NewTracerProvider.
type traceOptions struct {
	endpoint string // OTLP trace exporter endpoint (host:port)
}

// TraceOption configures NewTracerProvider.
type TraceOption func(*traceOptions)

// WithEndpoint sets the OTLP trace exporter endpoint explicitly, allowing the
// caller to inject it at runtime. A non-empty value takes precedence over the
// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT / OTEL_EXPORTER_OTLP_ENDPOINT environment
// variables. The value must be a host:port (no scheme), matching
// otlptracegrpc.WithEndpoint.
func WithEndpoint(endpoint string) TraceOption {
	return func(o *traceOptions) {
		o.endpoint = endpoint
	}
}

func newTraceOptions(ops ...TraceOption) *traceOptions {
	o := &traceOptions{}
	for _, op := range ops {
		op(o)
	}
	return o
}

// pyroscopeOptions holds runtime-injectable settings for NewPyroscope.
type pyroscopeOptions struct {
	address string // pyroscope server address
}

// PyroscopeOption configures NewPyroscope.
type PyroscopeOption func(*pyroscopeOptions)

// WithAddress sets the pyroscope server address explicitly, allowing the caller
// to inject it at runtime. A non-empty value takes precedence over the
// PYROSCOPE_ADHOC_SERVER_ADDRESS environment variable.
func WithAddress(address string) PyroscopeOption {
	return func(o *pyroscopeOptions) {
		o.address = address
	}
}

func newPyroscopeOptions(ops ...PyroscopeOption) *pyroscopeOptions {
	o := &pyroscopeOptions{}
	for _, op := range ops {
		op(o)
	}
	return o
}

// ResolveEndpoint returns the explicit option value when non-empty (surrounding
// whitespace trimmed), otherwise the first non-empty environment variable among
// envKeys, checked in order. An explicit option always wins over env. Returns
// "" when nothing is configured, which callers use as the "true no-op" signal.
func ResolveEndpoint(optionValue string, envKeys ...string) string {
	if v := strings.TrimSpace(optionValue); v != "" {
		return v
	}
	for _, k := range envKeys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}
