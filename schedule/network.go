package schedule

import (
	"fmt"
	"strings"
	"time"

	logtos "github.com/hkjojo/go-toolkits/log/v2/kratos"
	"github.com/shirou/gopsutil/v3/net"
)

type NetworkMonitor struct {
	prevNetStats []net.IOCountersStat
	prevTime     time.Time
}

func NewNetworkMonitor() *NetworkMonitor {
	return &NetworkMonitor{}
}

func (m *NetworkMonitor) collectNetworkStats(log *logtos.ActsHelper) error {
	currentTime := time.Now()
	deltaSec := currentTime.Sub(m.prevTime).Seconds()
	defer func() {
		m.prevTime = currentTime
	}()

	netStats, _ := net.IOCounters(true)
	var (
		sent, recv uint64
	)
	if m.prevNetStats != nil {
		for i, current := range netStats {
			if strings.HasPrefix(current.Name, "lo") || strings.HasPrefix(current.Name, "docker") ||
				strings.HasPrefix(current.Name, "veth") {
				continue
			}

			if i < len(m.prevNetStats) {
				prev := m.prevNetStats[i]
				sent += current.BytesSent - prev.BytesSent
				recv += current.BytesRecv - prev.BytesRecv
			}
		}

		log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("net_sent: %s , %s, net_recv: %s , %s",
			formatByteSpeed(sent, deltaSec), formatBytes(sent), formatByteSpeed(recv, deltaSec), formatBytes(recv)))
	}
	m.prevNetStats = netStats

	return nil
}
