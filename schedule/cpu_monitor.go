package schedule

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

const (
	envK8sPodUID     = "POD_UID"
	cgroupV1CPUPath  = "/sys/fs/cgroup/cpu,cpuacct"
	cgroupV2CPUPath  = "/sys/fs/cgroup"
	cpuAcctUsageFile = "cpuacct.usage"
	cpuStatFile      = "cpu.stat"
	cpuMaxFile       = "cpu.max"
)

type CPUMonitor struct {
	mu             sync.RWMutex
	isContainer    bool
	cgroupVersion  int
	cpuLimitCores  float64
	lastTotalUsage uint64
	lastTime       time.Time
}

func NewCPUMonitor() (*CPUMonitor, error) {
	m := &CPUMonitor{
		lastTime: time.Now(),
	}

	if isRunningInK8s() || hasContainerCgroups() {
		m.isContainer = true
		m.cgroupVersion = detectCgroupVersion()
		if limit, err := m.getCPULimit(); err == nil {
			m.cpuLimitCores = limit
		}
	}

	// 初始化基准数据
	if m.isContainer {
		usage, err := m.readContainerCPUUsage()
		if err != nil {
			return nil, fmt.Errorf("container CPU init failed: %w", err)
		}
		m.lastTotalUsage = usage
	} else {
		percent, err := cpu.Percent(0, false)
		if err != nil {
			return nil, fmt.Errorf("host CPU init failed: %w", err)
		}
		m.lastTotalUsage = uint64(percent[0])
	}
	fmt.Printf("cpu monitor info, isContainer: %t, cgroupVersion: %d, cpuLimitCores: %f\n", m.isContainer,
		m.cgroupVersion, m.cpuLimitCores)

	return m, nil
}

func (cm *CPUMonitor) safeGetUsage() (float64, float64, error) {
	/*usage, err := cm.GetUsage()
	if err != nil && cm.isContainer {
		// 尝试回退到宿主机的统计方式
		if hostUsage, hostErr := cpu.Percent(0, false); hostErr == nil {
			return hostUsage[0], nil
		}
	}
	return usage, err*/
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	interval := now.Sub(cm.lastTime).Seconds()

	containerUsage, err := cm.getContainerUsage(now, interval)
	if err != nil {
		return 0, 0, err
	}

	hostUsage, err := cm.getHostUsage(now, interval)
	if err != nil {
		return 0, 0, err
	}

	return containerUsage, hostUsage, nil
}

func (cm *CPUMonitor) GetUsage() (float64, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	interval := now.Sub(cm.lastTime).Seconds()

	if cm.isContainer {
		return cm.getContainerUsage(now, interval)
	}
	return cm.getHostUsage(now, interval)
}

// getHostUsage 获取宿主机CPU使用率
func (cm *CPUMonitor) getHostUsage(now time.Time, interval float64) (float64, error) {
	percent, err := cpu.Percent(0, false)
	if err != nil {
		return 0, fmt.Errorf("failed to get host CPU usage: %w", err)
	}

	cm.lastTime = now

	return percent[0], nil
}

// getContainerUsage 获取容器CPU使用率
func (cm *CPUMonitor) getContainerUsage(now time.Time, interval float64) (float64, error) {
	currentUsage, err := cm.readContainerCPUUsage()
	if err != nil {
		return 0, fmt.Errorf("failed to read container CPU usage: %w", err)
	}

	delta := currentUsage - cm.lastTotalUsage
	usage := float64(delta) / (interval * 1e9) * 100

	// 如果有CPU限制则计算相对使用率
	if cm.cpuLimitCores > 0 {
		usage = usage / cm.cpuLimitCores
	}

	cm.lastTotalUsage = currentUsage
	cm.lastTime = now
	return usage, nil
}

func isRunningInK8s() bool {
	return os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}

func hasContainerCgroups() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := os.Stat(cgroupV1CPUPath); err == nil {
		return true
	}
	return false
}

func detectCgroupVersion() int {
	if _, err := os.Stat(filepath.Join(cgroupV2CPUPath, cpuMaxFile)); err == nil {
		return 2
	}
	return 1
}

func (cm *CPUMonitor) getCPULimit() (float64, error) {
	if limit := getK8sCPULimit(); limit > 0 {
		return limit, nil
	}
	switch cm.cgroupVersion {
	case 1:
		return cm.getV1CPULimit()
	case 2:
		return cm.getV2CPULimit()
	default:
		return 0, fmt.Errorf("unsupported cgroup version")
	}
}

func getK8sCPULimit() float64 {
	if limit := os.Getenv("CPU_LIMIT"); limit != "" {
		if cores, err := strconv.ParseFloat(limit, 64); err == nil {
			return cores * 100
		}
	}
	return 0
}

func (cm *CPUMonitor) getV1CPULimit() (float64, error) {
	periodFile := filepath.Join(cgroupV1CPUPath, "cpu.cfs_period_us")
	quotaFile := filepath.Join(cgroupV1CPUPath, "cpu.cfs_quota_us")

	periodData, err := os.ReadFile(periodFile)
	if err != nil {
		return 0, err
	}
	quotaData, err := os.ReadFile(quotaFile)
	if err != nil {
		return 0, err
	}

	period, _ := strconv.ParseFloat(strings.TrimSpace(string(periodData)), 64)
	quota, _ := strconv.ParseFloat(strings.TrimSpace(string(quotaData)), 64)

	if quota <= 0 || period <= 0 {
		return 0, fmt.Errorf("no CPU limit set")
	}

	return (quota / period) * 100, nil
}

func (cm *CPUMonitor) getV2CPULimit() (float64, error) {
	data, err := os.ReadFile(filepath.Join(cgroupV2CPUPath, cpuMaxFile))
	if err != nil {
		return 0, err
	}

	parts := strings.SplitN(strings.TrimSpace(string(data)), " ", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid cpu.max format")
	}

	mx := parts[0]
	if mx == "max" {
		return 0, fmt.Errorf("no CPU limit set")
	}

	period := parts[1]
	quota, _ := strconv.ParseFloat(mx, 64)
	periodVal, _ := strconv.ParseFloat(period, 64)

	if quota <= 0 || periodVal <= 0 {
		return 0, fmt.Errorf("invalid CPU limit values")
	}

	return (quota / periodVal) * 100, nil
}

func (cm *CPUMonitor) readContainerCPUUsage() (uint64, error) {
	var usageFile string
	switch cm.cgroupVersion {
	case 1:
		usageFile = filepath.Join(cgroupV1CPUPath, cpuAcctUsageFile)
	case 2:
		usageFile = filepath.Join(cgroupV2CPUPath, cpuAcctUsageFile)
	default:
		return 0, fmt.Errorf("unsupported cgroup version")
	}

	data, err := os.ReadFile(usageFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read CPU usage file: %w", err)
	}

	usage, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse CPU usage: %w", err)
	}

	return usage, nil
}
