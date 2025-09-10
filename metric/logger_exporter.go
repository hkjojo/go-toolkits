package metric

import (
	"context"
	"encoding/json"

	dto "github.com/prometheus/client_model/go"
)

// jsonLoggerExporter 实现JSON日志导出器
type jsonLoggerExporter struct {
	logger Logger
}

// Export 导出metric到JSON日志
func (w *jsonLoggerExporter) Export(mf *dto.MetricFamily) {
	bs, err := json.Marshal(mf)
	if err != nil {
		w.OnError(err)
		return
	}
	w.logger.Infow("metric_collected", "data", json.RawMessage(bs))
}

// OnError 处理错误
func (w *jsonLoggerExporter) OnError(err error) {
	w.logger.Infow("metric_internal_error", "error", err)
}

// Shutdown 优雅关闭（JSON日志导出器无需特殊关闭操作）
func (w *jsonLoggerExporter) Shutdown(ctx context.Context) error {
	return nil
}

// IsStart 是否启用
func (w *jsonLoggerExporter) IsStart() bool {
	return true
}

// newJSONLoggerExporter 创建JSON日志导出器
func newJSONLoggerExporter(logger Logger) Exporter {
	return &jsonLoggerExporter{logger: logger}
}
