package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"

	logtos "github.com/hkjojo/go-toolkits/log/v2/kratos"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type SystemMonitorTask struct {
	prevNetStats  []net.IOCountersStat
	prevDiskStats map[string]disk.IOCountersStat
	mu            sync.Mutex
}

func NewSystemMonitorTask() *SystemMonitorTask {
	return &SystemMonitorTask{
		prevDiskStats: make(map[string]disk.IOCountersStat),
	}
}

func (t *SystemMonitorTask) Execute(ctx context.Context, log *logtos.ActsHelper) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 收集CPU指标
	cpuPercents, _ := cpu.Percent(1*time.Second, false)
	cpuUsage := 0.0
	if len(cpuPercents) > 0 {
		cpuUsage = cpuPercents[0]
	}
	log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("CPU usage: %.2f", cpuUsage))

	// 收集内存指标
	memInfo, _ := mem.VirtualMemory()
	memUsage := 0.0
	if memInfo != nil {
		memUsage = memInfo.UsedPercent
	}
	log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("Memory usage: %.2f", memUsage))

	// 收集磁盘指标
	diskUsage, _ := disk.Usage("./")
	diskUsed := 0.0
	if diskUsage != nil {
		diskUsed = diskUsage.UsedPercent
	}
	log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("Disk usage: %.2f", diskUsed))

	// 收集网络指标
	netStats, _ := net.IOCounters(true)
	var sent, recv uint64
	if t.prevNetStats != nil {
		for i, current := range netStats {
			if i < len(t.prevNetStats) {
				prev := t.prevNetStats[i]
				sent += current.BytesSent - prev.BytesSent
				recv += current.BytesRecv - prev.BytesRecv
			}
		}
	}
	t.prevNetStats = netStats
	log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("Net stats: sent: %d, recv: %d", sent, recv))

	// 收集磁盘IO指标
	diskIO, _ := disk.IOCounters()
	ioStats := make(map[string]uint64)
	for name, current := range diskIO {
		if prev, exists := t.prevDiskStats[name]; exists {
			ioStats[name+".read"] = current.ReadBytes - prev.ReadBytes
			ioStats[name+".write"] = current.WriteBytes - prev.WriteBytes
		}
		t.prevDiskStats[name] = current
	}
	for name, stat := range ioStats {
		log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("IO stats: %s: %d", name, stat))
	}

	return nil
}
