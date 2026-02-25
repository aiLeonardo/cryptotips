package lib

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
)

type RedisLogger struct {
	Log           *logrus.Logger
	LogLevel      logger.LogLevel
	SlowThreshold time.Duration
}

func NewRedisLogger() *RedisLogger {
	log := &RedisLogger{
		Log:           LoadLogger(),
		LogLevel:      logger.Info,
		SlowThreshold: 2 * time.Second,
	}

	return log
}

// Info 级别日志
func (l *RedisLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		l.Log.Infof(msg, data...)
	}
}

// Warn 级别日志
func (l *RedisLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		l.Log.Warnf(msg, data...)
	}
}

// Error 级别日志
func (l *RedisLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		l.Log.Errorf(msg, data...)
	}
}

// Error 级别日志
func (l *RedisLogger) Printf(ctx context.Context, msg string, data ...interface{}) {
	l.Info(ctx, msg, data)
}
