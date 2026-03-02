package engine

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// RuntimeDeps 提供策略运行所需依赖。
type RuntimeDeps struct {
	DB     *gorm.DB
	Redis  *redis.Client
	Logger *logrus.Logger
}

// RuntimeContext 为单次策略执行上下文。
type RuntimeContext struct {
	context.Context
	StrategyID string
	Mode       string
	Now        time.Time
	Deps       RuntimeDeps
	StateStore StateStore
}

// Strategy 定义通用策略接口。
type Strategy interface {
	ID() string
	Description() string
	Run(ctx RuntimeContext) error
	Status(ctx RuntimeContext) (map[string]any, error)
}

// StateStore 定义策略状态存储抽象。
type StateStore interface {
	Load(ctx context.Context, key string, target any) error
	Save(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}
