package apptools

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// baseMetric 基础指标结构体，包含通用字段和方法
type baseMetric struct {
	labelNames []string
	attrs      []attribute.KeyValue
}

// With 设置标签值
func (b *baseMetric) With(labelValues []string) {
	maxIndex := min(len(labelValues), len(b.labelNames))
	attrs := make([]attribute.KeyValue, 0, maxIndex)
	for i := 0; i < maxIndex; i++ {
		attrs = append(attrs, attribute.String(b.labelNames[i], labelValues[i]))
	}
	b.attrs = attrs
}

// NewInt64Counter 创建带预置标签的 Int64Counter
func NewInt64Counter(name string, labelNames []string, options ...metric.Int64CounterOption) (*Int64Counter, error) {
	counter, err := otel.Meter(Name).Int64Counter(name, options...)
	if err != nil {
		return nil, err
	}

	return &Int64Counter{
		Int64Counter: counter,
		baseMetric: baseMetric{
			labelNames: labelNames,
		},
	}, nil
}

// NewInt64UpDownCounter 创建带预置标签的 Int64UpDownCounter
func NewInt64UpDownCounter(name string, labelNames []string, options ...metric.Int64UpDownCounterOption) (*Int64UpDownCounter, error) {
	counter, err := otel.Meter(Name).Int64UpDownCounter(name, options...)
	if err != nil {
		return nil, err
	}

	return &Int64UpDownCounter{
		Int64UpDownCounter: counter,
		baseMetric: baseMetric{
			labelNames: labelNames,
		},
	}, nil
}

// NewInt64Histogram 创建带预置标签的 Int64Histogram
func NewInt64Histogram(name string, labelNames []string, options ...metric.Int64HistogramOption) (*Int64Histogram, error) {
	histogram, err := otel.Meter(Name).Int64Histogram(name, options...)
	if err != nil {
		return nil, err
	}

	return &Int64Histogram{
		Int64Histogram: histogram,
		baseMetric: baseMetric{
			labelNames: labelNames,
		},
	}, nil
}

// NewInt64Gauge 创建带预置标签的 Int64Gauge
func NewInt64Gauge(name string, labelNames []string, options ...metric.Int64GaugeOption) (*Int64Gauge, error) {
	gauge, err := otel.Meter(Name).Int64Gauge(name, options...)
	if err != nil {
		return nil, err
	}

	return &Int64Gauge{
		Int64Gauge: gauge,
		baseMetric: baseMetric{
			labelNames: labelNames,
		},
	}, nil
}

// NewFloat64Counter 创建带预置标签的 Float64Counter
func NewFloat64Counter(name string, labelNames []string, options ...metric.Float64CounterOption) (*Float64Counter, error) {
	counter, err := otel.Meter(Name).Float64Counter(name, options...)
	if err != nil {
		return nil, err
	}

	return &Float64Counter{
		Float64Counter: counter,
		baseMetric: baseMetric{
			labelNames: labelNames,
		},
	}, nil
}

// NewFloat64UpDownCounter 创建带预置标签的 Float64UpDownCounter
func NewFloat64UpDownCounter(name string, labelNames []string, options ...metric.Float64UpDownCounterOption) (*Float64UpDownCounter, error) {
	counter, err := otel.Meter(Name).Float64UpDownCounter(name, options...)
	if err != nil {
		return nil, err
	}

	return &Float64UpDownCounter{
		Float64UpDownCounter: counter,
		baseMetric: baseMetric{
			labelNames: labelNames,
		},
	}, nil
}

// NewFloat64Histogram 创建带预置标签的 Float64Histogram
func NewFloat64Histogram(name string, labelNames []string, options ...metric.Float64HistogramOption) (*Float64Histogram, error) {
	histogram, err := otel.Meter(Name).Float64Histogram(name, options...)
	if err != nil {
		return nil, err
	}

	return &Float64Histogram{
		Float64Histogram: histogram,
		baseMetric: baseMetric{
			labelNames: labelNames,
		},
	}, nil
}

// NewFloat64Gauge 创建带预置标签的 Float64Gauge
func NewFloat64Gauge(name string, labelNames []string, options ...metric.Float64GaugeOption) (*Float64Gauge, error) {
	gauge, err := otel.Meter(Name).Float64Gauge(name, options...)
	if err != nil {
		return nil, err
	}

	return &Float64Gauge{
		Float64Gauge: gauge,
		baseMetric: baseMetric{
			labelNames: labelNames,
		},
	}, nil
}

// ========== 包装器实现 ==========

// Int64Counter 带预置标签的 Int64Counter
type Int64Counter struct {
	metric.Int64Counter
	baseMetric
}

func (c *Int64Counter) Add(ctx context.Context, incr int64, opts ...metric.AddOption) {
	c.Int64Counter.Add(ctx, incr, append(opts, metric.WithAttributes(c.attrs...))...)
}

func (c *Int64Counter) With(labelValues ...string) *Int64Counter {
	c.baseMetric.With(labelValues)
	return c
}

// Int64UpDownCounter 带预置标签的 Int64UpDownCounter
type Int64UpDownCounter struct {
	metric.Int64UpDownCounter
	baseMetric
}

func (c *Int64UpDownCounter) Add(ctx context.Context, incr int64, opts ...metric.AddOption) {
	c.Int64UpDownCounter.Add(ctx, incr, append(opts, metric.WithAttributes(c.attrs...))...)
}

// Int64Histogram 带预置标签的 Int64Histogram
type Int64Histogram struct {
	metric.Int64Histogram
	baseMetric
}

func (h *Int64Histogram) Record(ctx context.Context, incr int64, opts ...metric.RecordOption) {
	h.Int64Histogram.Record(ctx, incr, append(opts, metric.WithAttributes(h.attrs...))...)
}

// Int64Gauge 带预置标签的 Int64Gauge
type Int64Gauge struct {
	metric.Int64Gauge
	baseMetric
}

func (g *Int64Gauge) Record(ctx context.Context, incr int64, opts ...metric.RecordOption) {
	g.Int64Gauge.Record(ctx, incr, append(opts, metric.WithAttributes(g.attrs...))...)
}

// Float64Counter 带预置标签的 Float64Counter
type Float64Counter struct {
	metric.Float64Counter
	baseMetric
}

func (c *Float64Counter) Add(ctx context.Context, incr float64, opts ...metric.AddOption) {
	c.Float64Counter.Add(ctx, incr, append(opts, metric.WithAttributes(c.attrs...))...)
}

// Float64UpDownCounter 带预置标签的 Float64UpDownCounter
type Float64UpDownCounter struct {
	metric.Float64UpDownCounter
	baseMetric
}

func (c *Float64UpDownCounter) Add(ctx context.Context, incr float64, opts ...metric.AddOption) {
	c.Float64UpDownCounter.Add(ctx, incr, append(opts, metric.WithAttributes(c.attrs...))...)
}

// Float64Histogram 带预置标签的 Float64Histogram
type Float64Histogram struct {
	metric.Float64Histogram
	baseMetric
}

func (h *Float64Histogram) Record(ctx context.Context, incr float64, opts ...metric.RecordOption) {
	h.Float64Histogram.Record(ctx, incr, append(opts, metric.WithAttributes(h.attrs...))...)
}

// Float64Gauge 带预置标签的 Float64Gauge
type Float64Gauge struct {
	metric.Float64Gauge
	baseMetric
}

func (g *Float64Gauge) Record(ctx context.Context, incr float64, opts ...metric.RecordOption) {
	g.Float64Gauge.Record(ctx, incr, append(opts, metric.WithAttributes(g.attrs...))...)
}
