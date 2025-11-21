package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Cache provides caching for LLM responses
type Cache interface {
	// Get retrieves a cached response
	Get(ctx context.Context, key string) (*Response, bool)
	// Set stores a response in cache
	Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error
	// Stats returns cache statistics
	Stats() CacheStats
}

// CacheStats holds cache statistics
type CacheStats struct {
	Hits   int64
	Misses int64
	Size   int64
}

// MemoryCache is an in-memory LRU cache for LLM responses
type MemoryCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	maxSize int
	ttl     time.Duration
	stats   CacheStats
}

type cacheEntry struct {
	response  *Response
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxSize int, ttl time.Duration) *MemoryCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	cache := &MemoryCache{
		entries: make(map[string]*cacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// Get retrieves a cached response
func (c *MemoryCache) Get(ctx context.Context, key string) (*Response, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.stats.Misses++
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	c.stats.Hits++
	c.mu.Unlock()

	keyPreview := key
	if len(key) > 16 {
		keyPreview = key[:16] + "..."
	}
	log.Debug().Str("key", keyPreview).Msg("cache hit")
	return entry.response, true
}

// Set stores a response in cache
func (c *MemoryCache) Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.ttl
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &cacheEntry{
		response:  resp,
		expiresAt: time.Now().Add(ttl),
	}
	c.stats.Size = int64(len(c.entries))

	keyPreview := key
	if len(key) > 16 {
		keyPreview = key[:16] + "..."
	}
	log.Debug().Str("key", keyPreview).Dur("ttl", ttl).Msg("cached response")
	return nil
}

// Stats returns cache statistics
func (c *MemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// evictOldest removes the oldest entry (simple LRU approximation)
func (c *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.expiresAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// cleanup periodically removes expired entries
func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.entries {
			if now.After(entry.expiresAt) {
				delete(c.entries, key)
			}
		}
		c.stats.Size = int64(len(c.entries))
		c.mu.Unlock()
	}
}

// GenerateCacheKey creates a cache key from a request
func GenerateCacheKey(req *Request) string {
	// Create a deterministic representation of the request
	keyData := struct {
		Tier        Tier
		System      string
		Messages    []Message
		Temperature float64
	}{
		Tier:        req.Tier,
		System:      req.System,
		Messages:    req.Messages,
		Temperature: req.Temperature,
	}

	data, _ := json.Marshal(keyData)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// CachedRouter wraps a Router with caching
type CachedRouter struct {
	router *Router
	cache  Cache
	ttl    time.Duration
}

// NewCachedRouter creates a router with caching enabled
func NewCachedRouter(router *Router, cache Cache, ttl time.Duration) *CachedRouter {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &CachedRouter{
		router: router,
		cache:  cache,
		ttl:    ttl,
	}
}

// Complete sends a completion request with caching
func (r *CachedRouter) Complete(ctx context.Context, req *Request) (*Response, error) {
	// Generate cache key
	cacheKey := GenerateCacheKey(req)

	// Check cache first
	if cached, ok := r.cache.Get(ctx, cacheKey); ok {
		cached.Cached = true
		return cached, nil
	}

	// Call underlying router
	resp, err := r.router.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache successful response
	if err := r.cache.Set(ctx, cacheKey, resp, r.ttl); err != nil {
		log.Warn().Err(err).Msg("failed to cache response")
	}

	return resp, nil
}

// HealthCheck delegates to underlying router
func (r *CachedRouter) HealthCheck() error {
	return r.router.HealthCheck()
}

// CacheStats returns cache statistics
func (r *CachedRouter) CacheStats() CacheStats {
	return r.cache.Stats()
}

// GetRouter returns the underlying router
func (r *CachedRouter) GetRouter() *Router {
	return r.router
}

// RedisCache implements Cache using Redis (optional, requires redis client)
type RedisCache struct {
	// Placeholder for Redis implementation
	// Would use github.com/redis/go-redis/v9
	client  interface{}
	prefix  string
	ttl     time.Duration
	stats   CacheStats
	statsMu sync.RWMutex
}

// NewRedisCache creates a new Redis cache
// For now, this returns nil as Redis is optional
func NewRedisCache(addr, password string, db int, ttl time.Duration) *RedisCache {
	// Redis implementation would go here
	// For MVP, we use MemoryCache
	log.Info().Msg("Redis cache not implemented, use MemoryCache instead")
	return nil
}

// NullCache is a no-op cache for testing or when caching is disabled
type NullCache struct{}

func (c *NullCache) Get(ctx context.Context, key string) (*Response, bool) {
	return nil, false
}

func (c *NullCache) Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error {
	return nil
}

func (c *NullCache) Stats() CacheStats {
	return CacheStats{}
}

// Helper function to create appropriate cache based on config
func CreateCache(cacheType string, maxSize int, ttl time.Duration) Cache {
	switch cacheType {
	case "memory":
		return NewMemoryCache(maxSize, ttl)
	case "none", "":
		return &NullCache{}
	default:
		log.Warn().Str("type", cacheType).Msg("unknown cache type, using memory cache")
		return NewMemoryCache(maxSize, ttl)
	}
}
