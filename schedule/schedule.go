package schedule

import (
	"context"
	"fmt"
	"time"

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

func NewScheduler(loc *time.Location, logger log.Logger) *Scheduler {
	return &Scheduler{
		cron: cron.New(cron.WithLocation(loc)),
		log:  logtos.NewActsHelper(logger),
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

func NewSystemMonitor(ioPath, path []string) (*SystemMonitor, error) {
	cpuMonitor, err := NewCPUMonitor()
	if err != nil {
		return nil, err
	}

	return &SystemMonitor{
		cm: cpuMonitor,
		mm: NewMemoryMonitor(),
		dm: NewDiskMonitor(ioPath, path),
		nm: NewNetworkMonitor(),
	}, nil
}

func (s *SystemMonitor) Execute(ctx context.Context, logger *logtos.ActsHelper) error {
	// cpu
	cpuUsage, err := s.cm.safeGetUsage()
	if err != nil {
		logger.Errorw(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("collect cpu_usage failed, %s", err))
	}
	logger.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("cpu_usage: %.2f%%", cpuUsage))

	// mem
	memUsed, memLimit, err := s.mm.collectMemStats()
	if err != nil {
		logger.Errorw(logtos.ModuleSystem, MonitorSource, "collect mem_usage failed")
	}
	log.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("mem_used: %s, mem_limit: %s", memUsed, memLimit))

	// disk
	err = s.dm.collectDiskStats(logger)
	if err != nil {
		logger.Errorw(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("collect disk stats failed, %s", err))
	}

	// network
	err = s.nm.collectNetworkStats(logger)
	if err != nil {
		logger.Errorw(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("collect network stats failed, %s", err))
	}

	return nil
}
