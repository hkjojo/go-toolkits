package metric

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ Observer = (*otelHistogram)(nil)

type otelHistogram struct {
	histogram metric.Float64Histogram
	attrs     []attribute.KeyValue
}

// newOTelHistogram creates a new OpenTelemetry histogram and returns Observer.
func newOTelHistogram(name, description string, unit string, buckets ...float64) Observer {
	meter := getMeter()
	opts := []metric.Float64HistogramOption{
		metric.WithDescription(description),
		metric.WithUnit(unit),
	}
	if len(buckets) > 0 {
		opts = append(opts, metric.WithExplicitBucketBoundaries(buckets...))
	}
	histogram, err := meter.Float64Histogram(name, opts...)
	if err != nil {
		panic(err)
	}
	return &otelHistogram{
		histogram: histogram,
	}
}

func (h *otelHistogram) With(lvs ...string) Observer {
	attrs := make([]attribute.KeyValue, 0, len(lvs)/2)
	for i := 0; i < len(lvs); i += 2 {
		if i+1 < len(lvs) {
			attrs = append(attrs, attribute.String(lvs[i], lvs[i+1]))
		}
	}
	return &otelHistogram{
		histogram: h.histogram,
		attrs:     attrs,
	}
}

func (h *otelHistogram) Observe(value float64) {
	h.histogram.Record(context.Background(), value, metric.WithAttributes(h.attrs...))
}
