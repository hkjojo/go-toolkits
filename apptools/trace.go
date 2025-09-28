package apptools

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

/*
NewTracerProvider Support Env

OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://openobserve.td-ops.com/api/dealer-dev/traces
OTEL_EXPORTER_OTLP_TRACES_TIMEOUT
OTEL_EXPORTER_OTLP_TRACES_INSECURE
OTEL_EXPORTER_OTLP_TRACES_HEADERS=Authorization=Basic YWRtaW5AZXhhbXBsZS5jb206MnZyOHIxZ3h4OXhqTXVvSg==,stream-name=default
OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE
OTEL_EXPORTER_OTLP_TRACES_CLIENT_CERTIFICATE
OTEL_EXPORTER_OTLP_TRACES_COMPRESSION
OTEL_EXPORTER_OTLP_TRACES_CLIENT_KEY
*/
func NewTracerProvider() (trace.TracerProvider, func(), error) {
	ctx := context.Background()
	traceExp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
	)
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

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(traceExp),
		tracesdk.WithResource(res),
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
	)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetTracerProvider(tp)

	shutdown := func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}

	return tp, shutdown, nil
}
