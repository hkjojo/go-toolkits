package schedule

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	logtos "github.com/hkjojo/go-toolkits/log/v2/kratos"
)

const (
	_  = iota
	KB = 1 << (10 * iota)
	MB
	GB
	TB
)

func formatBytes(b uint64) string {
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB", float64(b)/TB)
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/GB)
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/MB)
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/KB)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatByteSpeed(b uint64, deltaSec float64) string {
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB/s", float64(b)/TB/deltaSec)
	case b >= GB:
		return fmt.Sprintf("%.2f GB/s", float64(b)/GB/deltaSec)
	case b >= MB:
		return fmt.Sprintf("%.2f MB/s", float64(b)/MB/deltaSec)
	case b >= KB:
		return fmt.Sprintf("%.2f KB/s", float64(b)/KB/deltaSec)
	default:
		return fmt.Sprintf("%.2f B/s", float64(b)/deltaSec)
	}
}

type MemoryMonitor struct {
}

func NewMemoryMonitor() *MemoryMonitor {
	return &MemoryMonitor{}
}

func (m *MemoryMonitor) getMemStats(log *logtos.ActsHelper) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	alloc := formatBytes(memStats.Alloc) //当前堆对象分配的内存
	sys := formatBytes(memStats.Sys)     //从操作系统获得的总内存
	// formatBytes(memStats.HeapAlloc)     堆上分配且仍在使用的内存

	log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("mem_usage, alloc: %s, sys: %s", alloc, sys))
}

func showContainerLimitComparison(used, limit string) {
	usedVal, _ := strconv.ParseFloat(strings.Split(used, " ")[0], 64)
	limitVal, _ := strconv.ParseFloat(strings.Split(limit, " ")[0], 64)

	if limitVal > 0 {
		percent := usedVal / limitVal * 100
		fmt.Printf("mem_usage: %.1f%% (limit: %s)\n", percent, limit)
	}
}
