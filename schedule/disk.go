package schedule

import (
	"fmt"
	"time"

	logtos "github.com/hkjojo/go-toolkits/log/v2/kratos"
	"github.com/shirou/gopsutil/v3/disk"
)

type DiskMonitor struct {
	path          string
	prevTime      time.Time
	prevDiskStats map[string]disk.IOCountersStat
}

func NewDiskMonitor() *DiskMonitor {
	return &DiskMonitor{
		path:          "./",
		prevDiskStats: make(map[string]disk.IOCountersStat),
	}
}

func (m *DiskMonitor) collectDiskStats(log *logtos.ActsHelper) error {
	currentTime := time.Now()
	deltaSeconds := currentTime.Sub(m.prevTime).Seconds()
	defer func() {
		m.prevTime = currentTime
	}()

	usage, err := disk.Usage(m.path)
	if err != nil {
		return err
	}

	diskIO, _ := disk.IOCounters()
	ioStats := make(map[string]uint64)
	if deltaSeconds > 0 {
		for name, current := range diskIO {
			if prev, exists := m.prevDiskStats[name]; exists {
				ioStats[name+".read"] = current.ReadBytes - prev.ReadBytes
				ioStats[name+".write"] = current.WriteBytes - prev.WriteBytes
			}
			m.prevDiskStats[name] = current
		}
		for name, stat := range ioStats {
			log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("%s: %s, %s", name, formatByteSpeed(stat, deltaSeconds), formatBytes(stat)))
		}
	}

	log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("disk_usage: %.2f%%, total: %s, free: %s",
		usage.UsedPercent, formatBytes(usage.Total), formatBytes(usage.Free)))

	return nil
}
