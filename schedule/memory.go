package schedule

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/mem"
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
	used  uint64
	total uint64
}

func NewMemoryMonitor() *MemoryMonitor {
	return &MemoryMonitor{}
}

func (m *MemoryMonitor) collectMemStats() (uint64, uint64, error) {
	// 通过cgroup v2接口获取
	if memUsed, memLimit, err := readCgroupV2Memory(); err == nil {
		return memUsed, memLimit, nil
	}

	// 通过cgroup v1接口获取
	if memUsed, memLimit, err := readCgroupV1Memory(); err == nil {
		return memUsed, memLimit, nil
	}

	// 回退到gopsutil
	if memInfo, err := mem.VirtualMemory(); err == nil {
		return memInfo.Used, memInfo.Total, nil
	}

	return 0, 0, errors.New("collect memory stats failed")
}

// readCgroupMemoryV1 读取cgroup v1内存信息
func readCgroupV1Memory() (used, limit uint64, err error) {
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

// readCgroupV2MemoryCurrent 读取cgroup v2内存使用量
func readCgroupV2Memory() (used, limit uint64, err error) {
	cgroupPath, err := getCurrentCgroupPath()
	if err != nil {
		return 0, 0, err
	}

	usedData, err := os.ReadFile(filepath.Join(cgroupPath, "memory.current"))
	if err != nil {
		return 0, 0, err
	}

	used, err = strconv.ParseUint(strings.TrimSpace(string(usedData)), 10, 64)
	if err != nil {
		return 0, 0, err
	}

	limitData, err := os.ReadFile(filepath.Join(cgroupPath, "memory.max"))
	if err != nil {
		return 0, 0, err
	}
	limitStr := strings.TrimSpace(string(limitData))

	if limitStr == "max" {
		limit = ^uint64(0) // 设置为最大值
	} else {
		limit, err = strconv.ParseUint(limitStr, 10, 64)
		if err != nil {
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
