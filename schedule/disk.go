package schedule

import (
	"fmt"
	"time"

	logtos "github.com/hkjojo/go-toolkits/log/v2/kratos"
	"github.com/shirou/gopsutil/v3/disk"
)

type DiskMonitor struct {
	path          []string
	prevTime      time.Time
	prevDiskStats map[string]disk.IOCountersStat
	used          uint64
	total         uint64
	read          uint64
	write         uint64
}

func NewDiskMonitor(paths []string) *DiskMonitor {
	return &DiskMonitor{
		path:          paths,
		prevDiskStats: make(map[string]disk.IOCountersStat),
	}
}

func (m *DiskMonitor) collectDiskStats(log *logtos.ActsHelper) error {
	currentTime := time.Now()
	deltaSeconds := currentTime.Sub(m.prevTime).Seconds()
	defer func() {
		m.prevTime = currentTime
	}()

	var diskUsed, diskTotal uint64
	for _, path := range m.path {
		usage, err := disk.Usage(path)
		if err != nil {
			log.Errorw(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("get disk usage(%s) failed, %s", path, err))
			continue
		}
		log.Infow(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("[server] disk_usage(%s): %.2f%%, used: %s, "+
			"total: %s", path, float64(usage.Used*100)/float64(usage.Total), formatBytes(usage.Used), formatBytes(usage.Total)))
		diskUsed += usage.Used
		diskTotal += usage.Total
	}
	m.used = diskUsed
	m.total = diskTotal

	diskIO, _ := disk.IOCounters()
	var ioRead, ioWrite uint64
	if deltaSeconds > 0 {
		for name, current := range diskIO {
			if prev, exists := m.prevDiskStats[name]; exists {
				ioRead += current.ReadBytes - prev.ReadBytes
				ioWrite += current.WriteBytes - prev.WriteBytes
			}
			m.prevDiskStats[name] = current
		}
		m.read = ioRead
		m.write = ioWrite
		log.Infow(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("[server] disk_read: %s, %s, disk_write: %s, %s",
			formatByteSpeed(ioRead, deltaSeconds), formatBytes(ioRead), formatByteSpeed(ioWrite, deltaSeconds), formatBytes(ioWrite)))
	}

	return nil
}
