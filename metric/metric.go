package metric

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// 全局配置
	globalConfig = &Config{
		Mode:         ModeOTEL,
		Interval:     time.Minute,
		WithoutUp:    true,
		CollectStats: false,
		ServiceName:  "go-tookits",
	}

	// Prometheus注册器（仅在prometheus模式下使用）
	prometheusRegistry *prometheus.Registry

	// 运行时指标收集器
	runtimeGauge Gauge
	exporter     Exporter
)

// Start 启动metric采集
func Start(logger Logger, options ...Option) (func(), error) {
	// 先从环境变量读取默认值
	if ns := os.Getenv("METRIC_DEFAULT_NAMESPACE"); ns != "" {
		globalConfig.DefaultNamespace = ns
	}
	if ss := os.Getenv("METRIC_DEFAULT_SUBSYSTEM"); ss != "" {
		globalConfig.DefaultSubsystem = ss
	}

	// 然后应用 Option，Option 优先级更高
	for _, option := range options {
		option(globalConfig)
	}

	if logger != nil {
		switch globalConfig.Mode {
		case ModeLog:
			exporter = newJSONLoggerExporter(logger)
			prometheusRegistry = prometheus.NewRegistry()
		case ModeOpenObserve:
			// 创建OpenObserve导出器选项
			exporter = newOpenobserveExporter(logger,
				globalConfig.Endpoint,
				globalConfig.ServiceName,
				globalConfig.StreamName,
			)
			prometheusRegistry = prometheus.NewRegistry()
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
			return nil, fmt.Errorf("unsupported mode: %s", globalConfig.Mode)
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

	collector := &Collector{
		ticker:   time.NewTicker(globalConfig.Interval),
		wg:       sync.WaitGroup{},
		stopChan: make(chan struct{}),
	}
	collector.Start()

	// 返回停止函数
	return func() {
		collector.Stop()
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
		newOTelGauge("up", "the service up status", []string{"go_version"}).Set(1)
	case ModeLog, ModeOpenObserve:
		// Prometheus模式
		gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "up",
			Help: "the service up status",
		}, []string{"go_version"})
		prometheusRegistry.MustRegister(gaugeVec)
		gauge := newPrometheusGauge(gaugeVec)
		gauge.With(runtime.Version()).Set(1)
	}
}

// registerStatsMetric 注册运行时统计指标
func registerStatsMetric() {
	switch globalConfig.Mode {
	case ModeOTEL:
		runtimeGauge = newOTelGauge("runtime", "the service runtime stats", []string{"stats"})
	case ModeLog, ModeOpenObserve:
		// Prometheus模式
		gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "runtime",
			Help: "the service runtime stats",
		}, []string{"stats"})
		prometheusRegistry.MustRegister(gaugeVec)
		runtimeGauge = newPrometheusGauge(gaugeVec)
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

// NewCounter 创建计数器
func NewCounter(namespace, subsystem, name, description string, labelNames []string) Counter {
	// 应用默认值
	namespace, subsystem = applyDefaults(namespace, subsystem)

	switch globalConfig.Mode {
	case ModeLog, ModeOpenObserve:
		counterVec := prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      name,
			Help:      description,
		}, labelNames)
		prometheusRegistry.MustRegister(counterVec)
		return newPrometheusCounter(counterVec)
	default:
		fullName := name
		if namespace != "" && subsystem != "" {
			fullName = namespace + "_" + subsystem + "_" + name
		} else if namespace != "" {
			fullName = namespace + "_" + name
		} else if subsystem != "" {
			fullName = subsystem + "_" + name
		}
		return newOTelCounter(fullName, description, labelNames)
	}
}

// NewGauge 创建仪表盘
func NewGauge(namespace, subsystem, name, description string, labelNames []string) Gauge {
	// 应用默认值
	namespace, subsystem = applyDefaults(namespace, subsystem)

	switch globalConfig.Mode {
	case ModeLog, ModeOpenObserve:
		gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      name,
			Help:      description,
		}, labelNames)
		prometheusRegistry.MustRegister(gaugeVec)
		return newPrometheusGauge(gaugeVec)
	default:
		fullName := name
		if namespace != "" && subsystem != "" {
			fullName = namespace + "_" + subsystem + "_" + name
		} else if namespace != "" {
			fullName = namespace + "_" + name
		} else if subsystem != "" {
			fullName = subsystem + "_" + name
		}
		return newOTelGauge(fullName, description, labelNames)
	}
}

// NewHistogram 创建直方图
func NewHistogram(namespace, subsystem, name, description string, labelNames []string, buckets ...float64) Observer {
	// 应用默认值
	namespace, subsystem = applyDefaults(namespace, subsystem)

	switch globalConfig.Mode {
	case ModeLog, ModeOpenObserve:
		histogramVec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      name,
			Help:      description,
			Buckets:   buckets,
		}, labelNames)
		prometheusRegistry.MustRegister(histogramVec)
		return newPrometheusHistogram(histogramVec)
	default:
		fullName := name
		if namespace != "" && subsystem != "" {
			fullName = namespace + "_" + subsystem + "_" + name
		} else if namespace != "" {
			fullName = namespace + "_" + name
		} else if subsystem != "" {
			fullName = subsystem + "_" + name
		}
		return newOTelHistogram(fullName, description, labelNames, buckets...)
	}
}

// NewSummary 创建摘要
func NewSummary(namespace, subsystem, name, description string, labelNames []string) Observer {
	// 应用默认值
	namespace, subsystem = applyDefaults(namespace, subsystem)

	switch globalConfig.Mode {
	case ModeLog, ModeOpenObserve:
		summaryVec := prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      name,
			Help:      description,
		}, labelNames)
		prometheusRegistry.MustRegister(summaryVec)
		return newPrometheusSummary(summaryVec)
	default:
		fullName := name
		if namespace != "" && subsystem != "" {
			fullName = namespace + "_" + subsystem + "_" + name
		} else if namespace != "" {
			fullName = namespace + "_" + name
		} else if subsystem != "" {
			fullName = subsystem + "_" + name
		}
		return newOTelSummary(fullName, description, "1", labelNames)
	}
}

// applyDefaults 应用默认的 namespace 和 subsystem
func applyDefaults(namespace, subsystem string) (string, string) {
	if namespace == "" {
		namespace = globalConfig.DefaultNamespace
	}
	if subsystem == "" {
		subsystem = globalConfig.DefaultSubsystem
	}
	return namespace, subsystem
}

type Collector struct {
	ticker   *time.Ticker
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func (mc *Collector) Start() {
	mc.wg.Add(1)
	go func() {
		defer mc.wg.Done()
		for {
			select {
			case <-mc.ticker.C:
				collectMetrics()
			case <-mc.stopChan:
				return
			}
		}
	}()
}

func (mc *Collector) Stop() {
	close(mc.stopChan)
	mc.ticker.Stop()
	mc.wg.Wait()
}
