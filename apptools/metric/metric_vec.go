package metric

import (
	"go.opentelemetry.io/otel/metric"
)

// 指标名称常量
const (
	ServerRequestsTotal     = "server_requests_code_total"
	ClientRequestsTotal     = "client_requests_code_total"
	ServerRequestsDuration  = "server_requests_seconds"
	ClientRequestsDuration  = "client_requests_seconds"
	ServerUp                = "server_up"
	NetworkConnectionsTotal = "network_connections"
	SymbolQuoteTotal        = "symbol_quote_total"
)

// ServerRequestCounter 返回服务端请求计数器
func ServerRequestCounter() metric.Int64Counter {
	return NewInt64Counter(
		ServerRequestsTotal,
		[]string{},
		metric.WithUnit(UnitCall),
		metric.WithDescription("The total number of server processed requests"),
	).Int64Counter
}

// ClientRequestCounter 返回客户端请求计数器
func ClientRequestCounter() metric.Int64Counter {
	return NewInt64Counter(
		ClientRequestsTotal,
		[]string{},
		metric.WithUnit(UnitCall),
		metric.WithDescription("The total number of client processed requests"),
	).Int64Counter
}

// ServerRequestHistogram 返回服务端请求耗时直方图
func ServerRequestHistogram() metric.Float64Histogram {
	return NewFloat64Histogram(
		ServerRequestsDuration,
		[]string{},
		metric.WithUnit(UnitSeconds),
		metric.WithDescription("The duration of HTTP requests processed by the server"),
	).Float64Histogram
}

// ClientRequestHistogram 返回客户端请求耗时直方图
func ClientRequestHistogram() metric.Float64Histogram {
	return NewFloat64Histogram(
		ClientRequestsDuration,
		[]string{},
		metric.WithUnit(UnitSeconds),
		metric.WithDescription("The duration of HTTP requests processed by the client"),
	).Float64Histogram
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
