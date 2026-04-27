package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache is a wrapper for Redis client
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache initializes a new Redis cache client
func NewRedisCache(ctx context.Context, redisURL string) (*RedisCache, error) {
	const op = "store.NewRedisCache"

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to parse redis URL: %w", op, err)
	}

	client := redis.NewClient(opt)

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("%s: failed to ping redis: %w", op, err)
	}

	return &RedisCache{client: client}, nil
}

// Get retrieves a value from cache
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	return c.client.Get(ctx, key).Bytes()
}

// Set stores a value in cache
func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Del removes a key from cache
func (c *RedisCache) Del(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// DelPattern removes keys matching a pattern
func (c *RedisCache) DelPattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			c.client.Del(ctx, key)
		}
		if cursor == 0 {
			break
		}
	}
	return nil
}
