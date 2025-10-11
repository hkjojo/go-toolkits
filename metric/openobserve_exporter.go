package metric

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/prompb"
)

const (
	streamLabel       = "__name__"
	metricLabel       = "metric_name"
	serverLabel       = "server_name"
	defaultStreamName = "go"
)

var (
	// metricNameRE 用于验证Prometheus指标名称的正则表达式
	// 规则：必须以字母、下划线或冒号开头，后续字符可以是字母、数字、下划线或冒号
	// 符合Prometheus指标命名规范
	metricNameRE = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
)

// openobserveExporter 实现OpenObserve导出器
type openobserveExporter struct {
	init   bool
	header map[string]string
	client *http.Client
	logger Logger
}

// newOpenobserveExporter 创建OpenObserve导出器
func newOpenobserveExporter(logger Logger) Exporter {
	exporter := &openobserveExporter{
		client: &http.Client{Timeout: time.Second * 30},
		logger: logger,
	}

	if globalConfig.StreamName == "" {
		globalConfig.StreamName = defaultStreamName
	}

	header, ok := os.LookupEnv("METRIC_HEADERS")
	if ok {
		hm := make(map[string]string)
		for _, headers := range strings.Split(header, ",") {
			kv := strings.SplitN(headers, "=", 2)
			if len(kv) != 2 {
				continue
			}
			hm[kv[0]] = kv[1]
		}
		exporter.header = hm
	}

	if globalConfig.Endpoint != "" {
		exporter.init = true
		logger.Infow("openobserve exporter initialized", "endpoint", globalConfig.Endpoint)
	} else {
		logger.Warnw("openobserve exporter not initialized", "err", "endpoint empty")
	}
	return exporter
}

// Export 导出metric到OpenObserve
func (w *openobserveExporter) Export(mf *dto.MetricFamily) {
	if !w.init {
		return
	}

	var (
		err     error
		pbBytes []byte
		req     *http.Request
		resp    *http.Response
		pbData  *prompb.WriteRequest
	)
	defer func() {
		if err != nil {
			w.OnError(err)
		}
	}()

	pbData, err = w.toPrometheusPbWriteRequest(mf)
	if err != nil {
		return
	}
	pbBytes, err = proto.Marshal(pbData)
	if err != nil {
		return
	}

	req, err = http.NewRequest(http.MethodPost, globalConfig.Endpoint, bytes.NewBuffer(snappy.Encode(nil, pbBytes)))
	if err != nil {
		return
	}

	req.Header.Add("X-Prometheus-Remote-Write-Version", "0.1.0")
	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	for k, v := range w.header {
		req.Header.Add(k, v)
	}

	resp, err = w.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status code: %d", resp.StatusCode)
		return
	}
}

// OnError 处理错误
func (w *openobserveExporter) OnError(err error) {
	w.logger.Errorw("metric_internal_error", "error", err)
}

// Shutdown 优雅关闭（OpenObserve导出器无需特殊关闭操作）
func (w *openobserveExporter) Shutdown(ctx context.Context) error {
	return nil
}

// IsStart 是否启用
func (w *openobserveExporter) IsStart() bool {
	return w.init
}

