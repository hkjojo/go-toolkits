package metric

import (
	"testing"

	metricnoop "go.opentelemetry.io/otel/metric/noop"
)

func TestNewMetricProvider_WithEndpointOption_ReturnsRealProvider(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	globalConfig.Endpoint = ""
	t.Cleanup(func() { globalConfig.Endpoint = "" })

	mp, cleanup, err := NewMetricProvider(WithEndpoint("localhost:4317"))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if _, ok := mp.(metricnoop.MeterProvider); ok {
		t.Fatal("expected a real MeterProvider when WithEndpoint Option is set, got noop")
	}
	if cleanup != nil {
		cleanup()
	}
}

// Regression guard for #1: metrics configured only via the generic
// OTEL_EXPORTER_OTLP_ENDPOINT must still export, not fall through to no-op.
func TestNewMetricProvider_GenericEnvEndpoint_ReturnsRealProvider(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	globalConfig.Endpoint = ""
	t.Cleanup(func() { globalConfig.Endpoint = "" })

	mp, cleanup, err := NewMetricProvider()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if _, ok := mp.(metricnoop.MeterProvider); ok {
		t.Fatal("expected a real MeterProvider when only the generic env endpoint is set, got noop")
	}
	if cleanup != nil {
		cleanup()
	}
}

func TestNewMetricProvider_NoEndpoint_ReturnsNoop(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	globalConfig.Endpoint = ""
	t.Cleanup(func() { globalConfig.Endpoint = "" })

	mp, cleanup, err := NewMetricProvider()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if _, ok := mp.(metricnoop.MeterProvider); !ok {
		t.Fatalf("provider = %T, want noop.MeterProvider (no exporter/reader created)", mp)
	}
	if cleanup == nil {
		t.Fatal("cleanup must be non-nil")
	}
	cleanup() // must be a safe no-op
}

func TestNewMetricProvider_EnvEndpoint_ReturnsRealProvider(t *testing.T) {
	globalConfig.Endpoint = ""
	t.Cleanup(func() { globalConfig.Endpoint = "" })
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "localhost:4317")

	mp, cleanup, err := NewMetricProvider()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if _, ok := mp.(metricnoop.MeterProvider); ok {
		t.Fatal("expected a real MeterProvider when env endpoint is set, got noop")
	}
	if cleanup != nil {
		cleanup()
	}
}
