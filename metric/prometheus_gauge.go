package metric

import (
	"github.com/prometheus/client_golang/prometheus"
)

var _ Gauge = (*prometheusGauge)(nil)

// prometheusGauge 实现Prometheus仪表盘
type prometheusGauge struct {
	gv  *prometheus.GaugeVec
	lvs []string
}

// newPrometheusGauge 创建Prometheus仪表盘
func newPrometheusGauge(gv *prometheus.GaugeVec) Gauge {
	return &prometheusGauge{
		gv: gv,
	}
}

func (g *prometheusGauge) With(lvs ...string) Gauge {
	return &prometheusGauge{
		gv:  g.gv,
		lvs: lvs,
	}
}

func (g *prometheusGauge) Delete(lvs ...string) bool {
	return g.gv.DeleteLabelValues(lvs...)
}

func (g *prometheusGauge) Set(value float64) {
	g.gv.WithLabelValues(g.lvs...).Set(value)
}

func (g *prometheusGauge) Add(delta float64) {
	g.gv.WithLabelValues(g.lvs...).Add(delta)
}

func (g *prometheusGauge) Sub(delta float64) {
	g.gv.WithLabelValues(g.lvs...).Sub(delta)
}
