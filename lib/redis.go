package lib

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type RedisHandler struct {
	rdb    *redis.Client
	logger *logrus.Logger
}

func NewRedisHandler(rdb *redis.Client, logger *logrus.Logger) *RedisHandler {
	return &RedisHandler{
		rdb:    rdb,
		logger: logger,
	}
}

// SetMarketState 缓存当前市场状态
func (r *RedisHandler) SetMarketState(symbol, state string) error {
	ctx := context.Background()
	key := fmt.Sprintf("market_state:%s", symbol)
	return r.rdb.Set(ctx, key, state, 24*time.Hour).Err()
}

// GetMarketState 获取缓存的市场状态
func (r *RedisHandler) GetMarketState(symbol string) (string, error) {
	ctx := context.Background()
	key := fmt.Sprintf("market_state:%s", symbol)
	return r.rdb.Get(ctx, key).Result()
}

// SetOrderLock 设置下单防重复锁（防止同一信号重复下单）
// ttl：锁过期时间
func (r *RedisHandler) SetOrderLock(symbol, side string, ttl time.Duration) (bool, error) {
	ctx := context.Background()
	key := fmt.Sprintf("order_lock:%s:%s", symbol, side)
	return r.rdb.SetNX(ctx, key, "1", ttl).Result()
}

// ReleaseOrderLock 释放下单锁
func (r *RedisHandler) ReleaseOrderLock(symbol, side string) error {
	ctx := context.Background()
	key := fmt.Sprintf("order_lock:%s:%s", symbol, side)
	return r.rdb.Del(ctx, key).Err()
}

// SetDrawdownPause 设置回撤暂停标记
func (r *RedisHandler) SetDrawdownPause(reason string, ttl time.Duration) error {
	ctx := context.Background()
	return r.rdb.Set(ctx, "drawdown_pause", reason, ttl).Err()
}

// GetDrawdownPause 检查是否处于回撤暂停状态
func (r *RedisHandler) GetDrawdownPause() (string, error) {
	ctx := context.Background()
	val, err := r.rdb.Get(ctx, "drawdown_pause").Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// SetFuturesPause 设置合约暂停标记（连续亏损后暂停）
func (r *RedisHandler) SetFuturesPause(days int) error {
	ctx := context.Background()
	return r.rdb.Set(ctx, "futures_pause", "1", time.Duration(days)*24*time.Hour).Err()
}

// IsFuturesPaused 检查合约是否被暂停
func (r *RedisHandler) IsFuturesPaused() bool {
	ctx := context.Background()
	err := r.rdb.Get(ctx, "futures_pause").Err()
	return err == nil
}

// IncrConsecutiveLoss 增加连续亏损计数，返回当前计数
func (r *RedisHandler) IncrConsecutiveLoss(symbol string) (int64, error) {
	ctx := context.Background()
	key := fmt.Sprintf("consecutive_loss:%s", symbol)
	return r.rdb.Incr(ctx, key).Result()
}

// ResetConsecutiveLoss 重置连续亏损计数
func (r *RedisHandler) ResetConsecutiveLoss(symbol string) error {
	ctx := context.Background()
	key := fmt.Sprintf("consecutive_loss:%s", symbol)
	return r.rdb.Del(ctx, key).Err()
}
