package metric

import (
	"github.com/prometheus/client_golang/prometheus"
)

var _ Observer = (*prometheusHistogram)(nil)

// prometheusHistogram 实现Prometheus直方图
type prometheusHistogram struct {
	hv  *prometheus.HistogramVec
	lvs []string
}

// NewPrometheusHistogram 创建Prometheus直方图
func NewPrometheusHistogram(hv *prometheus.HistogramVec) Observer {
	return &prometheusHistogram{
		hv: hv,
	}
}

func (h *prometheusHistogram) With(lvs ...string) Observer {
	return &prometheusHistogram{
		hv:  h.hv,
		lvs: lvs,
	}
}

func (h *prometheusHistogram) Observe(value float64) {
	h.hv.WithLabelValues(h.lvs...).Observe(value)
}
