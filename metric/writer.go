package metric

import (
	"bytes"
	"encoding/json"
	"net/http"

	dto "github.com/prometheus/client_model/go"
)

type Writer interface {
	Write(*dto.MetricFamily)
	OnError(error)
}

// JSONLogger for internal services with go-toolkits/log package
type JSONLogger interface {
	Infow(string, ...interface{})
}

type jsonLoggerWriter struct {
	logger JSONLogger
}

func (w *jsonLoggerWriter) Write(mf *dto.MetricFamily) {
	bs, err := json.Marshal(mf)
	if err != nil {
		w.OnError(err)
		return
	}
	w.logger.Infow("metric_collected", "data", json.RawMessage(bs))
}

func (w *jsonLoggerWriter) OnError(err error) {
	w.logger.Infow("metric_internal_error", "error", err)
}

func newJSONLoggerWriter(logger JSONLogger) Writer {
	return &jsonLoggerWriter{logger: logger}
}

type httpWriter struct {
	endpoint string
	auth     string
	stream   string
	client   *http.Client
}

func newHTTPWriter(endpoint, auth, stream string) Writer {
	if stream == "" {
		stream = "default"
	}
	return &httpWriter{
		endpoint: endpoint,
		auth:     auth,
		stream:   stream,
		client:   http.DefaultClient,
	}
}

func (w *httpWriter) Write(mf *dto.MetricFamily) {
	var (
		err  error
		bs   []byte
		req  *http.Request
		resp *http.Response
	)
	defer func() {
		if err != nil {
			w.OnError(err)
		}
	}()

	bs, err = json.Marshal(mf)
	if err != nil {
		return
	}
	req, err = http.NewRequest("POST", w.endpoint, bytes.NewReader(bs))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", w.auth)
	req.Header.Set("stream-name", w.stream)

	resp, err = w.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func (w *httpWriter) OnError(err error) {
	// todo
}
