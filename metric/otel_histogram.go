package metric

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ Observer = (*otelHistogram)(nil)

type otelHistogram struct {
	histogram  metric.Float64Histogram
	labelNames []string
	attrs      []attribute.KeyValue
}

// newOTelHistogram creates a new OpenTelemetry histogram and returns Observer.
func newOTelHistogram(name, description string, labelNames []string, buckets ...float64) Observer {
	opts := []metric.Float64HistogramOption{
		metric.WithDescription(description),
	}
	if len(buckets) > 0 {
		opts = append(opts, metric.WithExplicitBucketBoundaries(buckets...))
	}
	histogram, err := otel.Meter(globalConfig.ServiceName).Float64Histogram(name, opts...)
	if err != nil {
		panic(err)
	}
	return &otelHistogram{
		histogram:  histogram,
		labelNames: labelNames,
	}
}

func (h *otelHistogram) With(labelValues ...string) Observer {
	maxIndex := min(len(labelValues), len(h.labelNames))
	attrs := make([]attribute.KeyValue, 0, maxIndex)
	for i := 0; i < maxIndex; i++ {
		attrs = append(attrs, attribute.String(h.labelNames[i], labelValues[i]))
	}
	return &otelHistogram{
		histogram:  h.histogram,
		labelNames: h.labelNames,
		attrs:      attrs,
	}
}

func (h *otelHistogram) Observe(value float64) {
	h.histogram.Record(context.Background(), value, metric.WithAttributes(h.attrs...))
}
