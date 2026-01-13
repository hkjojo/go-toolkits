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

	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	start, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: Name,
		ServerAddress:   addr,
		Logger:          pyroscope.StandardLogger,
		Tags:            map[string]string{"env": Env},

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
