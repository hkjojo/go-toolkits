package metric

import (
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
)

func TestOTelCounter(t *testing.T) {
	// Setup OpenTelemetry
	mp := metric.NewMeterProvider()
	otel.SetMeterProvider(mp)
	SetMeterProvider(mp)

	// Test counter creation and usage
	counter := NewOTelCounter("test_counter", "Test counter", "1")
	if counter == nil {
		t.Fatal("Failed to create OTel counter")
	}

	// Test counter operations
	counter.Inc()
	counter.Add(5.0)
	counter.With("label1", "value1").Inc()
	counter.With("label1", "value1", "label2", "value2").Add(3.0)
}

func TestOTelGauge(t *testing.T) {
	// Setup OpenTelemetry
	mp := metric.NewMeterProvider()
	otel.SetMeterProvider(mp)
	SetMeterProvider(mp)

	// Test gauge creation and usage
	gauge := NewOTelGauge("test_gauge", "Test gauge", "1")
	if gauge == nil {
		t.Fatal("Failed to create OTel gauge")
	}

	// Test gauge operations
	gauge.Set(10.0)
	gauge.Add(5.0)
	gauge.Sub(2.0)
	gauge.With("service", "test").Set(100.0)

	// Test delete (should return false for OTel)
	deleted := gauge.Delete("service", "test")
	if deleted {
		t.Error("OTel gauge Delete should return false")
	}
}

func TestOTelHistogram(t *testing.T) {
	// Setup OpenTelemetry
	mp := metric.NewMeterProvider()
	otel.SetMeterProvider(mp)
	SetMeterProvider(mp)

	// Test histogram creation and usage
	histogram := NewOTelHistogram("test_histogram", "Test histogram", "ms")
	if histogram == nil {
		t.Fatal("Failed to create OTel histogram")
	}

	// Test histogram operations
	histogram.Observe(100.0)
	histogram.With("endpoint", "/api").Observe(250.5)
	histogram.With("endpoint", "/api", "method", "GET").Observe(150.0)
}

func TestMetricModeSwitch(t *testing.T) {
	// Test mode switching
	originalMode := GetMetricMode()
	defer SetMetricMode(originalMode) // Restore original mode

	// Test setting OTel mode
	SetMetricMode("otel")
	if GetMetricMode() != "otel" {
		t.Error("Failed to set OTel mode")
	}

	// Test setting Prometheus mode
	SetMetricMode("prometheus")
	if GetMetricMode() != "prometheus" {
		t.Error("Failed to set Prometheus mode")
	}
}

func TestOTelModeIntegration(t *testing.T) {
	// Setup OpenTelemetry
	mp := metric.NewMeterProvider()
	otel.SetMeterProvider(mp)
	SetMeterProvider(mp)

	// Set OTel mode
	originalMode := GetMetricMode()
	defer SetMetricMode(originalMode)
	SetMetricMode("otel")

	// Test that MustRegister doesn't panic in OTel mode
	// (it should be a no-op)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustRegister panicked in OTel mode: %v", r)
		}
	}()

	// This should not register anything in OTel mode
	// MustRegister(prometheus.NewCounter(prometheus.CounterOpts{Name: "test"}))

	// Test metric collection start with OTel mode
	SetMetricMode("otel")
	stop, err := Start(
		WithInterval(100*time.Millisecond),
		WithJSONLoggerWriter(&testLogger{}),
	)
	if err != nil {
		t.Fatalf("Failed to start metrics in OTel mode: %v", err)
	}
	defer stop()

	// Wait a bit to ensure no panics
	time.Sleep(200 * time.Millisecond)
}

// Test logger for testing
type testLogger struct{}

func (l *testLogger) Infow(msg string, keysAndValues ...interface{}) {
	// Do nothing for tests
}
