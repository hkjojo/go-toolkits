package jaeger

import (
	opentracing "github.com/opentracing/opentracing-go"
	jg "github.com/uber/jaeger-client-go"
	jgcfg "github.com/uber/jaeger-client-go/config"
	"io"
	"time"
)

// NewTracer ...
func NewTracer(servicename string, addr string) (opentracing.Tracer, io.Closer, error) {
	cfg := jgcfg.Configuration{
		ServiceName: servicename, // tracer name
		Sampler: &jgcfg.SamplerConfig{
			Type:  jg.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jgcfg.ReporterConfig{
			LogSpans:            true,
			BufferFlushInterval: 1 * time.Second,
		},
	}
	sender, err := jg.NewUDPTransport(addr, 0)
	if err != nil {
		return nil, nil, err
	}
	reporter := jg.NewRemoteReporter(sender)

	// Initialize Opentracing tracer with Jaeger Reporter
	tracer, closer, err := cfg.NewTracer(
		jgcfg.Reporter(reporter),
	)
	return tracer, closer, err
}
