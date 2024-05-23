package metric

import (
	"time"
)

type config struct {
	interval  time.Duration
	withoutUp bool
	writer    Writer
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

// WithJSONLoggerWriter if set, the metric will become a logger metric
func WithJSONLoggerWriter(logger JSONLogger) Option {
	return func(cfg *config) {
		cfg.writer = newJSONLoggerWriter(logger)
	}
}

func WithPromRemoteWriter(logger ErrorLogger, opts ...PromOption) Option {
	return func(cfg *config) {
		cfg.writer = newPromRemoteWriter(logger, opts...)
	}
}
