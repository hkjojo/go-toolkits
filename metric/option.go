package metric

import (
	"time"
)

// Config metric配置
type Config struct {
	Mode           Mode          // 采集模式
	Endpoint       string        // 导出端点
	Interval       time.Duration // 采集间隔
	WithoutUp      bool          // 是否跳过up指标
	CollectStats   bool          // 是否采集运行时统计
	ServiceName    string        // 服务名称
	ServiceVersion string        // 服务版本
	Env            string        // 环境
	StreamName     string        // 流名称（OpenObserve用）
}

// Option 配置选项函数
type Option func(*Config)

// WithMode 设置metric采集模式
func WithMode(mode Mode) Option {
	return func(cfg *Config) {
		cfg.Mode = mode
	}
}

// WithInterval 设置采集间隔
func WithInterval(d time.Duration) Option {
	return func(cfg *Config) {
		cfg.Interval = d
	}
}

// WithoutUpMetric 跳过up指标
func WithoutUpMetric() Option {
	return func(cfg *Config) {
		cfg.WithoutUp = true
	}
}

// WithStatsMetric 启用运行时统计指标
func WithStatsMetric() Option {
	return func(cfg *Config) {
		cfg.CollectStats = true
	}
}

// WithEndpoint 设置导出端点
func WithEndpoint(endpoint string) Option {
	return func(cfg *Config) {
		cfg.Endpoint = endpoint
	}
}

// WithServiceName 设置服务名称
func WithServiceName(serviceName string) Option {
	return func(cfg *Config) {
		cfg.ServiceName = serviceName
	}
}

// WithStreamName 设置流名称（OpenObserve用）
func WithStreamName(streamName string) Option {
	return func(cfg *Config) {
		cfg.StreamName = streamName
	}
}

// WithServiceVersion 设置服务版本
func WithServiceVersion(serviceVersion string) Option {
	return func(cfg *Config) {
		cfg.ServiceVersion = serviceVersion
	}
}

// WithEnv 设置环境
func WithEnv(env string) Option {
	return func(cfg *Config) {
		cfg.Env = env
	}
}
