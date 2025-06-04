package schedule

import (
	"bufio"
	"errors"
	"fmt"
	logtos "github.com/hkjojo/go-toolkits/log/v2/kratos"
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

func (m *MemoryMonitor) getMemStats(logger *logtos.ActsHelper) {
	if memUsed, memLimit, err := readCgroupV2Memory(); err == nil {
		logger.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("cgroupV2, mem_used:%s, mem_limit:%s",
			formatBytes(memUsed), formatBytes(memLimit)))
	} else {
		logger.Errorw(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("read cgroupV2Memory failed, %s", err))
	}

	if memUsed, memLimit, err := readCgroupV1Memory(); err == nil {
		logger.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("cgroupV1, mem_used:%s, mem_limit:%s",
			formatBytes(memUsed), formatBytes(memLimit)))
	} else {
		logger.Errorw(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("read cgroupV1Memory failed, %s", err))
	}

	workSet, err := getContainerMemoryWorkingSet()
	if err != nil {
		logger.Errorw(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("getContainerMemoryWorking failed, %s", err))
	}
	logger.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("get ContainerMemory workset: %s", formatBytes(workSet)))
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

func getContainerMemoryWorkingSet() (uint64, error) {
	// 读取 memory.usage_in_bytes
	usageData, err := os.ReadFile("/sys/fs/cgroup/memory/memory.usage_in_bytes")
	if err != nil {
		return 0, err
	}
	usage, err := strconv.ParseUint(strings.TrimSpace(string(usageData)), 10, 64)
	if err != nil {
		return 0, err
	}

	// 读取 memory.stat
	statData, err := os.ReadFile("/sys/fs/cgroup/memory/memory.stat")
	if err != nil {
		return 0, err
	}

	// 解析 inactive_file
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

	// 计算 working set
	workingSet := usage - inactiveFile
	return workingSet, nil
}
