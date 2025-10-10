package metric

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/go-logr/stdr"
	"github.com/hkjojo/go-toolkits/apptools"
	contribruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
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

	// 创建资源（Resource），用于描述当前服务的元数据信息
	res, err := resource.New(ctx,
		resource.WithFromEnv(), // 从环境变量中读取资源属性（如 OTEL_RESOURCE_ATTRIBUTES）
		resource.WithHost(),    // 添加主机信息（主机名、操作系统等）
		resource.WithAttributes( // 添加自定义属性
			semconv.ServiceName(apptools.Name),          // 服务名称
			semconv.ServiceVersion(apptools.Version),    // 服务版本
			semconv.DeploymentEnvironment(apptools.Env), // 部署环境（如 dev、staging、prod）
		),
	)
	if err != nil {
		return nil, nil, err
	}

	// 构建 MeterProvider 的配置选项
	opts := []sdkmetric.Option{
		// 使用周期性读取器，按配置的间隔（默认 1 分钟）定期推送指标数据
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(globalConfig.Interval))),
		// 关联资源信息，所有 metric 都会包含这些元数据
		sdkmetric.WithResource(res),
	}

	mp := sdkmetric.NewMeterProvider(opts...)

	// 设置全局的 MeterProvider，后续通过 otel.Meter() 获取的都是这个 provider
	otel.SetMeterProvider(mp)

	if globalConfig.Debug {
		stdr.SetVerbosity(8) // 设置日志详细级别为 8（最详细）
		// 将 OpenTelemetry 的日志输出到标准输出，包含时间戳和文件位置
		otel.SetLogger(stdr.New(log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)))
	}

	// 如果配置了初始化回调函数，执行它
	// 可用于在 MeterProvider 创建后执行自定义的初始化逻辑（如注册自定义指标）
	if globalConfig.InitCallback != nil {
		globalConfig.InitCallback()
	}

	// 注册 "server_up" 指标，用于标识服务是否在线
	// 可通过 WithoutUpMetric() 选项禁用
	if !globalConfig.WithoutUp {
		registerUpMetric()
	}

	// 注册 Go runtime 统计指标（如内存使用、GC 次数、goroutine 数量等）
	// 可通过 WithStatsMetric() 选项启用
	if globalConfig.CollectStats {
		// 启动 runtime 指标收集，最小采集间隔为 10 秒
		err := contribruntime.Start(contribruntime.WithMinimumReadMemStatsInterval(10 * time.Second))
		if err != nil {
			return nil, nil, err
		}
	}

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
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
	upGauge, err := otel.Meter(apptools.Name).Int64Gauge(
		ServerUp,
		metric.WithDescription("The service health status (1 = healthy, 0 = unhealthy)"),
	)
	if err != nil {
		panic(err)
	}
	upGauge.Record(context.Background(), 1, metric.WithAttributes(attribute.String("server_name", apptools.Name)))
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
