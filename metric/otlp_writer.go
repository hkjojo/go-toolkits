package metric

import (
	"context"
	"os"
	"time"

	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// OTLPOption defines options for OTLP writer
type OTLPOption func(*otlpWriter)

// otlpWriter implements Writer interface for OpenTelemetry OTLP protocol
type otlpWriter struct {
	internal      time.Duration
	endpoint      string
	headers       map[string]string
	serviceName   string
	streamName    string
	meterProvider *metric.MeterProvider
	exporter      metric.Exporter
	logger        ErrorLogger
	init          bool
}

// WithOTLPEndpoint sets the OTLP endpoint
func WithOTLPEndpoint(endpoint string) OTLPOption {
	return func(w *otlpWriter) {
		w.endpoint = endpoint
	}
}

// WithOTLPEndpoint sets the OTLP internal
func WithOTLPInternal(internal time.Duration) OTLPOption {
	return func(w *otlpWriter) {
		w.internal = internal
	}
}

// WithOTLPHeaders sets custom headers for OTLP requests
func WithOTLPHeaders(headers map[string]string) OTLPOption {
	return func(w *otlpWriter) {
		w.headers = headers
	}
}

// newOTLPWriter creates a new OTLP writer
func newOTLPWriter(logger ErrorLogger, opts ...OTLPOption) Writer {
	w := &otlpWriter{
		serviceName: os.Getenv("SERVICE_NAME"),
		streamName:  os.Getenv("METRIC_OPENOBSERVE_STREAM_NAME"),
		logger:      logger,
	}

	// Apply options
	for _, opt := range opts {
		opt(w)
	}

	// Initialize OTLP exporter and meter provider
	if err := w.initialize(); err != nil {
		logger.Errorw("Failed to initialize OTLP writer", "error", err)
		return w
	}

	w.init = true
	logger.Infow("OTLP writer initialized", "header", w.headers, "endpoint", w.endpoint, "service", w.serviceName)
	return w
}

// initialize sets up the OTLP exporter and meter provider
func (w *otlpWriter) initialize() error {
	ctx := context.Background()

	// Create OTLP HTTP exporter
	exporterOpts := []otlpmetrichttp.Option{}

	// Add custom endpoint if provided
	if w.endpoint != "" {
		exporterOpts = append(exporterOpts, otlpmetrichttp.WithEndpointURL(w.endpoint))
	}
	// Add custom headers if provided
	if len(w.headers) > 0 {
		exporterOpts = append(exporterOpts, otlpmetrichttp.WithHeaders(w.headers))
	}

	exporter, err := otlpmetrichttp.New(ctx, exporterOpts...)
	if err != nil {
		return err
	}
	w.exporter = exporter

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(w.serviceName),
		),
	)
	if err != nil {
		return err
	}

	// Create meter provider with periodic reader
	exporter.Temporality(0)
	exporter.Aggregation(metric.InstrumentKindHistogram)
	periodReader := metric.NewPeriodicReader(exporter, metric.WithInterval(w.internal))
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(periodReader),
	)

	w.meterProvider = meterProvider

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)
	SetMeterProvider(meterProvider)

	return nil
}

// Write implements Writer interface
// In OTLP mode, metrics are sent automatically by the MeterProvider
// This method is kept for compatibility but doesn't need to do anything
func (w *otlpWriter) Write(mf *dto.MetricFamily) {
	if !w.init {
		return
	}
	// In OTLP mode, metrics are automatically sent by the MeterProvider's PeriodicReader
	// No manual conversion or sending is needed here
}

// OnError handles errors
func (w *otlpWriter) OnError(err error) {
	w.logger.Errorw("OTLP writer error", "error", err)
}

// Shutdown gracefully shuts down the OTLP writer
func (w *otlpWriter) Shutdown(ctx context.Context) error {
	if w.meterProvider != nil {
		return w.meterProvider.Shutdown(ctx)
	}
	return nil
}
