package mutation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache provides caching for mutation testing results.
// It uses file content hashes to determine if cached results are still valid.
type Cache struct {
	config    *CacheConfig
	mu        sync.RWMutex
	entries   map[string]*CacheEntry
	storePath string
}

// CacheConfig configures the mutation cache.
type CacheConfig struct {
	// Enabled controls whether caching is active
	Enabled bool `json:"enabled"`
	// Directory is the cache storage directory
	Directory string `json:"directory"`
	// TTL is how long cached results are valid
	TTL time.Duration `json:"ttl"`
	// MaxEntries is the maximum number of cached entries
	MaxEntries int `json:"maxEntries"`
	// PersistToDisk saves cache to disk between runs
	PersistToDisk bool `json:"persistToDisk"`
}

// DefaultCacheConfig returns default cache configuration.
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:       true,
		Directory:     ".qtest/mutation-cache",
		TTL:           24 * time.Hour,
		MaxEntries:    1000,
		PersistToDisk: true,
	}
}

// CacheEntry represents a cached mutation result.
type CacheEntry struct {
	// Key is the cache key (hash of source + test content)
	Key string `json:"key"`
	// SourceFile is the source file path
	SourceFile string `json:"sourceFile"`
	// TestFile is the test file path
	TestFile string `json:"testFile"`
	// SourceHash is the hash of the source file content
	SourceHash string `json:"sourceHash"`
	// TestHash is the hash of the test file content
	TestHash string `json:"testHash"`
	// Result is the cached mutation result
	Result *Result `json:"result"`
	// CreatedAt is when the entry was created
	CreatedAt time.Time `json:"createdAt"`
	// ExpiresAt is when the entry expires
	ExpiresAt time.Time `json:"expiresAt"`
	// Hits is the number of times this entry was retrieved
	Hits int `json:"hits"`
}

// NewCache creates a new mutation cache.
func NewCache(config *CacheConfig) (*Cache, error) {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &Cache{
		config:    config,
		entries:   make(map[string]*CacheEntry),
		storePath: filepath.Join(config.Directory, "cache.json"),
	}

	if config.Enabled && config.PersistToDisk {
		// Ensure directory exists
		if err := os.MkdirAll(config.Directory, 0755); err != nil {
			return nil, fmt.Errorf("failed to create cache directory: %w", err)
		}

		// Load existing cache
		if err := cache.load(); err != nil {
			// Non-fatal - start with empty cache
			cache.entries = make(map[string]*CacheEntry)
		}
	}

	return cache, nil
}

// Get retrieves a cached result if it exists and is valid.
func (c *Cache) Get(sourceFile, testFile string) (*Result, bool) {
	if !c.config.Enabled {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	key, err := c.generateKey(sourceFile, testFile)
	if err != nil {
		return nil, false
	}

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	// Validate hashes still match
	sourceHash, err := hashFile(sourceFile)
	if err != nil || sourceHash != entry.SourceHash {
		return nil, false
	}

	testHash, err := hashFile(testFile)
	if err != nil || testHash != entry.TestHash {
		return nil, false
	}

	// Update hit count (requires write lock, but we're in read lock)
	// We'll update asynchronously
	go func() {
		c.mu.Lock()
		if e, ok := c.entries[key]; ok {
			e.Hits++
		}
		c.mu.Unlock()
	}()

	return entry.Result, true
}

// Set stores a mutation result in the cache.
func (c *Cache) Set(sourceFile, testFile string, result *Result) error {
	if !c.config.Enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Enforce max entries
	if len(c.entries) >= c.config.MaxEntries {
		c.evictOldest()
	}

	key, err := c.generateKey(sourceFile, testFile)
	if err != nil {
		return fmt.Errorf("failed to generate cache key: %w", err)
	}

	sourceHash, err := hashFile(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to hash source file: %w", err)
	}

	testHash, err := hashFile(testFile)
	if err != nil {
		return fmt.Errorf("failed to hash test file: %w", err)
	}

	entry := &CacheEntry{
		Key:        key,
		SourceFile: sourceFile,
		TestFile:   testFile,
		SourceHash: sourceHash,
		TestHash:   testHash,
		Result:     result,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(c.config.TTL),
		Hits:       0,
	}

	c.entries[key] = entry

	// Persist if configured
	if c.config.PersistToDisk {
		return c.save()
	}

	return nil
}

// Invalidate removes a specific entry from the cache.
func (c *Cache) Invalidate(sourceFile, testFile string) {
	if !c.config.Enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key, err := c.generateKey(sourceFile, testFile)
	if err != nil {
		return
	}

	delete(c.entries, key)

	if c.config.PersistToDisk {
		c.save()
	}
}

// InvalidateFile removes all entries related to a file.
func (c *Cache) InvalidateFile(filePath string) {
	if !c.config.Enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.entries {
		if entry.SourceFile == filePath || entry.TestFile == filePath {
			delete(c.entries, key)
		}
	}

	if c.config.PersistToDisk {
		c.save()
	}
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)

	if c.config.PersistToDisk {
		// Remove cache file
		os.Remove(c.storePath)
	}

	return nil
}

