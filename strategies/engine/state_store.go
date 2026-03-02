package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStateStore struct {
	client     *redis.Client
	strategyID string
}

func NewRedisStateStore(client *redis.Client, strategyID string) *RedisStateStore {
	return &RedisStateStore{client: client, strategyID: strategyID}
}

func (s *RedisStateStore) key(k string) string {
	return fmt.Sprintf("strategy:%s:state:%s", s.strategyID, k)
}

func (s *RedisStateStore) Load(ctx context.Context, key string, target any) error {
	if s.client == nil {
		return nil
	}
	raw, err := s.client.Get(ctx, s.key(key)).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), target)
}

func (s *RedisStateStore) Save(ctx context.Context, key string, value any, ttl time.Duration) error {
	if s.client == nil {
		return nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.key(key), string(b), ttl).Err()
}

func (s *RedisStateStore) Delete(ctx context.Context, key string) error {
	if s.client == nil {
		return nil
	}
	return s.client.Del(ctx, s.key(key)).Err()
}
