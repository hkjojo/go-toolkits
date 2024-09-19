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
	Delete(lvs ...string) bool
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

	// default runtime gauge
	runtimeGauge Gauge
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

	if dc.collectStats {
		registerStats()
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
	if dc.collectStats {
		collectStats()
	}

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

func registerStats() {
	runtimeGauge = NewGauge(prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runtime",
		Help: "the service stats",
	}, []string{"stats"}))
}

func collectStats() {
	numRoutines := runtime.NumGoroutine()
	runtimeGauge.With("num_goroutines").Set(float64(numRoutines))
	numCgoCall := runtime.NumCgoCall()
	runtimeGauge.With("num_cgo_call").Set(float64(numCgoCall))
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	// system
	runtimeGauge.With("sys_bytes").Set(float64(stats.Sys))
	// heap
	runtimeGauge.With("malloc_count").Set(float64(stats.Mallocs))
	runtimeGauge.With("free_count").Set(float64(stats.Frees))
	runtimeGauge.With("alloc_bytes").Set(float64(stats.Alloc))
	runtimeGauge.With("heap_objects").Set(float64(stats.HeapObjects))
	//stack
	runtimeGauge.With("stack_sys_bytes").Set(float64(stats.StackSys))
	// gc
	runtimeGauge.With("total_gc_pause_ns").Set(float64(stats.PauseTotalNs))
	runtimeGauge.With("total_gc_runs").Set(float64(stats.NumGC))
}
