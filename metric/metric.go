package metric

import (
	"errors"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Counter is metrics counter.
type Counter interface {
	With(lvs ...string) Counter
	Inc()
	Add(delta float64)
}

// Gauge is metrics gauge.
type Gauge interface {
	With(lvs ...string) Gauge
	Set(value float64)
	Add(delta float64)
	Sub(delta float64)
}

// Observer is metrics observer.
type Observer interface {
	With(lvs ...string) Observer
	Observe(float64)
}

var (
	// default config
	dc = defaultConfig()

	// default register
	register = prometheus.NewRegistry()
)

// MustRegister for the metrics not use NewCounter/NewGauge/New...
func MustRegister(collector prometheus.Collector) {
	register.Register(collector)
}

// Start ...
func Start(options ...Option) (func(), error) {
	for _, option := range options {
		option(dc)
	}

	if dc.writer == nil {
		return nil, errors.New("no writer provided")
	}

	if !dc.withoutUp {
		registerUp()
	}

	ticker := time.NewTicker(dc.interval)
	go func() {
		for range ticker.C {
			metricCollector()
		}
	}()

	return func() {
		ticker.Stop()
	}, nil
}

func metricCollector() {
	mfs, err := register.Gather()
	if err != nil {
		dc.writer.OnError(err)
		return
	}

	for _, mf := range mfs {
		dc.writer.Write(mf)
	}
}

func registerUp() {
	gauge := NewGauge(prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "up",
		Help: "the service up status",
	}, []string{"go_version"}))
	gauge.With(runtime.Version()).Set(1)
}
