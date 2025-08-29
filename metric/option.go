package metric

import (
	"time"
)

type config struct {
	interval     time.Duration
	withoutUp    bool
	collectStats bool
	writer       Writer
}

type Option func(*config)

func defaultConfig() *config {
	return &config{
		interval: time.Minute,
	}
}

func WithInterval(d time.Duration) Option {
	return func(cfg *config) {
		cfg.interval = d
	}
}

func WithoutUpMetric() Option {
	return func(cfg *config) {
		cfg.withoutUp = true
	}
}

func WithStatsMetric() Option {
	return func(cfg *config) {
		cfg.collectStats = true
	}
}

// WithJSONLoggerWriter if set, the metric will become a logger metric
func WithJSONLoggerWriter(logger JSONLogger) Option {
	return func(cfg *config) {
		cfg.writer = newJSONLoggerWriter(logger)
	}
}

func WithOpenobserveWriter(logger ErrorLogger, opts ...PromOption) Option {
	return func(cfg *config) {
		cfg.writer = newOpenobserveWriter(logger, opts...)
		SetMetricMode("prometheus")
	}
}

// WithOTLPWriter creates a new OTLP writer for OpenTelemetry metrics
func WithOTLPWriter(logger ErrorLogger, opts ...OTLPOption) Option {
	return func(c *config) {
		c.writer = newOTLPWriter(logger, opts...)
		SetMetricMode("otel")
	}
}
