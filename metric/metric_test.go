package metric

import (
	"math/rand"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
	pusher := push.
		New("http://localhost:9091/", "test_job").
		Collector(_metricRequests)

	ticker := time.NewTicker(5 * time.Second)
	for {
		<-ticker.C

		_metricRequests.
			WithLabelValues("kind-val", "operation-val", "code-val", "reason-val").
			Inc()

		t.Log("before push")
		err := pusher.Push()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestPushHistogrm(t *testing.T) {
	pusher := push.
		New("http://localhost:9091/", "test_job").
		Collector(_metricSeconds)

	ticker := time.NewTicker(5 * time.Second)
	for {
		<-ticker.C

		_metricSeconds.
			WithLabelValues("http", "/api/test/").
			Observe(float64(rand.Intn(30)))

		t.Log("before push")
		err := pusher.Push()
		if err != nil {
			t.Fatal(err)
		}
	}
}
