package metric

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	// Global meter instance
	globalMeter metric.Meter
)

// getMeter returns the global meter instance
func getMeter() metric.Meter {
	if globalMeter == nil {
		// Use the global meter provider if no custom one is set
		globalMeter = otel.Meter("go-toolkits/metric")
	}
	return globalMeter
}
