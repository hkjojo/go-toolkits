package metric

import (
	"context"
	"strings"

	"github.com/hkjojo/go-toolkits/apptools"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// 单位常量（已在 metric.go 中定义，这里重新导出方便使用）
const (
	UnitDimensionless = "1"      // 无量纲
	UnitBytes         = "By"     // 字节
	UnitMilliseconds  = "ms"     // 毫秒
	UnitSeconds       = "s"      // 秒
	UnitMicroseconds  = "us"     // 微秒
	UnitNanoseconds   = "ns"     // 纳秒
	UnitPercent       = "%"      // 百分比
	UnitCall          = "{call}" // 调用次数
)

type labels []attribute.KeyValue
type labelNames []string

// With 设置标签值
func (b labelNames) With(labelValues []string) labels {
	maxIndex := min(len(labelValues), len(b))
	attrs := make([]attribute.KeyValue, 0, maxIndex)
	for i := range maxIndex {
		attrs = append(attrs, attribute.String(b[i], labelValues[i]))
	}
	return attrs
}

// withPrefix 如果在全局配置中设置了 DefaultPrefix，则为指标名加上该前缀。
// 示例：prefix="td"，name="requests_total" => "td_requests_total"
func withPrefix(name string) string {
	if globalConfig != nil && globalConfig.DefaultPrefix != "" {
		p := globalConfig.DefaultPrefix
		if !strings.HasPrefix(name, p+"_") {
			return p + "_" + name
		}
	}
	return name
}

// NewInt64Counter 创建带预置标签的 Int64Counter
func NewInt64Counter(name string, labelNames []string, options ...metric.Int64CounterOption) *Int64Counter {
	counter, err := otel.Meter(apptools.Name).Int64Counter(withPrefix(name), options...)
	if err != nil {
		panic(err)
	}
	return &Int64Counter{
		Int64Counter: counter,
		labelNames:   labelNames,
	}
}

// NewInt64UpDownCounter 创建带预置标签的 Int64UpDownCounter
func NewInt64UpDownCounter(name string, labelNames []string, options ...metric.Int64UpDownCounterOption) *Int64UpDownCounter {
	counter, err := otel.Meter(apptools.Name).Int64UpDownCounter(withPrefix(name), options...)
	if err != nil {
		panic(err)
	}

	return &Int64UpDownCounter{
		Int64UpDownCounter: counter,
		labelNames:         labelNames,
	}
}

// NewInt64Histogram 创建带预置标签的 Int64Histogram
func NewInt64Histogram(name string, labelNames []string, options ...metric.Int64HistogramOption) *Int64Histogram {
	histogram, err := otel.Meter(apptools.Name).Int64Histogram(withPrefix(name), options...)
	if err != nil {
		panic(err)
	}

	return &Int64Histogram{
		Int64Histogram: histogram,
		labelNames:     labelNames,
	}
}

// NewInt64Gauge 创建带预置标签的 Int64Gauge
func NewInt64Gauge(name string, labelNames []string, options ...metric.Int64GaugeOption) *Int64Gauge {
	gauge, err := otel.Meter(apptools.Name).Int64Gauge(withPrefix(name), options...)
	if err != nil {
		panic(err)
	}

	return &Int64Gauge{
		Int64Gauge: gauge,
		labelNames: labelNames,
	}
}

// NewFloat64Counter 创建带预置标签的 Float64Counter
func NewFloat64Counter(name string, labelNames []string, options ...metric.Float64CounterOption) *Float64Counter {
	counter, err := otel.Meter(apptools.Name).Float64Counter(withPrefix(name), options...)
	if err != nil {
		panic(err)
	}

	return &Float64Counter{
		Float64Counter: counter,
		labelNames:     labelNames,
	}
}

// NewFloat64UpDownCounter 创建带预置标签的 Float64UpDownCounter
func NewFloat64UpDownCounter(name string, labelNames []string, options ...metric.Float64UpDownCounterOption) *Float64UpDownCounter {
	counter, err := otel.Meter(apptools.Name).Float64UpDownCounter(withPrefix(name), options...)
	if err != nil {
		panic(err)
	}

	return &Float64UpDownCounter{
		Float64UpDownCounter: counter,
		labelNames:           labelNames,
	}
}

// NewFloat64Histogram 创建带预置标签的 Float64Histogram
func NewFloat64Histogram(name string, labelNames []string, options ...metric.Float64HistogramOption) *Float64Histogram {
	histogram, err := otel.Meter(apptools.Name).Float64Histogram(withPrefix(name), options...)
	if err != nil {
		panic(err)
	}

	return &Float64Histogram{
		Float64Histogram: histogram,
		labelNames:       labelNames,
	}
}

// NewFloat64Gauge 创建带预置标签的 Float64Gauge
func NewFloat64Gauge(name string, labelNames []string, options ...metric.Float64GaugeOption) *Float64Gauge {
	gauge, err := otel.Meter(apptools.Name).Float64Gauge(withPrefix(name), options...)
	if err != nil {
		panic(err)
	}

	return &Float64Gauge{
		Float64Gauge: gauge,
		labelNames:   labelNames,
	}
}

// ========== 包装器实现 ==========

// Int64Counter 带预置标签的 Int64Counter
type Int64Counter struct {
	metric.Int64Counter
	labelNames labelNames
	labels     labels
}

func (c *Int64Counter) Inc(opts ...metric.AddOption) {
	c.Int64Counter.Add(context.Background(), 1, append(opts, metric.WithAttributes(c.labels...))...)
}

func (c *Int64Counter) Add(incr int64, opts ...metric.AddOption) {
	c.Int64Counter.Add(context.Background(), incr, append(opts, metric.WithAttributes(c.labels...))...)
}

func (c *Int64Counter) With(labelValues ...string) *Int64Counter {
	return &Int64Counter{
		c.Int64Counter,
		c.labelNames,
		c.labelNames.With(labelValues),
	}
}

// Int64UpDownCounter 带预置标签的 Int64UpDownCounter
type Int64UpDownCounter struct {
	metric.Int64UpDownCounter
	labelNames labelNames
	labels     labels
}

func (c *Int64UpDownCounter) Inc(opts ...metric.AddOption) {
	c.Int64UpDownCounter.Add(context.Background(), 1, append(opts, metric.WithAttributes(c.labels...))...)
}

func (c *Int64UpDownCounter) Dec(opts ...metric.AddOption) {
	c.Int64UpDownCounter.Add(context.Background(), -1, append(opts, metric.WithAttributes(c.labels...))...)
}

func (c *Int64UpDownCounter) Add(incr int64, opts ...metric.AddOption) {
	c.Int64UpDownCounter.Add(context.Background(), incr, append(opts, metric.WithAttributes(c.labels...))...)
}

func (c *Int64UpDownCounter) With(labelValues ...string) *Int64UpDownCounter {
	return &Int64UpDownCounter{
		c.Int64UpDownCounter,
		c.labelNames,
		c.labelNames.With(labelValues),
	}
}

// Int64Histogram 带预置标签的 Int64Histogram
type Int64Histogram struct {
	metric.Int64Histogram
	labelNames labelNames
	labels     labels
}

func (h *Int64Histogram) Record(incr int64, opts ...metric.RecordOption) {
	h.Int64Histogram.Record(context.Background(), incr, append(opts, metric.WithAttributes(h.labels...))...)
}

func (h *Int64Histogram) With(labelValues ...string) *Int64Histogram {
	return &Int64Histogram{
		h.Int64Histogram,
		h.labelNames,
		h.labelNames.With(labelValues),
	}
}

// Int64Gauge 带预置标签的 Int64Gauge
type Int64Gauge struct {
	metric.Int64Gauge
	labelNames labelNames
	labels     labels
}

func (g *Int64Gauge) Record(incr int64, opts ...metric.RecordOption) {
	g.Int64Gauge.Record(context.Background(), incr, append(opts, metric.WithAttributes(g.labels...))...)
}

func (g *Int64Gauge) With(labelValues ...string) *Int64Gauge {
	return &Int64Gauge{
		g.Int64Gauge,
		g.labelNames,
		g.labelNames.With(labelValues),
	}
}

// Float64Counter 带预置标签的 Float64Counter
type Float64Counter struct {
	metric.Float64Counter
	labelNames labelNames
	labels     labels
}

func (c *Float64Counter) Add(incr float64, opts ...metric.AddOption) {
	c.Float64Counter.Add(context.Background(), incr, append(opts, metric.WithAttributes(c.labels...))...)
}

func (c *Float64Counter) With(labelValues ...string) *Float64Counter {
	return &Float64Counter{
		c.Float64Counter,
		c.labelNames,
		c.labelNames.With(labelValues),
	}
}

// Float64UpDownCounter 带预置标签的 Float64UpDownCounter
type Float64UpDownCounter struct {
	metric.Float64UpDownCounter
	labelNames labelNames
	labels     labels
}

func (c *Float64UpDownCounter) Add(incr float64, opts ...metric.AddOption) {
	c.Float64UpDownCounter.Add(context.Background(), incr, append(opts, metric.WithAttributes(c.labels...))...)
}

func (c *Float64UpDownCounter) With(labelValues ...string) *Float64UpDownCounter {
	return &Float64UpDownCounter{
		c.Float64UpDownCounter,
		c.labelNames,
		c.labelNames.With(labelValues),
	}
}

// Float64Histogram 带预置标签的 Float64Histogram
type Float64Histogram struct {
	metric.Float64Histogram
	labelNames labelNames
	labels     labels
}

func (h *Float64Histogram) Record(incr float64, opts ...metric.RecordOption) {
	h.Float64Histogram.Record(context.Background(), incr, append(opts, metric.WithAttributes(h.labels...))...)
}

func (h *Float64Histogram) With(labelValues ...string) *Float64Histogram {
	return &Float64Histogram{
		h.Float64Histogram,
		h.labelNames,
		h.labelNames.With(labelValues),
	}
}

// Float64Gauge 带预置标签的 Float64Gauge
type Float64Gauge struct {
	metric.Float64Gauge
	labelNames labelNames
	labels     labels
}

func (g *Float64Gauge) Record(incr float64, opts ...metric.RecordOption) {
	g.Float64Gauge.Record(context.Background(), incr, append(opts, metric.WithAttributes(g.labels...))...)
}

func (g *Float64Gauge) With(labelValues ...string) *Float64Gauge {
	return &Float64Gauge{
		g.Float64Gauge,
		g.labelNames,
		g.labelNames.With(labelValues),
	}
}

// Observable(异步)instrument：回调型，SDK 每导出周期调回调采集当前值，
// 适合“值由业务持有、周期读”的场景（累计原子计数 / 派生 gauge / 动态 label）。
// 与同步构造器共享 withPrefix + otel.Meter(apptools.Name)。
//
// RegisterCallback 传入的 instrument 必须由同一个 meter 创建，因此这些构造器与
// RegisterCallback 都绑定在 apptools.Name 这个 scope——业务侧不要另用别的 meter
// scope 注册回调，否则 instrument 不属于该 meter，观测会被丢弃。

// NewInt64ObservableCounter 异步累计计数器。回调里 Observe 的是**累计总值**，
// SDK 自行换算增量导出，勿手动做 delta。
func NewInt64ObservableCounter(name string, options ...metric.Int64ObservableCounterOption) metric.Int64ObservableCounter {
	c, err := otel.Meter(apptools.Name).Int64ObservableCounter(withPrefix(name), options...)
	if err != nil {
		panic(err)
	}
	return c
}

// NewInt64ObservableGauge 异步 gauge（整型瞬时值）。
func NewInt64ObservableGauge(name string, options ...metric.Int64ObservableGaugeOption) metric.Int64ObservableGauge {
	g, err := otel.Meter(apptools.Name).Int64ObservableGauge(withPrefix(name), options...)
	if err != nil {
		panic(err)
	}
	return g
}

// NewFloat64ObservableGauge 异步 gauge（浮点瞬时值，如秒级时长）。
func NewFloat64ObservableGauge(name string, options ...metric.Float64ObservableGaugeOption) metric.Float64ObservableGauge {
	g, err := otel.Meter(apptools.Name).Float64ObservableGauge(withPrefix(name), options...)
	if err != nil {
		panic(err)
	}
	return g
}

// RegisterCallback 注册一个覆盖多个 observable instrument 的回调。回调需在有限时间内
// 完成、可重入、并发安全，且避免在回调内加锁（见 OTel 文档）。instrument 必须来自本包
// 的 NewXxxObservable* 构造器（同 meter scope）。
func RegisterCallback(f metric.Callback, instruments ...metric.Observable) (metric.Registration, error) {
	return otel.Meter(apptools.Name).RegisterCallback(f, instruments...)
}
