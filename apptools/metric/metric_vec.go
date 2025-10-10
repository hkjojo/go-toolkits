package metric

import (
	"github.com/hkjojo/go-toolkits/apptools"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// 指标名称常量
const (
	ServerRequestsTotal     = "server_requests_code_total"
	ClientRequestsTotal     = "client_requests_code_total"
	ServerRequestsDuration  = "server_requests_seconds_bucket"
	ClientRequestsDuration  = "client_requests_seconds_bucket"
	ServerUp                = "server_up"
	NetworkConnectionsTotal = "network_connections"
	SymbolQuoteTotal        = "symbol_quote_total"
)

// ServerRequestCounter 返回服务端请求计数器
func ServerRequestCounter() metric.Int64Counter {
	counter, err := otel.Meter(apptools.Name).Int64Counter(
		ServerRequestsTotal,
		metric.WithUnit(UnitCall),
		metric.WithDescription("The total number of server processed requests"))
	if err != nil {
		panic(err)
	}
	return counter
}

// ClientRequestCounter 返回客户端请求计数器
func ClientRequestCounter() metric.Int64Counter {
	counter, err := otel.Meter(apptools.Name).Int64Counter(
		ClientRequestsTotal,
		metric.WithUnit(UnitCall),
		metric.WithDescription("The total number of client processed requests"))
	if err != nil {
		panic(err)
	}
	return counter
}

// ServerRequestHistogram 返回服务端请求耗时直方图
func ServerRequestHistogram() metric.Float64Histogram {
	histogram, err := otel.Meter(apptools.Name).Float64Histogram(
		ServerRequestsDuration,
		metric.WithUnit(UnitSeconds),
		metric.WithDescription("The duration of HTTP requests processed by the server"))
	if err != nil {
		panic(err)
	}
	return histogram
}

// ClientRequestHistogram 返回客户端请求耗时直方图
func ClientRequestHistogram() metric.Float64Histogram {
	histogram, err := otel.Meter(apptools.Name).Float64Histogram(
		ClientRequestsDuration,
		metric.WithUnit(UnitSeconds),
		metric.WithDescription("The duration of HTTP requests processed by the client"))
	if err != nil {
		panic(err)
	}
	return histogram
}

// NewConnectionsCounter 创建网络连接计数器
// 标签: kind - 连接类型（fix/grpc/ws/tcp 等）
func NewConnectionsCounter() *Int64UpDownCounter {
	return NewInt64UpDownCounter(
		NetworkConnectionsTotal,
		[]string{"kind"},
		metric.WithDescription("The total number of connections in memory like (fix/grpc stream/ws/tcp)"),
		metric.WithUnit(UnitDimensionless),
	)
}

// NewQuoteCounter 创建行情计数器
// 标签: symbol - 交易符号
func NewQuoteCounter() *Int64Counter {
	return NewInt64Counter(
		SymbolQuoteTotal,
		[]string{"symbol"},
		metric.WithDescription("The total number of symbol quote"),
		metric.WithUnit(UnitDimensionless),
	)
}
