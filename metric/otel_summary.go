package metric

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ Observer = (*otelSummary)(nil)

// otelSummary 实现OpenTelemetry摘要
type otelSummary struct {
	summary metric.Float64Histogram
	attrs   []attribute.KeyValue
}

// newOTelSummary 创建OpenTelemetry摘要
func newOTelSummary(name, description string, unit string) Observer {
	meter := otel.Meter("go-toolkits/metric")
	summary, err := meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		panic(err)
	}
	return &otelSummary{
		summary: summary,
	}
}

func (s *otelSummary) With(lvs ...string) Observer {
	attrs := make([]attribute.KeyValue, 0, len(lvs)/2)
	for i := 0; i < len(lvs); i += 2 {
		if i+1 < len(lvs) {
			attrs = append(attrs, attribute.String(lvs[i], lvs[i+1]))
		}
	}
	return &otelSummary{
		summary: s.summary,
		attrs:   attrs,
	}
}

func (s *otelSummary) Observe(value float64) {
	s.summary.Record(context.Background(), value, metric.WithAttributes(s.attrs...))
}
