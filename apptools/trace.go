package apptools

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

/*
	OTEL ENV

	OTEL_EXPORTER_OTLP_TRACES_ENDPOINT
	OTEL_EXPORTER_OTLP_TRACES_TIMEOUT
	OTEL_EXPORTER_OTLP_TRACES_INSECURE
	OTEL_EXPORTER_OTLP_TRACES_HEADERS
	OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE
	OTEL_EXPORTER_OTLP_TRACES_CLIENT_CERTIFICATE
	OTEL_EXPORTER_OTLP_TRACES_COMPRESSION
	OTEL_EXPORTER_OTLP_TRACES_CLIENT_KEY
*/

// NewTracerProvider ...
func NewTracerProvider(opts ...otlptracehttp.Option) (trace.TracerProvider, func(), error) {
	endpoint, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	if ok && endpoint == "" {
		return noop.NewTracerProvider(), func() {}, nil
	}

	ctx := context.Background()
	traceExp, err := otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
	if err != nil {
		return nil, nil, err
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(Name),
			semconv.ServiceVersionKey.String(Version),
			semconv.DeploymentEnvironmentKey.String(Env),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	bsp := tracesdk.NewBatchSpanProcessor(traceExp)
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		tracesdk.WithResource(res),
		tracesdk.WithSpanProcessor(bsp),
	)

	return tp, func() {
		cxt, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := tp.Shutdown(cxt); err != nil {
			otel.Handle(err)
		}
	}, nil
}
