package metric

import (
	"context"
	"time"

	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// otelExporter implements Exporter interface for OpenTelemetry OTEL protocol
type otelExporter struct {
	interval       time.Duration
	endpoint       string
	serviceName    string
	serviceVersion string
	env            string
	meterProvider  *metric.MeterProvider
	logger         Logger
	init           bool
}

// newOTELExporter creates a new OTEL exporter
func newOTELExporter(logger Logger, endpoint string, interval time.Duration, serviceName, serviceVersion, env string) Exporter {
	exporter := &otelExporter{
		serviceName:    serviceName,
		serviceVersion: serviceVersion,
		env:            env,
		interval:       interval,
		endpoint:       endpoint,
		logger:         logger,
	}

	// Initialize OTEL exporter and meter provider
	// use OTEL ENV config: go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc@v1.37.0/internal/oconf/envconfig.go:54
	if err := exporter.initialize(); err != nil {
		logger.Errorw("otel exporter initialize failed", "err", err)
		return exporter
	}

	exporter.init = true
	logger.Infow("otel exporter initialized", "service", exporter.serviceName)
	return exporter
}

// initialize sets up the OTEL exporter and meter provider
func (w *otelExporter) initialize() error {
	ctx := context.Background()

	options := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithInsecure(),
	}
	if w.endpoint != "" {
		options = append(options, otlpmetricgrpc.WithEndpoint(w.endpoint))
	}

	exporter, err := otlpmetricgrpc.New(ctx,
		options...,
	)
	if err != nil {
		return err
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(w.serviceName),
			semconv.ServiceVersion(w.serviceVersion),
			semconv.DeploymentEnvironment(w.env),
		),
	)
	if err != nil {
		return err
	}

	// Create meter provider with period reader
	exporter.Temporality(0)
	exporter.Aggregation(metric.InstrumentKindHistogram)

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exporter, metric.WithInterval(w.interval))),
		metric.WithResource(res),
	)

	w.meterProvider = mp

	// Set global meter provider
	otel.SetMeterProvider(mp)
	return nil
}

// Export implements Exporter interface
// In OTEL mode, metrics are sent automatically by the MeterProvider
// This method is kept for compatibility but doesn't need to do anything
func (w *otelExporter) Export(_ *dto.MetricFamily) {
	// In OTEL mode, metrics are automatically sent by the MeterProvider's PeriodicReader
	// No manual conversion or sending is needed here
}

// OnError handles errors
func (w *otelExporter) OnError(err error) {
	w.logger.Errorw("otel exporter error", "error", err)
}

// Shutdown gracefully shuts down the OTEL exporter
func (w *otelExporter) Shutdown(ctx context.Context) error {
	if w.meterProvider != nil {
		return w.meterProvider.Shutdown(ctx)
	}
	return nil
}

// IsStart 是否启用
func (w *otelExporter) IsStart() bool {
	return w.init
}
