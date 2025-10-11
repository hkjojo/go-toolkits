package metric

import (
	"context"
	"crypto/md5"
	"fmt"
	"sort"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ Gauge = (*otelGauge)(nil)

type otelGauge struct {
	gauge      metric.Float64UpDownCounter
	labelNames []string
	attrs      []attribute.KeyValue
	values     *sync.Map // 存储每个标签组合的当前值
}

// newOTelGauge creates a new OpenTelemetry gauge and returns Gauge.
func newOTelGauge(name, description string, labelNames []string) Gauge {
	gauge, err := otel.Meter(globalConfig.ServiceName).Float64UpDownCounter(
		name,
		metric.WithDescription(description),
	)
	if err != nil {
		panic(err)
	}
	return &otelGauge{
		gauge:      gauge,
		labelNames: labelNames,
		values:     &sync.Map{},
	}
}

func (g *otelGauge) With(labelValues ...string) Gauge {
	maxIndex := min(len(labelValues), len(g.labelNames))
	attrs := make([]attribute.KeyValue, 0, maxIndex)
	for i := 0; i < maxIndex; i++ {
		attrs = append(attrs, attribute.String(g.labelNames[i], labelValues[i]))
	}
	return &otelGauge{
		gauge:      g.gauge,
		labelNames: g.labelNames,
		attrs:      attrs,
		values:     g.values, // 共享values存储
	}
}

// makeAttrKey 生成基于属性的唯一key
func (g *otelGauge) makeAttrKey() string {
	if len(g.attrs) == 0 {
		return "_default_"
	}

	// 创建一个可排序的键值对切片
	pairs := make([]string, 0, len(g.attrs))
	for _, attr := range g.attrs {
		pairs = append(pairs, fmt.Sprintf("%s=%s", string(attr.Key), attr.Value.AsString()))
	}

	// 排序以确保一致性
	sort.Strings(pairs)

	// 生成MD5哈希作为key
	data := fmt.Sprintf("%v", pairs)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func (g *otelGauge) Delete(lvs ...string) bool {
	// OpenTelemetry doesn't support deleting specific label combinations
	// This is a limitation compared to Prometheus
	// but we can delete the value from the local storage
	key := g.makeAttrKey()
	g.values.Delete(key)
	return false // return false means OTEL does not support true deletion
}

func (g *otelGauge) Set(value float64) {
	// OpenTelemetry UpDownCounter doesn't have a Set method
	// We need to track the current value and calculate the delta
	key := g.makeAttrKey()

	// use CompareAndSwap to implement atomic Set operation (Go 1.20+)
	for {
		oldValue, exists := g.values.Load(key)
		var delta float64

		if exists {
			delta = value - oldValue.(float64)
		} else {
			delta = value // first set
		}

		// atomically compare and swap value
		if g.values.CompareAndSwap(key, oldValue, value) {
			// successfully update the internal state, send delta to OTEL
			g.gauge.Add(context.Background(), delta, metric.WithAttributes(g.attrs...))
			break
		}
		// CAS failed, retry (说明有其他goroutine同时修改了这个key）
	}
}

func (g *otelGauge) Add(delta float64) {
	// update the value in the internal storage
	key := g.makeAttrKey()

	// use CompareAndSwap to implement atomic update (Go 1.20+)
	for {
		oldValue, exists := g.values.Load(key)
		var newValue float64
		if exists {
			newValue = oldValue.(float64) + delta
		} else {
			newValue = delta
		}

		// atomically compare and swap value
		if g.values.CompareAndSwap(key, oldValue, newValue) {
			break // 成功更新，退出循环
		}
		// CAS failed, retry (说明有其他goroutine同时修改了这个key）
	}

	// add to OTEL gauge
	g.gauge.Add(context.Background(), delta, metric.WithAttributes(g.attrs...))
}

func (g *otelGauge) Sub(delta float64) {
	g.Add(-delta)
}