// Cleanup removes expired entries.
func (c *Cache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			removed++
		}
	}

	if removed > 0 && c.config.PersistToDisk {
		c.save()
	}

	return removed
}

// Stats returns cache statistics.
func (c *Cache) Stats() *CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &CacheStats{
		TotalEntries: len(c.entries),
		Enabled:      c.config.Enabled,
	}

	now := time.Now()
	for _, entry := range c.entries {
		stats.TotalHits += entry.Hits
		if now.After(entry.ExpiresAt) {
			stats.ExpiredEntries++
		}
	}

	return stats
}

// CacheStats contains cache statistics.
type CacheStats struct {
	TotalEntries   int  `json:"totalEntries"`
	ExpiredEntries int  `json:"expiredEntries"`
	TotalHits      int  `json:"totalHits"`
	Enabled        bool `json:"enabled"`
}

// generateKey creates a cache key from source and test file paths.
func (c *Cache) generateKey(sourceFile, testFile string) (string, error) {
	// Use absolute paths for consistency
	absSource, err := filepath.Abs(sourceFile)
	if err != nil {
		absSource = sourceFile
	}
	absTest, err := filepath.Abs(testFile)
	if err != nil {
		absTest = testFile
	}

	combined := absSource + "|" + absTest
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:16]), nil // Use first 16 bytes for shorter keys
}

// evictOldest removes the oldest/least used entries.
func (c *Cache) evictOldest() {
	// Simple LRU: remove entries with lowest hit count and oldest creation time
	var oldestKey string
	var oldestTime time.Time
	minHits := int(^uint(0) >> 1) // Max int

	for key, entry := range c.entries {
		if entry.Hits < minHits || (entry.Hits == minHits && entry.CreatedAt.Before(oldestTime)) {
			oldestKey = key
			oldestTime = entry.CreatedAt
			minHits = entry.Hits
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// save persists the cache to disk.
func (c *Cache) save() error {
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	return os.WriteFile(c.storePath, data, 0644)
}

// load reads the cache from disk.
func (c *Cache) load() error {
	data, err := os.ReadFile(c.storePath)
	if err != nil {
		return err
	}

	entries := make(map[string]*CacheEntry)
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to unmarshal cache: %w", err)
	}

	c.entries = entries
	return nil
}

// hashFile computes SHA256 hash of a file's contents.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// CachedRunner wraps a mutation tool with caching.
type CachedRunner struct {
	tool  Tool
	cache *Cache
}

// NewCachedRunner creates a cached mutation runner.
func NewCachedRunner(tool Tool, cache *Cache) *CachedRunner {
	return &CachedRunner{
		tool:  tool,
		cache: cache,
	}
}

// Run runs mutation testing with caching.
func (r *CachedRunner) Run(ctx context.Context, sourceFile, testFile string, config MutationConfig) (*Result, error) {
	// Check cache first
	if cached, found := r.cache.Get(sourceFile, testFile); found {
		return cached, nil
	}

	// Run actual mutation testing
	result, err := r.tool.Run(ctx, sourceFile, testFile, config)
	if err != nil {
		return nil, err
	}

	// Cache successful results
	if result.Error == "" {
		r.cache.Set(sourceFile, testFile, result)
	}

	return result, nil
}

// Name returns the name of the underlying tool.
func (r *CachedRunner) Name() string {
	return r.tool.Name() + " (cached)"
}

// IsAvailable checks if the underlying tool is available.
func (r *CachedRunner) IsAvailable(ctx context.Context) bool {
	return r.tool.IsAvailable(ctx)
}
