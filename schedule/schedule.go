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
	Name string
	cron string
	Task Task
}

func (s *Scheduler) AddTask(t *ScheduleTask) error {
	_, err := s.cron.AddFunc(t.cron, func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Errorw(logtos.ModuleSystem, MonitorSource, t.Name+" task panic recovered")
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		if err := t.Task.Execute(ctx, s.log); err != nil {
			s.log.Errorw(logtos.ModuleSystem, MonitorSource, t.Name+" task execution failed")
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

func NewSystemMonitor() (*SystemMonitor, error) {
	cpuMonitor, err := NewCPUMonitor()
	if err != nil {
		return nil, err
	}

	return &SystemMonitor{
		cm: cpuMonitor,
		mm: NewMemoryMonitor(),
		dm: NewDiskMonitor(),
		nm: NewNetworkMonitor(),
	}, nil
}

func (s *SystemMonitor) Execute(ctx context.Context, logger *logtos.ActsHelper) error {
	// cpu
	cpuUsage, err := s.cm.safeGetUsage()
	if err != nil {
		logger.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("get cpu usage failed, %s", err))
		return err
	}
	logger.Infow(logtos.ModuleSystem, MonitorSource, fmt.Sprintf("cpu_usage: %.2f", cpuUsage))
	// mem
	s.mm.getMemStats(logger)
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
