package metric

import (
	"reflect"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestRequestSecondsHistogramView(t *testing.T) {
	stream, ok := RequestSecondsHistogramView()(sdkmetric.Instrument{Name: "td_server_requests_seconds"})
	if !ok {
		t.Fatal("expected request seconds view to match")
	}

	agg, ok := stream.Aggregation.(sdkmetric.AggregationExplicitBucketHistogram)
	if !ok {
		t.Fatalf("aggregation = %T, want AggregationExplicitBucketHistogram", stream.Aggregation)
	}

	want := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	if !reflect.DeepEqual(agg.Boundaries, want) {
		t.Fatalf("boundaries = %v, want %v", agg.Boundaries, want)
	}

	if _, ok := RequestSecondsHistogramView()(sdkmetric.Instrument{Name: "td_transaction_duration_seconds"}); ok {
		t.Fatal("request seconds view should not match non-request histogram")
	}
}

func TestWithViews(t *testing.T) {
	cfg := &Config{}

	// 单次调用接收变长参数：两个 view 都应入列。
	WithViews(RequestSecondsHistogramView(), RequestSecondsHistogramView())(cfg)
	if len(cfg.Views) != 2 {
		t.Fatalf("after first WithViews: len = %d, want 2", len(cfg.Views))
	}

	// 重复调用应累加而非覆盖（防回归：误改成 cfg.Views = views 会让先注册的视图被丢弃）。
	WithViews(RequestSecondsHistogramView())(cfg)
	if len(cfg.Views) != 3 {
		t.Fatalf("after second WithViews: len = %d, want 3 (append, not replace)", len(cfg.Views))
	}
}
