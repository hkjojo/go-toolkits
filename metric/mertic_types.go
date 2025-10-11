package metric

import (
	"context"

	dto "github.com/prometheus/client_model/go"
)

// Mode 定义metric采集模式
type Mode string

const (
	ModeLog         Mode = "log"         // 日志模式
	ModeOpenObserve Mode = "openobserve" // OpenObserve模式
	ModeOTEL        Mode = "otel"        // OpenTelemetry模式
)

// Exporter 定义metric导出器接口
type Exporter interface {
	// Export 导出metric数据
	Export(mf *dto.MetricFamily)
	// OnError 处理错误
	OnError(error)
	// Shutdown 优雅关闭
	Shutdown(ctx context.Context) error
	// IsStart 是否启用
	IsStart() bool
}

// Logger 定义日志接口
type Logger interface {
	Errorw(string, ...interface{})
	Warnw(string, ...interface{})
	Infow(string, ...interface{})
}

// Counter 计数器接口
type Counter interface {
	With(lvs ...string) Counter
	Inc()
	Add(delta float64)
}

// Gauge 仪表盘接口
type Gauge interface {
	With(lvs ...string) Gauge
	Delete(lvs ...string) bool
	Set(value float64)
	Add(delta float64)
	Sub(delta float64)
}

// Observer 观察者接口（用于Histogram和Summary）
type Observer interface {
	With(lvs ...string) Observer
	Observe(float64)
}

// MetricFactory metric工厂接口
type MetricFactory interface {
	NewCounter(name, description, unit string) Counter
	NewGauge(name, description, unit string) Gauge
	NewHistogram(name, description, unit string, buckets ...float64) Observer
	NewSummary(name, description, unit string) Observer
}
