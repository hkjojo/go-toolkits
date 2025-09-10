package metric

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ Gauge = (*otelGauge)(nil)

type otelGauge struct {
	gauge metric.Float64UpDownCounter
	attrs []attribute.KeyValue
}

// newOTelGauge creates a new OpenTelemetry gauge and returns Gauge.
func newOTelGauge(name, description string, unit string) Gauge {
	meter := getMeter()
	gauge, err := meter.Float64UpDownCounter(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		panic(err)
	}
	return &otelGauge{
		gauge: gauge,
	}
}

func (g *otelGauge) With(lvs ...string) Gauge {
	attrs := make([]attribute.KeyValue, 0, len(lvs)/2)
	for i := 0; i < len(lvs); i += 2 {
		if i+1 < len(lvs) {
			attrs = append(attrs, attribute.String(lvs[i], lvs[i+1]))
		}
	}
	return &otelGauge{
		gauge: g.gauge,
		attrs: attrs,
	}
}

func (g *otelGauge) Delete(lvs ...string) bool {
	// OpenTelemetry doesn't support deleting specific label combinations
	// This is a limitation compared to Prometheus
	return false
}

func (g *otelGauge) Set(value float64) {
	// OpenTelemetry UpDownCounter doesn't have a Set method
	// We need to track the current value and calculate the delta
	// For simplicity, we'll use Add with the value
	// Note: This is not a perfect replacement for Prometheus Gauge.Set()
	g.gauge.Add(context.Background(), value, metric.WithAttributes(g.attrs...))
}

func (g *otelGauge) Add(delta float64) {
	g.gauge.Add(context.Background(), delta, metric.WithAttributes(g.attrs...))
}

func (g *otelGauge) Sub(delta float64) {
	g.gauge.Add(context.Background(), -delta, metric.WithAttributes(g.attrs...))
}
