package apptools

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

/*
NewTracerProvider Support Env

OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://openobserve.td-ops.com/api/dealer-dev/traces
OTEL_EXPORTER_OTLP_TRACES_TIMEOUT
OTEL_EXPORTER_OTLP_TRACES_INSECURE
OTEL_EXPORTER_OTLP_TRACES_HEADERS=Authorization=Basic YWRtaW5AZXhhbXBsZS5jb206MnZyOHIxZ3h4OXhqTXVvSg==,stream-name=default
OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE
OTEL_EXPORTER_OTLP_TRACES_CLIENT_CERTIFICATE
OTEL_EXPORTER_OTLP_TRACES_COMPRESSION
OTEL_EXPORTER_OTLP_TRACES_CLIENT_KEY

行为说明：endpoint 解析优先级为显式 Option（WithEndpoint）> 信号专用 env
（OTEL_EXPORTER_OTLP_TRACES_ENDPOINT）> 通用 env（OTEL_EXPORTER_OTLP_ENDPOINT）。当三者都
为空时，NewTracerProvider 返回一个 no-op TracerProvider —— 不创建 exporter/batcher，不设置
采样、传播器或全局 TracerProvider，零初始化、零上报、零连接。这是相对旧版本（旧版本会无条件
创建 exporter 并回退到默认 localhost:4317）的行为变更。
*/
func NewTracerProvider(ops ...TraceOption) (trace.TracerProvider, func(), error) {
	opts := newTraceOptions(ops...)

	// 解析 endpoint：显式 Option > env(信号专用 OTEL_EXPORTER_OTLP_TRACES_ENDPOINT) >
	// env(通用 OTEL_EXPORTER_OTLP_ENDPOINT)。都为空时视为未配置，与 SDK envconfig 的优先级一致。
	endpoint := ResolveEndpoint(opts.endpoint,
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
	)
	if endpoint == "" {
		// 真 no-op：完全跳过 exporter/batcher/采样/传播器/全局注册，零初始化、零上报、零连接。
		return tracenoop.NewTracerProvider(), func() {}, nil
	}

	ctx := context.Background()

	grpcOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithInsecure(),
	}
	// 仅当来自 Option 时才显式设置 endpoint（host:port）；来自 env 时交由 SDK
	// 自行解析 OTLP endpoint 环境变量（可能是完整 URL），保持向后兼容。
	if opt := strings.TrimSpace(opts.endpoint); opt != "" {
		grpcOpts = append(grpcOpts, otlptracegrpc.WithEndpoint(opt))
	}

	traceExp, err := otlptracegrpc.New(ctx, grpcOpts...)
	if err != nil {
		return nil, nil, err
	}

	// 创建资源（Resource），用于描述当前服务的元数据信息
	res, err := resource.New(ctx,
		resource.WithFromEnv(), // 从环境变量中读取资源属性（如 OTEL_RESOURCE_ATTRIBUTES）
		resource.WithHost(),    // 添加主机信息（主机名、操作系统等）
		resource.WithAttributes( // 添加自定义属性
			semconv.ServiceNameKey.String(Name),          // 服务名称（使用 apptools.Name 变量）
			semconv.ServiceVersionKey.String(Version),    // 服务版本（使用 apptools.Version 变量）
			semconv.DeploymentEnvironmentKey.String(Env), // 部署环境（如 dev、staging、prod，使用 apptools.Env 变量）
		),
	)
	if err != nil {
		return nil, nil, err
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(traceExp),                // 使用批处理方式发送 trace 数据到导出器，提高性能
		tracesdk.WithResource(res),                    // 关联资源信息，所有 trace 都会包含这些元数据
		tracesdk.WithSampler(tracesdk.AlwaysSample()), // 设置采样策略为"全量采样"（所有 trace 都会被记录）
	)

	// 设置全局的上下文传播器（用于在不同服务间传递 trace 上下文）
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // W3C Trace Context 标准传播格式
		propagation.Baggage{}))     // W3C Baggage 传播格式（用于传递额外的键值对数据）

	// 设置全局的 TracerProvider，后续通过 otel.Tracer() 获取的都是这个 provider
	otel.SetTracerProvider(tp)

	shutdown := func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}

	return tp, shutdown, nil
}
