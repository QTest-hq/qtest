package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage implements Storage using Redis for distributed rate limiting
type RedisStorage struct {
	client    *redis.Client
	keyPrefix string
}

// NewRedisStorage creates a new Redis-backed rate limit storage
func NewRedisStorage(redisURL string) (*RedisStorage, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisStorage{
		client:    client,
		keyPrefix: "qtest:ratelimit:",
	}, nil
}

// Increment increments the counter for a key and returns the new count
// Uses Redis INCR with EXPIRE for atomic operations
func (r *RedisStorage) Increment(ctx context.Context, key string, window time.Duration, limit int) (int64, time.Time, error) {
	fullKey := r.keyPrefix + key

	// Use a Lua script for atomic increment + expire
	script := redis.NewScript(`
		local current = redis.call('INCR', KEYS[1])
		if current == 1 then
			redis.call('PEXPIRE', KEYS[1], ARGV[1])
		end
		local ttl = redis.call('PTTL', KEYS[1])
		return {current, ttl}
	`)

	result, err := script.Run(ctx, r.client, []string{fullKey}, window.Milliseconds()).Slice()
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to increment rate limit counter: %w", err)
	}

	count := result[0].(int64)
	ttlMs := result[1].(int64)

	// Calculate reset time from TTL
	var resetAt time.Time
	if ttlMs > 0 {
		resetAt = time.Now().Add(time.Duration(ttlMs) * time.Millisecond)
	} else {
		resetAt = time.Now().Add(window)
	}

	return count, resetAt, nil
}

// Get returns the current count and reset time without incrementing
func (r *RedisStorage) Get(ctx context.Context, key string) (int64, time.Time, error) {
	fullKey := r.keyPrefix + key

	// Get count and TTL in pipeline
	pipe := r.client.Pipeline()
	getCmd := pipe.Get(ctx, fullKey)
	ttlCmd := pipe.PTTL(ctx, fullKey)
	_, err := pipe.Exec(ctx)

	// If key doesn't exist, return 0
	if err == redis.Nil {
		return 0, time.Now(), nil
	}

	count, err := getCmd.Int64()
	if err == redis.Nil {
		return 0, time.Now(), nil
	}
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to get rate limit counter: %w", err)
	}

	ttl := ttlCmd.Val()
	var resetAt time.Time
	if ttl > 0 {
		resetAt = time.Now().Add(ttl)
	} else {
		resetAt = time.Now()
	}

	return count, resetAt, nil
}

// Reset clears the counter for a key
func (r *RedisStorage) Reset(ctx context.Context, key string) error {
	fullKey := r.keyPrefix + key

	if err := r.client.Del(ctx, fullKey).Err(); err != nil {
		return fmt.Errorf("failed to reset rate limit counter: %w", err)
	}

	return nil
}

// Close releases the Redis connection
func (r *RedisStorage) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
