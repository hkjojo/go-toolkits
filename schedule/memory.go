package schedule

import (
	"bufio"
	"fmt"
	logtos "github.com/hkjojo/go-toolkits/log/v2/kratos"
	"github.com/shirou/gopsutil/v3/mem"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

/*func (m *MemoryMonitor) collectMemStats(log *logtos.ActsHelper) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	alloc := formatBytes(memStats.Alloc)         //当前堆对象分配的内存
	sys := formatBytes(memStats.Sys)             //从操作系统获得的总内存
	heapAlloc := formatBytes(memStats.HeapAlloc) // 堆上分配且仍在使用的内存
	totalAlloc := formatBytes(memStats.TotalAlloc)

	log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("mem_usage, alloc: %s, sys: %s, heap_alloc: %s, "+
		"total_alloc: %s", alloc, sys, heapAlloc, totalAlloc))
}*/

func (m *MemoryMonitor) collectMemStats(log *logtos.ActsHelper) {
	getContainerMemory(log)
}

func showContainerLimitComparison(used, limit string) {
	usedVal, _ := strconv.ParseFloat(strings.Split(used, " ")[0], 64)
	limitVal, _ := strconv.ParseFloat(strings.Split(limit, " ")[0], 64)

	if limitVal > 0 {
		percent := usedVal / limitVal * 100
		fmt.Printf("mem_usage: %.1f%% (limit: %s)\n", percent, limit)
	}
}

func getContainerMemory(log *logtos.ActsHelper) {
	// 方法1：通过cgroup v1接口获取
	if memUsed, memLimit, err := readCgroupMemoryV1(); err == nil {
		log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("container_mem_v1, used:%s, limit:%s",
			formatBytes(memUsed), formatBytes(memLimit)))
	}

	// 方法2：通过cgroup v2接口获取
	if memUsed, memLimit, err := readCgroupMemoryV2(); err == nil {
		log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("container_mem_v2, used:%s, limit:%s",
			formatBytes(memUsed), formatBytes(memLimit)))
	}

	// 方法3：回退到gopsutil（不准确）
	if memInfo, err := mem.VirtualMemory(); err == nil {
		log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("container_mem_gops, used:%s, limit:%s",
			formatBytes(memInfo.Used), formatBytes(memInfo.Total)))
	}
}

// readCgroupMemoryV1 读取cgroup v1内存信息
func readCgroupMemoryV1() (used, limit uint64, err error) {
	// 尝试读取内存使用量
	if data, err := os.ReadFile("/sys/fs/cgroup/memory/memory.usage_in_bytes"); err == nil {
		if used, err = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err != nil {
			return 0, 0, err
		}
	} else {
		return 0, 0, err
	}

	// 尝试读取内存限制
	if data, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
		if limit, err = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err != nil {
			return 0, 0, err
		}
	} else {
		return 0, 0, err
	}

	return used, limit, nil
}

func readCgroupMemoryV2() (used, limit uint64, err error) {
	// 获取当前cgroup路径
	cgroupPath, err := getCurrentCgroupPath()
	if err != nil {
		return 0, 0, err
	}

	// 读取内存统计
	memStatPath := filepath.Join(cgroupPath, "memory.stat")
	file, err := os.Open(memStatPath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "anon":
			if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				used += val
			}
		case "file":
			if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				used += val
			}
		}
	}

	// 读取内存限制
	if data, err := os.ReadFile(filepath.Join(cgroupPath, "memory.max")); err == nil {
		strVal := strings.TrimSpace(string(data))
		if strVal == "max" {
			limit = ^uint64(0) // 无限制
		} else if limit, err = strconv.ParseUint(strVal, 10, 64); err != nil {
			return 0, 0, err
		}
	}

	return used, limit, nil
}

func getCurrentCgroupPath() (string, error) {
	// 读取当前进程的cgroup信息
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}
		// 查找控制器中包含"memory"的条目
		controllers := strings.Split(parts[1], ",")
		for _, ctrl := range controllers {
			if ctrl == "memory" {
				return filepath.Join("/sys/fs/cgroup", parts[2]), nil
			}
		}
	}

	return "", fmt.Errorf("memory cgroup not found")
}
