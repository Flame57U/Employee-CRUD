package store

import (
	"context"
	"fmt"
	"time"

	"github.com/quantsaas/platform/internal/saas/config"
	"github.com/redis/go-redis/v9"
)

// RedisClient wraps go-redis for champion gene caching and session storage.
// It is NOT a signal-passing channel — use NATS JetStream for that.
type RedisClient struct {
	c *redis.Client
}

// NewRedis creates a connected Redis client.
func NewRedis(cfg *config.Config) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisClient{c: rdb}, nil
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.c.Get(ctx, key).Result()
}

// Set stores value with an optional TTL; pass 0 for no expiry.
func (r *RedisClient) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return r.c.Set(ctx, key, value, ttl).Err()
}

func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.c.Del(ctx, keys...).Err()
}
