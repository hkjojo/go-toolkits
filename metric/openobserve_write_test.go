package metric

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

func TestConvertOne(t *testing.T) {
	registry := prometheus.NewRegistry()

	_counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "http_request_connect",
		Help:        "help of counter",
		ConstLabels: nil,
	})
	_gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "http_request_connecting",
		Help:        "help of gauge",
		ConstLabels: nil,
	})

	_histogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "A histogram of the HTTP request durations in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.1, 1.5, 5),
	})

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		_counter,
		_gauge,
		_histogram,
	)

	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

	timer := time.NewTimer(time.Second * 30)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return
		case <-ticker.C:
			_counter.Add(1)
			_gauge.Add(1)

			mfs, err := registry.Gather()
			if err != nil {
				return
			}

			for _, mf := range mfs {
				ts, err := convertOne(mf)
				if err != nil {
					t.Error(err)
					return
				}
				metadata := getMetadata(mf)
				t.Log(ts, metadata)
			}
		}
	}
}
