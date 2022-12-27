package apptools

import (
	"gopkg.in/DataDog/dd-trace-go.v1/profiler"
)

// NewDatadog can use configuration or environment variables(DD_AGENT_HOST,DD_ENV)
func NewDatadog(endpoint string) (func(), error) {
	opts := []profiler.Option{
		profiler.WithEnv(Env),
		profiler.WithService(Name),
		profiler.WithVersion(Version),
		profiler.WithProfileTypes(
			profiler.CPUProfile,
			profiler.HeapProfile,
			profiler.BlockProfile,
			profiler.MutexProfile,
			profiler.GoroutineProfile,
		),
	}
	if endpoint != "" {
		opts = append(opts, profiler.WithAgentAddr(endpoint))
	}

	return func() {
		profiler.Stop()
	}, profiler.Start(opts...)
}
