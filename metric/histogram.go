package metric

import (
	"github.com/prometheus/client_golang/prometheus"
)

var _ Observer = (*histogram)(nil)

type histogram struct {
	hv  *prometheus.HistogramVec
	lvs []string
}

// NewHistogram new a prometheus histogram and returns Histogram.
func NewHistogram(hv *prometheus.HistogramVec) Observer {
	register.MustRegister(hv)
	return &histogram{
		hv: hv,
	}
}

func (h *histogram) With(lvs ...string) Observer {
	return &histogram{
		hv:  h.hv,
		lvs: lvs,
	}
}

func (h *histogram) Observe(value float64) {
	h.hv.WithLabelValues(h.lvs...).Observe(value)
}
