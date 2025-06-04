package schedule

import (
	"bufio"
	"fmt"
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
	memUsed, memLimit, err := getContainerMemory()
	if err != nil {
		return 0, 0, err
	}

	return memUsed, memLimit, nil
}

func getContainerMemory() (used, limit uint64, err error) {
	usageData, err := os.ReadFile(cgroupMemUsagePath)
	if err != nil {
		return 0, 0, err
	}
	usage, err := strconv.ParseUint(strings.TrimSpace(string(usageData)), 10, 64)
	if err != nil {
		return 0, 0, err
	}

	if limitData, err := os.ReadFile(cgroupMemLimitPath); err == nil {
		if limit, err = strconv.ParseUint(strings.TrimSpace(string(limitData)), 10, 64); err != nil {
			return 0, 0, err
		}
	} else {
		return 0, 0, err
	}

	statData, err := os.ReadFile(cgroupMemStatPath)
	if err != nil {
		return 0, 0, err
	}

	inactiveFile := uint64(0)
	scanner := bufio.NewScanner(strings.NewReader(string(statData)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "total_inactive_file ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				inactiveFile, _ = strconv.ParseUint(fields[1], 10, 64)
			}
			break
		}
	}
	used = usage - inactiveFile

	return used, limit, nil
}

// readCgroupMemoryV1 读取cgroup v1内存信息
func readCgroupV1Memory() (used, limit uint64, err error) {
	if data, err := os.ReadFile(cgroupMemUsagePath); err == nil {
		if used, err = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err != nil {
			return 0, 0, err
		}
	} else {
		return 0, 0, err
	}

	if data, err := os.ReadFile(cgroupMemLimitPath); err == nil {
		if limit, err = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err != nil {
			return 0, 0, err
		}
	} else {
		return 0, 0, err
	}

	return used, limit, nil
}

// 需挂载/sys/fs/cgroup，测试结果与grafana数据不一致
// readCgroupV2MemoryCurrent 读取cgroup v2内存使用量
func readCgroupV2Memory() (used, limit uint64, err error) {
	path, err := getCurrentCgroupPath()
	if err != nil {
		return 0, 0, err
	}

	usedData, err := os.ReadFile(filepath.Join(path, "memory.current"))
	if err != nil {
		return 0, 0, err
	}

	used, err = strconv.ParseUint(strings.TrimSpace(string(usedData)), 10, 64)
	if err != nil {
		return 0, 0, err
	}

	limitData, err := os.ReadFile(filepath.Join(path, "memory.max"))
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
				return filepath.Join(cgroupPath, parts[2]), nil
			}
		}
	}

	return "", fmt.Errorf("memory cgroup not found")
}
