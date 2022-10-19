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

	ConnectionsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "connections_total",
			Help: "The total number of connections in memory like (fix/grpc stream/ws/tcp)",
		}, []string{"kind"})

	QuoteCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "quote_count",
			Help: "The total number of symbol quote",
		}, []string{"symbol"})
)
