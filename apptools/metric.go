package apptools

import (
	"context"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/go-logr/stdr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

var (
	// 全局配置
	globalConfig = &Config{
		Interval:     time.Minute,
		WithoutUp:    true,
		CollectStats: false,
		Debug:        false,
	}
)

func NewMetricProvider(ops ...Option) (metric.MeterProvider, func(), error) {
	initConfig(ops...)
	ctx := context.Background()

	options := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithInsecure(),
	}
	if globalConfig.Endpoint != "" {
		options = append(options, otlpmetricgrpc.WithEndpoint(globalConfig.Endpoint))
	}

	exporter, err := otlpmetricgrpc.New(ctx,
		options...,
	)
	if err != nil {
		return nil, nil, err
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(Name),
			semconv.ServiceVersion(Version),
			semconv.DeploymentEnvironment(Env),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	// Create meter provider with period reader
	exporter.Temporality(0)
	exporter.Aggregation(sdkmetric.InstrumentKindHistogram)

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(globalConfig.Interval))),
		sdkmetric.WithResource(res),
	)

	// Set global meter provider
	otel.SetMeterProvider(mp)

	if globalConfig.Debug {
		stdr.SetVerbosity(8)
		otel.SetLogger(stdr.New(log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)))
	}

	if globalConfig.InitCallback != nil {
		globalConfig.InitCallback()
	}

	// 注册up指标
	if !globalConfig.WithoutUp {
		registerUpMetric()
	}

	// 注册运行时统计指标
	if globalConfig.CollectStats {
		err := registerStatsMetric()
		if err != nil {
			return nil, nil, err
		}
	}

	shutdown := func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := mp.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}

	return mp, shutdown, nil
}

func initConfig(ops ...Option) {
	// 先从环境变量读取默认值
	if prefix := os.Getenv("METRIC_DEFAULT_PREFIX"); prefix != "" {
		globalConfig.DefaultPrefix = prefix
	}
	if debug := os.Getenv("METRIC_LOG_DEBUG"); debug != "" {
		globalConfig.Debug = debug == "true"
	}

	for _, option := range ops {
		option(globalConfig)
	}
}

// registerUpMetric 注册服务状态指标 "server_name"
func registerUpMetric() {
	upGauge, err := otel.Meter(Name).Int64Gauge(
		"server_up",
		metric.WithDescription("The service up status"),
	)
	if err != nil {
		panic(err)
	}
	upGauge.Record(context.Background(), 1, metric.WithAttributes(attribute.String("server_name", Name)))
}

// collectRuntimeStats 注册运行时统计指标
func registerStatsMetric() error {
	_, err := otel.Meter(Name).Float64ObservableGauge(
		"runtime",
		metric.WithDescription("The service runtime stats"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			numRoutines := runtime.NumGoroutine()
			numCgoCall := runtime.NumCgoCall()
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			o.Observe(float64(numRoutines), metric.WithAttributes(attribute.String("stats", "num_goroutines")))
			o.Observe(float64(numCgoCall), metric.WithAttributes(attribute.String("stats", "num_cgo_call")))
			o.Observe(float64(stats.Sys), metric.WithAttributes(attribute.String("stats", "sys_bytes")))
			o.Observe(float64(stats.Mallocs), metric.WithAttributes(attribute.String("stats", "malloc_count")))
			o.Observe(float64(stats.Frees), metric.WithAttributes(attribute.String("stats", "free_count")))
			o.Observe(float64(stats.Alloc), metric.WithAttributes(attribute.String("stats", "alloc_bytes")))
			o.Observe(float64(stats.HeapObjects), metric.WithAttributes(attribute.String("stats", "heap_objects")))
			o.Observe(float64(stats.StackSys), metric.WithAttributes(attribute.String("stats", "stack_sys_bytes")))
			return nil
		}),
	)
	return err
}

// ServerRequestCounter "kind", "operation", "code", "reason"
func ServerRequestCounter() *Int64Counter {
	return NewInt64Counter(
		"server_requests_code_total",
		[]string{"kind", "operation", "code", "reason"},
		metric.WithDescription("The total number of server processed requests"),
	)
}

// ClientRequestCounter "kind", "operation", "code", "reason"
func ClientRequestCounter() *Int64Counter {
	return NewInt64Counter(
		"client_requests_code_total",
		[]string{"kind", "operation", "code", "reason"},
		metric.WithDescription("The total number of client processed requests"),
	)
}

// ServerRequestHistogram "kind", "operation"
func ServerRequestHistogram() *Float64Histogram {
	return NewFloat64Histogram(
		"server_requests_duration",
		[]string{"kind", "operation"},
		metric.WithDescription("The duration of HTTP requests processed by the server"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.05, 0.1, 1, 5),
	)
}

// ClientRequestHistogram "kind", "operation"
func ClientRequestHistogram() *Float64Histogram {
	return NewFloat64Histogram(
		"client_requests_duration",
		[]string{"kind", "operation"},
		metric.WithDescription("The duration of HTTP requests processed by the client"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.05, 0.1, 1, 5),
	)
}

// NewConnectionsCounter "kind"
func NewConnectionsCounter() *Int64UpDownCounter {
	return NewInt64UpDownCounter(
		"network_connections_total",
		[]string{"kind"},
		metric.WithDescription("The total number of connections in memory like (fix/grpc stream/ws/tcp"),
	)
}

// NewQuoteCounter "symbol"
func NewQuoteCounter() *Int64Counter {
	return NewInt64Counter(
		"symbol_quote_count",
		[]string{"symbol"},
		metric.WithDescription("The total number of symbol quote"),
	)
}

// Config metric配置
type Config struct {
	Endpoint      string        // 导出端点
	Interval      time.Duration // 采集间隔
	WithoutUp     bool          // 是否跳过up指标
	CollectStats  bool          // 是否采集运行时统计
	DefaultPrefix string        // 默认指标前缀
	Debug         bool          // 调试模式
	InitCallback  func()        // 初始化回调函数
}

// Option 配置选项函数
type Option func(*Config)

// WithInterval 设置采集间隔
func WithInterval(d time.Duration) Option {
	return func(cfg *Config) {
		cfg.Interval = d
	}
}

// WithDebug 设置调试模式
func WithDebug(debug bool) Option {
	return func(cfg *Config) {
		cfg.Debug = debug
	}
}

// WithoutUpMetric 跳过up指标
func WithoutUpMetric() Option {
	return func(cfg *Config) {
		cfg.WithoutUp = true
	}
}

// WithStatsMetric 启用运行时统计指标
func WithStatsMetric() Option {
	return func(cfg *Config) {
		cfg.CollectStats = true
	}
}

// WithEndpoint 设置导出端点
func WithEndpoint(endpoint string) Option {
	return func(cfg *Config) {
		cfg.Endpoint = endpoint
	}
}

// WithDefaultPrefix 设置默认前缀
func WithDefaultPrefix(prefix string) Option {
	return func(cfg *Config) {
		cfg.DefaultPrefix = prefix
	}
}

// WithInitCallback 设置默认前缀
func WithInitCallback(fn func()) Option {
	return func(cfg *Config) {
		cfg.InitCallback = fn
	}
}
