package schedule

import (
	"bufio"
	"errors"
	"fmt"
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
	used  uint64
	total uint64
}

func NewMemoryMonitor() *MemoryMonitor {
	return &MemoryMonitor{}
}

func (m *MemoryMonitor) collectMemStats() (uint64, uint64, error) {
	// 通过cgroup v1接口获取
	if memUsed, memLimit, err := readCgroupMemoryV1(); err == nil {
		return memUsed, memLimit, nil
	}

	// 通过cgroup v2接口获取
	if memUsed, memLimit, err := readCgroupMemoryV2(); err == nil {
		return memUsed, memLimit, nil
	}

	// 回退到gopsutil
	if memInfo, err := mem.VirtualMemory(); err == nil {
		return memInfo.Used, memInfo.Total, nil
	}

	return 0, 0, errors.New("collect memory stats failed")
}

func showContainerLimitComparison(used, limit string) {
	usedVal, _ := strconv.ParseFloat(strings.Split(used, " ")[0], 64)
	limitVal, _ := strconv.ParseFloat(strings.Split(limit, " ")[0], 64)

	if limitVal > 0 {
		percent := usedVal / limitVal * 100
		fmt.Printf("mem_usage: %.1f%% (limit: %s)\n", percent, limit)
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
