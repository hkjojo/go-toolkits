package metric

func ServerRequestCounter() Counter {
	return NewCounter(
		"",
		"",
		"server_requests_code_total",
		"The total number of server processed requests",
		[]string{"kind", "operation", "code", "reason"})
}

func ClientRequestCounter() Counter {
	return NewCounter(
		"",
		"",
		"client_requests_code_total",
		"The total number of client processed requests",
		[]string{"kind", "operation", "code", "reason"})
}

func ServerRequestHistogram() Observer {
	return NewHistogram(
		"",
		"",
		"server_requests_duration",
		"The duration of HTTP requests processed by the server",
		[]string{"kind", "operation"},
		0.005, 0.01, 0.05, 0.1, 1, 5)
}

func ClientRequestHistogram() Observer {
	return NewHistogram(
		"",
		"",
		"client_requests_duration",
		"The duration of HTTP requests processed by the client",
		[]string{"kind", "operation"},
		0.005, 0.01, 0.05, 0.1, 1, 5)
}

func MT5StateGauge() Gauge {
	return NewGauge(
		"",
		"",
		"mt5_server_state",
		"The state of MT5 server",
		[]string{"kind"}) // user_real_total/user_real_limit/license_date
}

func MT5GatewayStateGauge() Gauge {
	return NewGauge(
		"",
		"",
		"mt5_gateway_state",
		"The state of MT5 gateway",
		[]string{"kind", "gateway_name"}) // gateway_connections/feeder_connections/quotes_count/trades_count/trade_average_time
}

func NewConnectionsGauge(labelNames ...string) Gauge {
	return NewGauge(
		"",
		"",
		"network_connections_total",
		"The total number of connections in memory like (fix/grpc stream/ws/tcp)",
		append([]string{"kind"}, labelNames...))
}

func NewQuoteCounter(labelNames ...string) Counter {
	return NewCounter(
		"",
		"",
		"symbol_quote_count",
		"The total number of symbol quote",
		append([]string{"symbol"}, labelNames...))
}
