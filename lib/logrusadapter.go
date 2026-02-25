package lib

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type LogrusAdapter struct {
	Log           *logrus.Logger
	LogLevel      logger.LogLevel
	SlowThreshold time.Duration
}

func NewLogrusAdapter() *LogrusAdapter {
	log := &LogrusAdapter{
		Log:           LoadLogger(),
		LogLevel:      logger.Warn,
		SlowThreshold: 2 * time.Second,
	}

	return log
}

// LogMode 让 GORM 可以设置日志级别
func (l *LogrusAdapter) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info 级别日志
func (l *LogrusAdapter) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		l.Log.Infof(msg, data...)
	}
}

// Warn 级别日志
func (l *LogrusAdapter) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		l.Log.Warnf(msg, data...)
	}
}

// Error 级别日志
func (l *LogrusAdapter) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		l.Log.Errorf(msg, data...)
	}
}

// Trace 负责打印 SQL 执行日志
func (l *LogrusAdapter) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	switch {
	case err != nil && l.LogLevel >= logger.Error:
		// 这个太多了,如果未查询到数据,不必打印日志.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}

		l.Log.WithFields(logrus.Fields{
			"sql":          sql,
			"rowsAffected": rows,
			"duration":     elapsed,
			"error":        err,
		}).Errorf("SQL error: %s, %s", err, sql)

	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
		l.Log.WithFields(logrus.Fields{
			"sql":          sql,
			"rowsAffected": rows,
			"duration":     elapsed,
		}).Warnf("Slow SQL:%.2f s, %s", elapsed.Seconds(), sql)

	case l.LogLevel == logger.Info:
		l.Log.WithFields(logrus.Fields{
			"sql":          sql,
			"rowsAffected": rows,
			"duration":     elapsed,
		}).Info("SQL executed")
	}
}
