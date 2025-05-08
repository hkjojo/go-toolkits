package sql

import (
	"context"
	"errors"
	"time"

	tlog "github.com/hkjojo/go-toolkits/log/v2"
	tlogk "github.com/hkjojo/go-toolkits/log/v2/kratos"
	gormLogger "gorm.io/gorm/logger"
)

// GormLogger 自定义 GORM 日志结构，包含慢查询阈值和当前日志级别
type GormLogger struct {
	log           *tlogk.Helper
	SlowThreshold time.Duration
	LogLevel      gormLogger.LogLevel
}

func NewGormLogger(cfg *tlog.Config, slowThreshold time.Duration) (*GormLogger, error) {
	logger, err := tlogk.NewZapLog(cfg)
	if err != nil {
		return nil, err
	}

	if slowThreshold == 0 {
		slowThreshold = 200 * time.Millisecond
	}

	return &GormLogger{
		log:           tlogk.NewHelper(logger),
		SlowThreshold: slowThreshold,
		LogLevel:      gormLogger.Warn,
	}, nil
}

// WithSlowThreshold ...
func (l *GormLogger) WithSlowThreshold(slowThreshold time.Duration) gormLogger.Interface {
	l.SlowThreshold = slowThreshold
	return l
}

// LogMode 实现 logger.Interface 接口，用于设置日志级别
func (l *GormLogger) LogMode(level gormLogger.LogLevel) gormLogger.Interface {
	l.LogLevel = level
	return l
}

// Info 实现日志接口的 Info 方法
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel < gormLogger.Info {
		return
	}
	tlog.Infof(msg, data...)
}

// Warn 实现日志接口的 Warn 方法
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel < gormLogger.Warn {
		return
	}
	tlog.Warnf(msg, data...)
}

// Error 实现日志接口的 Error 方法
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel < gormLogger.Error {
		return
	}
	tlog.Errorf(msg, data...)
}

// Trace 实现日志接口的 Trace 方法，用于记录 SQL 执行情况
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormLogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	f := func() (string, int64) {
		sql, rows := fc()
		trimmedSQL := sql
		if len(trimmedSQL) > 200 {
			trimmedSQL = trimmedSQL[:200] + "..."
		}
		return trimmedSQL, rows
	}
	switch {
	case err != nil && l.LogLevel >= gormLogger.Error && (!errors.Is(err, gormLogger.ErrRecordNotFound)):
		sql, rows := f()
		l.log.Errorw("db error", "sql", sql, "rows", rows, "cost", elapsed, "err", err)
	case l.LogLevel >= gormLogger.Warn && elapsed > l.SlowThreshold:
		sql, rows := f()
		l.log.Warnw("slow threshold", "sql", sql, "rows", rows, "cost", elapsed)
	case l.LogLevel == gormLogger.Info:
		sql, rows := f()
		l.log.Infow("exec sql", "sql", sql, "rows", rows, "cost", elapsed)
	}
}
