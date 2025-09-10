package metric

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	// 全局配置
	globalConfig *Config

	// Prometheus注册器（仅在prometheus模式下使用）
	prometheusRegistry = prometheus.NewRegistry()

	// 运行时指标收集器
	runtimeGauge Gauge
	exporter     Exporter
)

// Start 启动metric采集
func Start(logger Logger, options ...Option) (func(), error) {
	globalConfig = &Config{
		Mode:     ModeOTEL, // 默认使用OTEL模式
		Interval: time.Minute,
	}

	// 应用选项
	for _, option := range options {
		option(globalConfig)
	}

	if logger != nil {
		switch globalConfig.Mode {
		case ModeLog:
			exporter = newJSONLoggerExporter(logger)
		case ModeOpenObserve:
			// 创建OpenObserve导出器选项
			exporter = newOpenobserveExporter(logger,
				globalConfig.Endpoint,
				globalConfig.ServiceName,
				globalConfig.StreamName,
			)
		case ModeOTEL:
			// 创建OTEL导出器选项
			exporter = newOTELExporter(
				logger,
				globalConfig.Endpoint,
				globalConfig.Interval,
				globalConfig.ServiceName,
				globalConfig.ServiceVersion,
				globalConfig.Env,
			)
		default:
			return nil, errors.New(fmt.Sprintf("unsupported mode: %s", globalConfig.Mode))
		}
	}

	if !exporter.IsStart() {
		return nil, errors.New("no exporter provided")
	}

	// 注册up指标
	if !globalConfig.WithoutUp {
		registerUpMetric()
	}

	// 注册运行时统计指标
	if globalConfig.CollectStats {
		registerStatsMetric()
	}

	// 启动定时采集
	ticker := time.NewTicker(globalConfig.Interval)
	go func() {
		for range ticker.C {
			collectMetrics()
		}
	}()

	// 返回停止函数
	return func() {
		ticker.Stop()
		if exporter != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			exporter.Shutdown(ctx)
		}
	}, nil
}

// collectMetrics 收集并导出指标
func collectMetrics() {
	// 收集运行时统计
	if globalConfig.CollectStats {
		collectRuntimeStats()
	}

	// 根据模式处理指标导出
	switch globalConfig.Mode {
	case ModeLog, ModeOpenObserve:
		// Prometheus模式：手动收集和导出
		mfs, err := prometheusRegistry.Gather()
		if err != nil {
			exporter.OnError(err)
			return
		}

		for _, mf := range mfs {
			exporter.Export(mf)
		}
	case ModeOTEL:
		// OTEL模式：指标由MeterProvider自动发送
		// 无需手动收集
	}
}

// registerUpMetric 注册服务状态指标
func registerUpMetric() {
	switch globalConfig.Mode {
	case ModeOTEL:
		// OpenTelemetry模式
		meter := otel.Meter("go-toolkits/metric")
		gauge, err := meter.Float64UpDownCounter(
			"up",
			metric.WithDescription("the service up status"),
			metric.WithUnit("1"),
		)
		if err == nil {
			runtimeGauge = &otelGauge{gauge: gauge}
			runtimeGauge.With("go_version", runtime.Version()).Set(1)
		}
	case ModeLog, ModeOpenObserve:
		// Prometheus模式
		gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "up",
			Help: "the service up status",
		}, []string{"go_version"})
		prometheusRegistry.MustRegister(gaugeVec)
		runtimeGauge = &prometheusGauge{gv: gaugeVec}
		runtimeGauge.With(runtime.Version()).Set(1)
	}
}

// registerStatsMetric 注册运行时统计指标
func registerStatsMetric() {
	switch globalConfig.Mode {
	case ModeOTEL:
		// OpenTelemetry模式
		meter := otel.Meter("go-toolkits/metric")
		gauge, err := meter.Float64UpDownCounter(
			"runtime",
			metric.WithDescription("the service runtime stats"),
			metric.WithUnit("1"),
		)
		if err == nil {
			runtimeGauge = &otelGauge{gauge: gauge}
		}
	case ModeLog, ModeOpenObserve:
		// Prometheus模式
		gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "runtime",
			Help: "the service runtime stats",
		}, []string{"stats"})
		prometheusRegistry.MustRegister(gaugeVec)
		runtimeGauge = &prometheusGauge{gv: gaugeVec}
	}
}

