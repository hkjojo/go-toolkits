package apptools

import (
	"os"
	"runtime"
	"strings"

	"github.com/grafana/pyroscope-go"
)

func NewPyroscope(ops ...PyroscopeOption) (*pyroscope.Profiler, error) {
	opts := newPyroscopeOptions(ops...)

	// address 优先级：显式 Option > env(PYROSCOPE_ADHOC_SERVER_ADDRESS)。
	// 两者都为空时保持原有 no-op：不启动 profiler、零上报、零连接。
	addr := ResolveEndpoint(opts.address, "PYROSCOPE_ADHOC_SERVER_ADDRESS")
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

	appName := Name
	if Env != "" {
		appName = Env + "." + Name
	}
	start, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: appName,
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
