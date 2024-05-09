package apptools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	ggrpc "google.golang.org/grpc"
)

type ClientMode string

const (
	TraceClientGRPC ClientMode = "grpc"
	TraceClientHTTP ClientMode = "http"
)

var defaultConfig *tradeConfig

type Option func(*tradeConfig)

type tradeConfig struct {
	endpoint      string
	authorization string
	organization  string
	stream        string
	insecure      bool
	clientMode    ClientMode
}

// WithEndpoint default use os env: OTLP_ENDPOINT
func WithEndpoint(endpoint string) Option {
	return func(c *tradeConfig) {
		c.endpoint = endpoint
	}
}

// WithAuthorization default use os env: OTLP_AUTHORIZATION
func WithAuthorization(authorization string) Option {
	return func(c *tradeConfig) {
		c.authorization = authorization
	}
}

// WithOrganization default use os env: OTLP_ORGANIZATION
func WithOrganization(organization string) Option {
	return func(c *tradeConfig) {
		c.organization = organization
	}
}

// WithInsecure default use os env: OTLP_INSECURE
func WithInsecure(insecure bool) Option {
	return func(c *tradeConfig) {
		c.insecure = insecure
	}
}

// WithStream default use os env: OTLP_STREAM_NAME
func WithStream(streamName string) Option {
	return func(c *tradeConfig) {
		c.stream = streamName
	}
}

// WithClientMode default grpc
func WithClientMode(mode ClientMode) Option {
	return func(c *tradeConfig) {
		c.clientMode = mode
	}
}

func initDefaultConfig() {
	defaultConfig = &tradeConfig{
		endpoint:      OtlpEndpoint,
		authorization: OtlpAuthorization,
		organization:  OtlpOrganization,
		stream:        OtlpStreamName,
		insecure:      OtlpInsecure,
		clientMode:    ClientMode(strings.ToLower(OtlpClient)),
	}
}

// NewTracerProvider ...
func NewTracerProvider(opts ...Option) (trace.TracerProvider, func(), error) {
	initDefaultConfig()

	for _, option := range opts {
		option(defaultConfig)
	}

	if defaultConfig.endpoint == "" {
		return noop.NewTracerProvider(), func() {}, nil
	}

	var (
		traceClient otlptrace.Client
		header      = map[string]string{
			"Authorization": defaultConfig.authorization,
			"organization":  defaultConfig.organization,
			"stream-name":   defaultConfig.stream,
		}
	)

	switch defaultConfig.clientMode {
	case TraceClientGRPC:
		var options []otlptracegrpc.Option
		options = append(options, otlptracegrpc.WithEndpoint(defaultConfig.endpoint))
		options = append(options, otlptracegrpc.WithDialOption(ggrpc.WithTimeout(10*time.Second)))
		options = append(options, otlptracegrpc.WithHeaders(header))
		if defaultConfig.insecure {
			options = append(options, otlptracegrpc.WithInsecure())
		}
		traceClient = otlptracegrpc.NewClient(options...)
	case TraceClientHTTP:
		var options []otlptracehttp.Option
		options = append(options, otlptracehttp.WithEndpoint(defaultConfig.endpoint))
		options = append(options, otlptracehttp.WithURLPath(fmt.Sprintf("/api/%s/traces", defaultConfig.organization)))
		options = append(options, otlptracehttp.WithHeaders(header))
		if defaultConfig.insecure {
			options = append(options, otlptracehttp.WithInsecure())
		}
		traceClient = otlptracehttp.NewClient(options...)
	default:
		return noop.NewTracerProvider(), func() {}, nil
	}

	ctx := context.Background()
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
