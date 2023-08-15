package metric

import (
	"github.com/prometheus/client_golang/prometheus"
)

var _ Gauge = (*gauge)(nil)

type gauge struct {
	gv  *prometheus.GaugeVec
	lvs []string
}

// NewGauge new a prometheus gauge and returns Gauge.
func NewGauge(gv *prometheus.GaugeVec) Gauge {
	register.Register(gv)
	return &gauge{
		gv: gv,
	}
}

func (g *gauge) With(lvs ...string) Gauge {
	return &gauge{
		gv:  g.gv,
		lvs: lvs,
	}
}

func (g *gauge) Delete(lvs ...string) bool {
	return g.gv.DeleteLabelValues(lvs...)
}

func (g *gauge) Set(value float64) {
	g.gv.WithLabelValues(g.lvs...).Set(value)
}

func (g *gauge) Add(delta float64) {
	g.gv.WithLabelValues(g.lvs...).Add(delta)
}

func (g *gauge) Sub(delta float64) {
	g.gv.WithLabelValues(g.lvs...).Sub(delta)
}
