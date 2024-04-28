package metric

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/prompb"
)

const (
	LabelName = "__name__"
)

type ErrorLogger interface {
	Errorw(string, ...interface{})
}

type httpWriter struct {
	init     bool
	endpoint string
	auth     string
	stream   string

	header map[string]string
	client *http.Client
	logger ErrorLogger
}

func newHTTPWriter(endpoint, auth, stream string, logger ErrorLogger) Writer {
	if endpoint == "" {
		logger.Errorw("metric_internal_error", "error", "endpoint empty")
		return &httpWriter{}
	}

	return &httpWriter{
		init:     true,
		endpoint: endpoint,
		stream:   stream,
		header: map[string]string{
			"Authorization": auth,
		},
		client: &http.Client{Timeout: time.Second * 10},
		logger: logger,
	}
}

func (w *httpWriter) Write(mf *dto.MetricFamily) {
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

	req, err = http.NewRequest(http.MethodPost, w.endpoint, bytes.NewBuffer(snappy.Encode(nil, pbBytes)))
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

func (w *httpWriter) OnError(err error) {
	w.logger.Errorw("metric_internal_error", "error", err)
}

var metricNameRE = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

func convertOne(mf *dto.MetricFamily) (prompb.TimeSeries, error) {
	metrics := mf.GetMetric()

	if !metricNameRE.MatchString(mf.GetName()) {
		return prompb.TimeSeries{}, errors.New("invalid metrics name")
	}
	if metrics == nil {
		return prompb.TimeSeries{}, errors.New("metrics empty")
	}

	var (
		lbs     []prompb.Label
		samples []prompb.Sample
	)
	lbs = append(lbs, prompb.Label{Name: LabelName, Value: mf.GetName()})

	for _, metric := range metrics {
		for _, lb := range metric.GetLabel() {
			if lb == nil {
				continue
			}
			lbs = append(lbs, prompb.Label{Name: lb.GetName(), Value: lb.GetValue()})
		}

		var value float64
		g := metric.GetGauge()
		if g != nil {
			value = g.GetValue()
		}

		c := metric.GetCounter()
		if c != nil {
			value = c.GetValue()
		}

		samples = append(samples, prompb.Sample{
			Value:     value,
			Timestamp: time.Unix(time.Now().Unix(), 0).UnixNano() / 1e6,
		})
	}

	return prompb.TimeSeries{
		Labels:  lbs,
		Samples: samples,
	}, nil
}

func toPrometheusPbWriteRequest(mf *dto.MetricFamily) (*prompb.WriteRequest, error) {
	ts, err := convertOne(mf)
	if err != nil {
		return nil, err
	}
	return &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{ts},
	}, nil
}
