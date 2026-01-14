package apptools

import (
	"github.com/grafana/pyroscope-go"
	"os"
	"runtime"
)

func NewPyroscope() (*pyroscope.Profiler, error) {
	var addr string
	if addr = os.Getenv("PYROSCOPE_ADHOC_SERVER_ADDRESS"); addr == "" {
		return nil, nil
	}

	// These 2 lines are only required if you're using mutex or block profiling
	// Read the explanation below for how to set these rates:
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	start, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: Name,
		ServerAddress:   addr,
		Logger:          pyroscope.StandardLogger,
		// you can provide static tags via a map:
		Tags: map[string]string{"env": Env},

		ProfileTypes: []pyroscope.ProfileType{
			// these profile types are enabled by default:
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,

			// these profile types are optional:
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
	if err != nil {
		return nil, err
	}
	return start, nil
}
