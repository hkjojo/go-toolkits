package schedule

import (
	"fmt"

	logtos "github.com/hkjojo/go-toolkits/log/v2/kratos"
	"github.com/shirou/gopsutil/v3/disk"
)

type DiskMonitor struct {
	path          string
	prevDiskStats map[string]disk.IOCountersStat
}

func NewDiskMonitor() *DiskMonitor {
	return &DiskMonitor{
		path:          "./",
		prevDiskStats: make(map[string]disk.IOCountersStat),
	}
}

func (m *DiskMonitor) collectDiskStats(log *logtos.ActsHelper) error {
	usage, err := disk.Usage(m.path)
	if err != nil {
		return err
	}

	diskIO, _ := disk.IOCounters()
	ioStats := make(map[string]uint64)
	for name, current := range diskIO {
		if prev, exists := m.prevDiskStats[name]; exists {
			ioStats[name+".read"] = current.ReadBytes - prev.ReadBytes
			ioStats[name+".write"] = current.WriteBytes - prev.WriteBytes
		}
		m.prevDiskStats[name] = current
	}
	for name, stat := range ioStats {
		log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("disk_io, %s: %s", name, formatBytes(stat)))
	}

	log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("total: %s, free: %s, disk_usage: %.2f%",
		formatBytes(usage.Total), formatBytes(usage.Free), usage.UsedPercent))

	return nil
}
