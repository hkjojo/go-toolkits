package apptools

import (
	"github.com/grafana/pyroscope-go"
	"os"
	"runtime"
	"strings"
)

func NewPyroscope() (*pyroscope.Profiler, error) {
	addr := os.Getenv("PYROSCOPE_ADHOC_SERVER_ADDRESS")
	if addr == "" {
		return nil, nil
	}

	// 基础 profiles（始终开启）
	profileTypes := []pyroscope.ProfileType{
		pyroscope.ProfileCPU,
		pyroscope.ProfileAllocObjects,
		pyroscope.ProfileAllocSpace,
		pyroscope.ProfileInuseObjects,
		pyroscope.ProfileInuseSpace,
	}

	// 可选 profiles（通过环境变量控制）
	optional := parseOptionalProfiles(os.Getenv("PYROSCOPE_OPTIONAL_PROFILES"))
	profileTypes = append(profileTypes, optional...)

	start, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: Name,
		ServerAddress:   addr,
		Logger:          pyroscope.StandardLogger,
		Tags:            map[string]string{"env": Env},
		ProfileTypes:    profileTypes,
	})
	if err != nil {
		return nil, err
	}

	return start, nil
}

func parseOptionalProfiles(val string) []pyroscope.ProfileType {
	if val == "" {
		return nil
	}

	val = strings.ToLower(strings.TrimSpace(val))

	// 全开
	if val == "all" {
		enableMutexAndBlock()
		return []pyroscope.ProfileType{
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		}
	}

	var profiles []pyroscope.ProfileType
	items := strings.Split(val, ",")

	for _, item := range items {
		switch strings.TrimSpace(item) {
		case "goroutines":
			profiles = append(profiles, pyroscope.ProfileGoroutines)

		case "mutex":
			enableMutexAndBlock()
			profiles = append(profiles,
				pyroscope.ProfileMutexCount,
				pyroscope.ProfileMutexDuration,
			)

		case "block":
			enableMutexAndBlock()
			profiles = append(profiles,
				pyroscope.ProfileBlockCount,
				pyroscope.ProfileBlockDuration,
			)
		}
	}

	return profiles
}

func enableMutexAndBlock() {
	// 仅在需要时开启，避免全局开销
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)
}
