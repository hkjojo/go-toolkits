package metric

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/prompb"
)

const (
	labelName  = "__name__"
	serverName = "server_name"
)

var (
	metricNameRE  = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
	defaultWriter *promRemoteWriter
)

type ErrorLogger interface {
	Errorw(string, ...interface{})
	Debugw(string, ...interface{})
}

type PromOption func(*promRemoteWriter)

type promRemoteWriter struct {
	init          bool
	url           string
	endpoint      string
	organization  string
	authorization string
	insecure      bool
	header        map[string]string
	client        *http.Client
	logger        ErrorLogger
}

// WithHTTPClient ...
func WithHTTPClient(client *http.Client) PromOption {
	return func(writer *promRemoteWriter) {
		writer.client = client
	}
}

// WithEndpoint ...
func WithEndpoint(endpoint string) PromOption {
	return func(writer *promRemoteWriter) {
		writer.endpoint = endpoint
	}
}

// WithAuthorization ...
func WithAuthorization(authorization string) PromOption {
	return func(writer *promRemoteWriter) {
		writer.authorization = authorization
	}
}

// WithOrganization default use os env: OTLP_ORGANIZATION
func WithOrganization(organization string) PromOption {
	return func(writer *promRemoteWriter) {
		writer.organization = organization
	}
}

// WithInsecure ...
func WithInsecure(insecure bool) PromOption {
	return func(writer *promRemoteWriter) {
		writer.insecure = insecure
	}
}

func initPromConfig(logger ErrorLogger) {
	var (
		err      error
		insecure bool
	)

	insecureEnv, ok := os.LookupEnv("METRIC_INSECURE")
	if ok {
		insecure, err = strconv.ParseBool(insecureEnv)
		if err != nil {
			logger.Errorw("metric_internal_error", "error", err)
		}
	}

	defaultWriter = &promRemoteWriter{
		endpoint:      os.Getenv("METRIC_ENDPOINT"),
		organization:  os.Getenv("METRIC_ORGANIZATION"),
		authorization: os.Getenv("METRIC_AUTHORIZATION"),
		insecure:      insecure,
		client:        &http.Client{Timeout: time.Second * 30},
		logger:        logger,
	}
}

func (w *promRemoteWriter) initUrlAndHeader() {
	var scheme = "https"
	if w.insecure {
		scheme = "http"
	}
	w.url = fmt.Sprintf("%s://%s/api/%s/prometheus/api/v1/write", scheme, w.endpoint, w.organization)
	w.header = map[string]string{
		"Authorization": w.authorization,
	}

	w.logger.Debugw("initUrlAndHeader", "writer", fmt.Sprintf("%+v", w))
}

func newPromRemoteWriter(logger ErrorLogger, opts ...PromOption) Writer {
	initPromConfig(logger)

	for _, option := range opts {
		option(defaultWriter)
	}

	if defaultWriter.endpoint != "" {
		defaultWriter.init = true
	} else {
		logger.Errorw("metric_internal_error", "error", "endpoint empty")
	}

	defaultWriter.initUrlAndHeader()
	return defaultWriter
}

func (w *promRemoteWriter) Write(mf *dto.MetricFamily) {
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

	pbData, err = toPrometheusPbWriteRequest(mf)
	if err != nil {
		return
	}
	pbBytes, err = proto.Marshal(pbData)
	if err != nil {
		return
	}

	req, err = http.NewRequest(http.MethodPost, w.url, bytes.NewBuffer(snappy.Encode(nil, pbBytes)))
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

func (w *promRemoteWriter) OnError(err error) {
	w.logger.Errorw("metric_internal_error", "error", err)
}

func convertOne(mf *dto.MetricFamily) (prompb.TimeSeries, prompb.MetricMetadata, error) {
	if !metricNameRE.MatchString(mf.GetName()) {
		return prompb.TimeSeries{}, prompb.MetricMetadata{}, errors.New("invalid metrics name")
	}
	if mf.Type == nil {
		return prompb.TimeSeries{}, prompb.MetricMetadata{}, errors.New("invalid metrics type")
	}

	var metrics = mf.GetMetric()
	if metrics == nil {
		return prompb.TimeSeries{}, prompb.MetricMetadata{}, errors.New("metrics empty")
	}

	var (
		lbs        []prompb.Label
		samples    []prompb.Sample
		histograms []prompb.Histogram
	)
	// reserved label name
	lbs = append(lbs, prompb.Label{Name: labelName, Value: mf.GetName()})
	lbs = append(lbs, prompb.Label{Name: serverName, Value: os.Getenv("SERVICE_NAME")})

	for _, metric := range metrics {
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
	}

	metadata := prompb.MetricMetadata{
		Type:             toMetricType(mf.GetType()),
		MetricFamilyName: mf.GetName(),
		Help:             mf.GetHelp(),
		Unit:             mf.GetUnit(),
	}

	return prompb.TimeSeries{
		Labels:     lbs,
		Samples:    samples,
		Histograms: histograms,
	}, metadata, nil
}

func toPrometheusPbWriteRequest(mf *dto.MetricFamily) (*prompb.WriteRequest, error) {
	ts, metadata, err := convertOne(mf)
	if err != nil {
		return nil, err
	}
	return &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{ts},
		Metadata:   []prompb.MetricMetadata{metadata},
	}, nil
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

func spansToSpansProto(s []*dto.BucketSpan) []*prompb.BucketSpan {
	spans := make([]*prompb.BucketSpan, len(s))
	for i := 0; i < len(s); i++ {
		spans[i] = &prompb.BucketSpan{Offset: s[i].GetOffset(), Length: s[i].GetLength()}
	}
	return spans
}
