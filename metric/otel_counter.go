package metric

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ Counter = (*otelCounter)(nil)

type otelCounter struct {
	counter    metric.Float64Counter
	labelNames []string
	attrs      []attribute.KeyValue
}

// newOTelCounter creates a new OpenTelemetry counter and returns Counter.
func newOTelCounter(name, description string, labelNames []string) Counter {
	counter, err := getMeter().Float64Counter(
		name,
		metric.WithDescription(description),
	)
	if err != nil {
		panic(err)
	}
	return &otelCounter{
		counter:    counter,
		labelNames: labelNames,
	}
}

func (c *otelCounter) With(labelValues ...string) Counter {
	maxIndex := min(len(labelValues), len(c.labelNames))
	attrs := make([]attribute.KeyValue, 0, maxIndex)
	for i := 0; i < maxIndex; i++ {
		attrs = append(attrs, attribute.String(c.labelNames[i], labelValues[i]))
	}
	return &otelCounter{
		counter:    c.counter,
		labelNames: c.labelNames,
		attrs:      attrs,
	}
}

func (c *otelCounter) Inc() {
	c.Add(1)
}

func (c *otelCounter) Add(delta float64) {
	c.counter.Add(context.Background(), delta, metric.WithAttributes(c.attrs...))
}
