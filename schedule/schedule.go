package schedule

import (
	"context"
	"fmt"
	"time"

	pbc "git.gonit.codes/dealer/actshub/protocol/go/common/v1"

	"github.com/go-kratos/kratos/v2/log"
	logtos "github.com/hkjojo/go-toolkits/log/v2/kratos"
	"github.com/robfig/cron/v3"
)

const MonitorSource = "Monitor"

type Task interface {
	Execute(ctx context.Context, logger *logtos.ActsHelper) error
}

type Scheduler struct {
	cron *cron.Cron
	log  *logtos.ActsHelper
}

// NewScheduler  cron start with seconds
func NewScheduler(loc *time.Location, logger log.Logger) *Scheduler {
	return &Scheduler{
		cron: cron.New(
			cron.WithSeconds(),
			cron.WithLocation(loc),
		),
		log: logtos.NewActsHelper(logger),
	}
}

type ScheduleTask struct {
	Name    string
	Cron    string
	CtxTime time.Duration
	Task    Task
}

func (s *Scheduler) AddTask(t *ScheduleTask) error {
	_, err := s.cron.AddFunc(t.Cron, func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Errorw(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("%s task panic recovered", t.Name))
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), t.CtxTime)
		defer cancel()

		if err := t.Task.Execute(ctx, s.log); err != nil {
			s.log.Errorw(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("%s task execution failed", t.Name))
		}
	})
	return err
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

type SystemMonitor struct {
	cm *CPUMonitor
	mm *MemoryMonitor
	dm *DiskMonitor
	nm *NetworkMonitor
}

func NewSystemMonitor(path []string) (*SystemMonitor, error) {
	cpuMonitor, err := NewCPUMonitor()
	if err != nil {
		return nil, err
	}

	return &SystemMonitor{
		cm: cpuMonitor,
		mm: NewMemoryMonitor(),
		dm: NewDiskMonitor(path),
		nm: NewNetworkMonitor(),
	}, nil
}

func (s *SystemMonitor) Execute(ctx context.Context, logger *logtos.ActsHelper) error {
	// cpu
	cpuUsage, err := s.cm.safeGetUsage()
	if err != nil {
		logger.Errorw(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("collect cpu_usage failed, %s", err))
	}
	s.cm.lastUsage = cpuUsage
	logger.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("cpu_usage: %.2f%%", cpuUsage))

	// mem
	memUsed, memLimit, err := s.mm.collectMemStats()
	if err != nil {
		logger.Errorw(logtos.ModuleSystem, MonitorSource, "collect mem_stats failed")
	}
	s.mm.used = memUsed
	s.mm.total = memLimit
	logger.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("mem_usage: %.2f%%, mem_used: %s, mem_limit: %s",
		float64(memUsed*100)/float64(memLimit), formatBytes(memUsed), formatBytes(memLimit)))

	// disk
	err = s.dm.collectDiskStats(logger)
	if err != nil {
		logger.Errorw(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("collect disk_stats failed, %s", err))
	}

	// network
	err = s.nm.collectNetworkStats(logger)
	if err != nil {
		logger.Errorw(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("collect network_stats failed, %s", err))
	}

	return nil
}

func (s *SystemMonitor) GetSysStats() *pbc.ServerLoad {
	return &pbc.ServerLoad{
		TrafficReceived:    s.nm.recv,
		TrafficTransmitted: s.nm.sent,
		CPUUsage:           s.cm.lastUsage,
		MemoryAvailable:    s.mm.total - s.mm.used,
		MemoryTotal:        s.mm.total,
		DiskAvailable:      s.dm.total - s.dm.used,
		DiskTotal:          s.dm.total,
		DiskRead:           s.dm.read,
		DiskWrite:          s.dm.write,
	}
}
