package metric

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	// Global meter instance
	globalMeter metric.Meter
	meterMutex  sync.RWMutex

	// Global context for metrics
	globalContext = context.Background()
)

// SetMeterProvider sets the global meter provider for OpenTelemetry metrics
func SetMeterProvider(mp metric.MeterProvider) {
	meterMutex.Lock()
	defer meterMutex.Unlock()
	globalMeter = mp.Meter("go-toolkits/metric")
}

// getMeter returns the global meter instance
func getMeter() metric.Meter {
	meterMutex.RLock()
	defer meterMutex.RUnlock()
	if globalMeter == nil {
		// Use the global meter provider if no custom one is set
		globalMeter = otel.Meter("go-toolkits/metric")
	}
	return globalMeter
}

// getContext returns the context for metric operations
func getContext() context.Context {
	return globalContext
}

// SetContext sets the global context for metric operations
func SetContext(ctx context.Context) {
	globalContext = ctx
}