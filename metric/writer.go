package metric

import (
	"encoding/json"

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
