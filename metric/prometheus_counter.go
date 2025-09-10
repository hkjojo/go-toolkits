package metric

import (
	"github.com/prometheus/client_golang/prometheus"
)

var _ Counter = (*prometheusCounter)(nil)

// prometheusCounter 实现Prometheus计数器
type prometheusCounter struct {
	cv  *prometheus.CounterVec
	lvs []string
}

// NewPrometheusCounter 创建Prometheus计数器
func NewPrometheusCounter(cv *prometheus.CounterVec) Counter {
	return &prometheusCounter{
		cv: cv,
	}
}

func (c *prometheusCounter) With(lvs ...string) Counter {
	return &prometheusCounter{
		cv:  c.cv,
		lvs: lvs,
	}
}

func (c *prometheusCounter) Inc() {
	c.cv.WithLabelValues(c.lvs...).Inc()
}

func (c *prometheusCounter) Add(delta float64) {
	c.cv.WithLabelValues(c.lvs...).Add(delta)
}
