package metric

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ServerRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "server",
			Subsystem: "requests",
			Name:      "code_total",
			Help:      "The total number of server processed requests",
		}, []string{"kind", "operation", "code", "reason"})

	ClientRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "client",
			Subsystem: "requests",
			Name:      "code_total",
			Help:      "The total number of server processed requests",
		}, []string{"kind", "operation", "code", "reason"})

	ServerRequestHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "server",
			Subsystem: "requests",
			Name:      "duration_ms",
			Help:      "server requests duration(ms).",
			Buckets:   []float64{0.005, 0.01, 0.05, 0.1, 1, 5},
		}, []string{"kind", "operation"})

	ClientRequestHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "client",
			Subsystem: "requests",
			Name:      "duration_ms",
			Help:      "server requests duration(ms).",
			Buckets:   []float64{0.005, 0.01, 0.05, 0.1, 1, 5},
		}, []string{"kind", "operation"})
	MT5StateGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mt5_state",
			Help: "MT5 server state",
		}, []string{
			"kind", // user_real_total/user_real_limit/license_date
		})
	MT5GatewayStateGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mt5_gateway_state",
			Help: "MT5 gateway state",
		}, []string{
			"kind", // gateway_connections/feeder_connections/quotes_count/trades_count/trade_average_time
			"gateway_name",
		})
)

func NewConnectionsGauge(labelNames ...string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "connections_total",
		Help: "The total number of connections in memory like (fix/grpc stream/ws/tcp)",
	}, append([]string{"kind"}, labelNames...))
}

func NewQuoteCounter(labelNames ...string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "quote_count",
			Help: "The total number of symbol quote",
		}, append([]string{"symbol"}, labelNames...))
}
