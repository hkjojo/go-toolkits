package apptools

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	ggrpc "google.golang.org/grpc"
)

// NewTracerProvider ...
func NewTracerProvider(endpoint, authorization, organization string) (trace.TracerProvider, func(), error) {
	if endpoint == "" {
		return noop.NewTracerProvider(), func() {}, nil
	}

	options := make([]otlptracegrpc.Option, 0, 4)
	options = append(options, otlptracegrpc.WithEndpoint(endpoint))
	options = append(options, otlptracegrpc.WithDialOption(ggrpc.WithTimeout(10*time.Second)))

	if authorization == "" {
		options = append(options, otlptracegrpc.WithInsecure())
	} else {
		options = append(options, otlptracegrpc.WithHeaders(
			map[string]string{
				"Authorization": authorization,
				"organization":  organization,
				"stream-name":   "default",
			},
		))
	}

	ctx := context.Background()
	traceClient := otlptracegrpc.NewClient(options...)
	traceExp, err := otlptrace.New(ctx, traceClient)
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
