package metric

import (
	"github.com/prometheus/client_golang/prometheus"
)

var _ Observer = (*prometheusSummary)(nil)

// prometheusSummary 实现Prometheus摘要
type prometheusSummary struct {
	sv  *prometheus.SummaryVec
	lvs []string
}

// NewPrometheusSummary 创建Prometheus摘要
func NewPrometheusSummary(sv *prometheus.SummaryVec) Observer {
	return &prometheusSummary{
		sv: sv,
	}
}

func (s *prometheusSummary) With(lvs ...string) Observer {
	return &prometheusSummary{
		sv:  s.sv,
		lvs: lvs,
	}
}

func (s *prometheusSummary) Observe(value float64) {
	s.sv.WithLabelValues(s.lvs...).Observe(value)
}
