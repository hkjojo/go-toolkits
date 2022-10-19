package metric

import (
	"encoding/json"
	"fmt"
	"testing"

	"math/rand"
	"time"

	// "github.com/go-kratos/kratos/v2/middleware/metrics"

	prom "github.com/go-kratos/kratos/contrib/metrics/prometheus/v2"
	"github.com/prometheus/client_golang/prometheus"

	dto "github.com/prometheus/client_model/go"

	// "github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
)

var (
	_metricSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "server",
		Subsystem: "requests",
		Name:      "duration_sec",
		Help:      "server requests duration(sec).",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.250, 0.5, 1},
	}, []string{"kind", "operation"})

	_metricRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "client",
		Subsystem: "requests",
		Name:      "code_total",
		Help:      "The total number of processed requests",
	}, []string{"kind", "operation", "code", "reason"})
)

func TestPushCounter(t *testing.T) {
	pusher := push.New("http://localhost:9091/", "test_job")
	pc := prom.NewCounter(_metricRequests)
	pusher.Collector(_metricRequests)
	ticker := time.NewTicker(5 * time.Second)
	for {
		<-ticker.C

		var mc = make(chan prometheus.Metric, 1024)
		pc.With(fmt.Sprintf("%d", rand.Intn(200))).Add(float64(rand.Intn(10)))
		_metricRequests.Collect(mc)
		close(mc)
		for m := range mc {
			var dm dto.Metric
			m.Write(&dm)
			bs, _ := json.Marshal(dm)
			t.Logf("%s", string(bs))
		}

		t.Log("before push")
		err := pusher.Push()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestPushHistogrm(t *testing.T) {
	pusher := push.New("http://localhost:9091/", "test_job")
	pc := prom.NewHistogram(_metricSeconds)
	pusher.Collector(_metricSeconds)
	ticker := time.NewTicker(5 * time.Second)
	for {
		<-ticker.C

		var mc = make(chan prometheus.Metric, 1024)
		pc.With("/api/test").Observe(float64(rand.Intn(30)))
		_metricSeconds.Collect(mc)
		close(mc)
		for m := range mc {
			var dm dto.Metric
			m.Write(&dm)
			bs, _ := json.Marshal(dm)
			t.Logf("%s", string(bs))
		}

		t.Log("before push")
		err := pusher.Push()
		if err != nil {
			t.Fatal(err)
		}
	}
}
