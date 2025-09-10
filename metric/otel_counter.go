package metric

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ Counter = (*otelCounter)(nil)

type otelCounter struct {
	counter metric.Float64Counter
	attrs   []attribute.KeyValue
}

// newOTelCounter creates a new OpenTelemetry counter and returns Counter.
func newOTelCounter(name, description string, unit string) Counter {
	meter := getMeter()
	counter, err := meter.Float64Counter(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		panic(err)
	}
	return &otelCounter{
		counter: counter,
	}
}

func (c *otelCounter) With(lvs ...string) Counter {
	attrs := make([]attribute.KeyValue, 0, len(lvs)/2)
	for i := 0; i < len(lvs); i += 2 {
		if i+1 < len(lvs) {
			attrs = append(attrs, attribute.String(lvs[i], lvs[i+1]))
		}
	}
	return &otelCounter{
		counter: c.counter,
		attrs:   attrs,
	}
}

func (c *otelCounter) Inc() {
	c.Add(1)
}

func (c *otelCounter) Add(delta float64) {
	c.counter.Add(context.Background(), delta, metric.WithAttributes(c.attrs...))
}