// collectRuntimeStats 收集运行时统计信息
func collectRuntimeStats() {
	if runtimeGauge == nil {
		return
	}

	numRoutines := runtime.NumGoroutine()
	numCgoCall := runtime.NumCgoCall()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	switch globalConfig.Mode {
	case ModeOTEL:
		// OpenTelemetry模式
		runtimeGauge.With("stats", "num_goroutines").Set(float64(numRoutines))
		runtimeGauge.With("stats", "num_cgo_call").Set(float64(numCgoCall))
		runtimeGauge.With("stats", "sys_bytes").Set(float64(stats.Sys))
		runtimeGauge.With("stats", "malloc_count").Set(float64(stats.Mallocs))
		runtimeGauge.With("stats", "free_count").Set(float64(stats.Frees))
		runtimeGauge.With("stats", "alloc_bytes").Set(float64(stats.Alloc))
		runtimeGauge.With("stats", "heap_objects").Set(float64(stats.HeapObjects))
		runtimeGauge.With("stats", "stack_sys_bytes").Set(float64(stats.StackSys))
		runtimeGauge.With("stats", "total_gc_pause_ns").Set(float64(stats.PauseTotalNs))
		runtimeGauge.With("stats", "total_gc_runs").Set(float64(stats.NumGC))
	case ModeLog, ModeOpenObserve:
		// Prometheus模式
		runtimeGauge.With("num_goroutines").Set(float64(numRoutines))
		runtimeGauge.With("num_cgo_call").Set(float64(numCgoCall))
		runtimeGauge.With("sys_bytes").Set(float64(stats.Sys))
		runtimeGauge.With("malloc_count").Set(float64(stats.Mallocs))
		runtimeGauge.With("free_count").Set(float64(stats.Frees))
		runtimeGauge.With("alloc_bytes").Set(float64(stats.Alloc))
		runtimeGauge.With("heap_objects").Set(float64(stats.HeapObjects))
		runtimeGauge.With("stack_sys_bytes").Set(float64(stats.StackSys))
		runtimeGauge.With("total_gc_pause_ns").Set(float64(stats.PauseTotalNs))
		runtimeGauge.With("total_gc_runs").Set(float64(stats.NumGC))
	}
}

// MustRegister 注册Prometheus收集器（仅在Prometheus模式下有效）
func MustRegister(collector prometheus.Collector) {
	if globalConfig != nil && (globalConfig.Mode == ModeLog || globalConfig.Mode == ModeOpenObserve) {
		prometheusRegistry.MustRegister(collector)
	}
}

// NewCounter 创建计数器
func NewCounter(name, description, unit string) Counter {
	if globalConfig == nil {
		// 默认使用OTEL模式
		return newOTelCounter(name, description, unit)
	}

	switch globalConfig.Mode {
	case ModeLog, ModeOpenObserve:
		counterVec := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: name,
			Help: description,
		}, []string{})
		prometheusRegistry.MustRegister(counterVec)
		return &prometheusCounter{cv: counterVec}
	default:
		return newOTelCounter(name, description, unit)
	}
}

// NewGauge 创建仪表盘
func NewGauge(name, description, unit string) Gauge {
	if globalConfig == nil {
		// 默认使用OTEL模式
		return newOTelGauge(name, description, unit)
	}

	switch globalConfig.Mode {
	case ModeLog, ModeOpenObserve:
		gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: name,
			Help: description,
		}, []string{})
		prometheusRegistry.MustRegister(gaugeVec)
		return &prometheusGauge{gv: gaugeVec}
	default:
		return newOTelGauge(name, description, unit)
	}
}

// NewHistogram 创建直方图
func NewHistogram(name, description, unit string, buckets ...float64) Observer {
	if globalConfig == nil {
		// 默认使用OTEL模式
		return newOTelHistogram(name, description, unit, buckets...)
	}

	switch globalConfig.Mode {
	case ModeLog, ModeOpenObserve:
		histogramVec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    name,
			Help:    description,
			Buckets: buckets,
		}, []string{})
		prometheusRegistry.MustRegister(histogramVec)
		return &prometheusHistogram{hv: histogramVec}
	default:
		return newOTelHistogram(name, description, unit, buckets...)
	}
}

// NewSummary 创建摘要
func NewSummary(name, description, unit string) Observer {
	if globalConfig == nil {
		// 默认使用OTEL模式
		return newOTelSummary(name, description, unit)
	}

	switch globalConfig.Mode {
	case ModeLog, ModeOpenObserve:
		summaryVec := prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Name: name,
			Help: description,
		}, []string{})
		prometheusRegistry.MustRegister(summaryVec)
		return &prometheusSummary{sv: summaryVec}
	default:
		return newOTelSummary(name, description, unit)
	}
}
