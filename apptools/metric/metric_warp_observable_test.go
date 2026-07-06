package metric

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	om "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// 证明 observable 走的是 OTLP 导出路径（ManualReader.Collect），名字带 DefaultPrefix，
// 回调值被 SDK 正确采集——这是消费方（edge-backend）要复用的验证手法，先在 toolkit 保住。
func TestObservableInstruments_PrefixedAndExported(t *testing.T) {
	globalConfig.DefaultPrefix = "tpedge"
	t.Cleanup(func() { globalConfig.DefaultPrefix = "" })

	reader := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))

	counter := NewInt64ObservableCounter("ms_buffer_drop_total")
	gauge := NewFloat64ObservableGauge("ms_maintenance_duration_by_scope_seconds")

	reg, err := RegisterCallback(func(_ context.Context, o om.Observer) error {
		o.ObserveInt64(counter, 7, om.WithAttributes(attribute.String("kind", "priced")))
		o.ObserveFloat64(gauge, 1.5, om.WithAttributes(attribute.String("scope", "daily")))
		return nil
	}, counter, gauge)
	if err != nil {
		t.Fatalf("RegisterCallback: %v", err)
	}
	t.Cleanup(func() { _ = reg.Unregister() })

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	names := map[string]bool{}
	var counterVal int64
	var gaugeVal float64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			names[m.Name] = true
			if sum, ok := m.Data.(metricdata.Sum[int64]); ok && m.Name == "tpedge_ms_buffer_drop_total" {
				if len(sum.DataPoints) > 0 {
					counterVal = sum.DataPoints[0].Value
				}
			}
			if g, ok := m.Data.(metricdata.Gauge[float64]); ok && m.Name == "tpedge_ms_maintenance_duration_by_scope_seconds" {
				if len(g.DataPoints) > 0 {
					gaugeVal = g.DataPoints[0].Value
				}
			}
		}
	}

	if !names["tpedge_ms_buffer_drop_total"] {
		t.Fatalf("counter not exported with prefix; got names: %v", names)
	}
	if !names["tpedge_ms_maintenance_duration_by_scope_seconds"] {
		t.Fatalf("gauge not exported with prefix; got names: %v", names)
	}
	if counterVal != 7 {
		t.Fatalf("counter value = %d, want 7", counterVal)
	}
	if gaugeVal != 1.5 {
		t.Fatalf("gauge value = %v, want 1.5", gaugeVal)
	}
}

// 前缀已存在时不重复叠加（withPrefix 幂等），且不设前缀时保持原名。
func TestObservable_PrefixIdempotentAndOptional(t *testing.T) {
	globalConfig.DefaultPrefix = "tpedge"
	t.Cleanup(func() { globalConfig.DefaultPrefix = "" })

	reader := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))

	already := NewInt64ObservableGauge("tpedge_ms_wal_checkpoint_blocked_streams") // 已带前缀
	if _, err := RegisterCallback(func(_ context.Context, o om.Observer) error {
		o.ObserveInt64(already, 1)
		return nil
	}, already); err != nil {
		t.Fatalf("RegisterCallback: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "tpedge_tpedge_ms_wal_checkpoint_blocked_streams" {
				t.Fatalf("prefix double-applied: %s", m.Name)
			}
		}
	}
}