func (w *openobserveExporter) convert(mf *dto.MetricFamily) ([]prompb.TimeSeries, error) {
	if !metricNameRE.MatchString(mf.GetName()) {
		return nil, errors.New("invalid metrics name")
	}
	if mf.Type == nil {
		return nil, errors.New("invalid metrics type")
	}

	var metrics = mf.GetMetric()
	if metrics == nil {
		return nil, errors.New("metrics empty")
	}

	var (
		defaultLbs []prompb.Label
		timeseries []prompb.TimeSeries
	)
	// reserved label name
	defaultLbs = append(defaultLbs,
		prompb.Label{Name: streamLabel, Value: globalConfig.StreamName},
		prompb.Label{Name: serverLabel, Value: globalConfig.ServiceName},
		prompb.Label{Name: metricLabel, Value: mf.GetName()},
	)

	for _, metric := range metrics {
		var (
			lbs        []prompb.Label
			samples    []prompb.Sample
			histograms []prompb.Histogram
		)
		for _, lb := range metric.GetLabel() {
			if lb == nil {
				continue
			}
			lbs = append(lbs, prompb.Label{Name: lb.GetName(), Value: lb.GetValue()})
		}

		var ts = time.Now().UnixNano() / 1e6
		if metric.GetTimestampMs() != 0 {
			ts = metric.GetTimestampMs()
		}

		switch mf.GetType() {
		case dto.MetricType_COUNTER:
			samples = append(samples, prompb.Sample{Value: metric.GetCounter().GetValue(), Timestamp: ts})
		case dto.MetricType_GAUGE:
			samples = append(samples, prompb.Sample{Value: metric.GetGauge().GetValue(), Timestamp: ts})
		case dto.MetricType_SUMMARY:
		case dto.MetricType_HISTOGRAM:
			hist := metric.GetHistogram()
			if hist == nil {
				continue
			}
			pbHist := prompb.Histogram{
				Sum:            hist.GetSampleSum(),
				Schema:         hist.GetSchema(),
				ZeroThreshold:  hist.GetZeroThreshold(),
				NegativeSpans:  spansToSpansProto(hist.GetNegativeSpan()),
				NegativeDeltas: hist.GetNegativeDelta(),
				NegativeCounts: hist.GetNegativeCount(),
				PositiveSpans:  spansToSpansProto(hist.GetPositiveSpan()),
				PositiveDeltas: hist.GetPositiveDelta(),
				PositiveCounts: hist.GetPositiveCount(),
				ResetHint:      prompb.Histogram_YES,
				Timestamp:      hist.GetCreatedTimestamp().GetSeconds() * 1e3,
			}

			if hist.SampleCount != nil {
				pbHist.Count = &prompb.Histogram_CountInt{CountInt: hist.GetSampleCount()}
			} else {
				pbHist.Count = &prompb.Histogram_CountFloat{CountFloat: hist.GetSampleCountFloat()}
			}

			if hist.ZeroCount != nil {
				pbHist.ZeroCount = &prompb.Histogram_ZeroCountInt{ZeroCountInt: hist.GetZeroCount()}
			} else {
				pbHist.ZeroCount = &prompb.Histogram_ZeroCountFloat{ZeroCountFloat: hist.GetZeroCountFloat()}
			}
			histograms = append(histograms, pbHist)
		}
		lbs = append(lbs, defaultLbs...)
		timeseries = append(timeseries, prompb.TimeSeries{
			Labels:     lbs,
			Samples:    samples,
			Histograms: histograms,
		})
	}

	return timeseries, nil
}

func (w *openobserveExporter) toPrometheusPbWriteRequest(mf *dto.MetricFamily) (*prompb.WriteRequest, error) {
	ts, err := w.convert(mf)
	if err != nil {
		return nil, err
	}
	metadata := getMetadata(mf)
	metadata.MetricFamilyName = globalConfig.StreamName
	return &prompb.WriteRequest{
		Timeseries: ts,
		Metadata:   []prompb.MetricMetadata{metadata},
	}, nil
}

func getMetadata(mf *dto.MetricFamily) prompb.MetricMetadata {
	return prompb.MetricMetadata{
		Type:             toMetricType(mf.GetType()),
		MetricFamilyName: mf.GetName(),
		Help:             mf.GetHelp(),
		Unit:             mf.GetUnit(),
	}
}

func toMetricType(metricType dto.MetricType) prompb.MetricMetadata_MetricType {
	switch metricType {
	case dto.MetricType_COUNTER:
		return prompb.MetricMetadata_COUNTER
	case dto.MetricType_GAUGE:
		return prompb.MetricMetadata_GAUGE
	case dto.MetricType_SUMMARY:
		return prompb.MetricMetadata_SUMMARY
	case dto.MetricType_HISTOGRAM:
		return prompb.MetricMetadata_HISTOGRAM
	case dto.MetricType_GAUGE_HISTOGRAM:
		return prompb.MetricMetadata_GAUGEHISTOGRAM
	}
	return prompb.MetricMetadata_UNKNOWN
}

func spansToSpansProto(s []*dto.BucketSpan) []prompb.BucketSpan {
	spans := make([]prompb.BucketSpan, len(s))
	for i := 0; i < len(s); i++ {
		spans[i] = prompb.BucketSpan{Offset: s[i].GetOffset(), Length: s[i].GetLength()}
	}
	return spans
}
