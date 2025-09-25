package metric

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ Observer = (*otelSummary)(nil)

// otelSummary 实现OpenTelemetry摘要
type otelSummary struct {
	summary    metric.Float64Histogram
	labelNames []string
	attrs      []attribute.KeyValue
}

// newOTelSummary 创建OpenTelemetry摘要
func newOTelSummary(name, description string, unit string, labelNames []string) Observer {
	summary, err := getMeter().Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		panic(err)
	}
	return &otelSummary{
		summary:    summary,
		labelNames: labelNames,
	}
}

func (s *otelSummary) With(labelValues ...string) Observer {
	maxIndex := min(len(labelValues), len(s.labelNames))
	attrs := make([]attribute.KeyValue, 0, maxIndex)
	for i := 0; i < maxIndex; i++ {
		attrs = append(attrs, attribute.String(s.labelNames[i], labelValues[i]))
	}
	return &otelSummary{
		summary:    s.summary,
		labelNames: s.labelNames,
		attrs:      attrs,
	}
}

func (s *otelSummary) Observe(value float64) {
	s.summary.Record(context.Background(), value, metric.WithAttributes(s.attrs...))
}
