package llm

import (
	"context"
	"testing"
	"time"
)

func TestNewMemoryCache(t *testing.T) {
	// Test with defaults
	c := NewMemoryCache(0, 0)
	if c == nil {
		t.Fatal("NewMemoryCache returned nil")
	}
	if c.maxSize != 1000 {
		t.Errorf("default maxSize = %d, want 1000", c.maxSize)
	}
	if c.ttl != 24*time.Hour {
		t.Errorf("default ttl = %v, want 24h", c.ttl)
	}

	// Test with custom values
	c2 := NewMemoryCache(500, 1*time.Hour)
	if c2.maxSize != 500 {
		t.Errorf("maxSize = %d, want 500", c2.maxSize)
	}
	if c2.ttl != 1*time.Hour {
		t.Errorf("ttl = %v, want 1h", c2.ttl)
	}
}

func TestMemoryCache_SetAndGet(t *testing.T) {
	c := NewMemoryCache(100, 1*time.Hour)
	ctx := context.Background()

	resp := &Response{
		Content:      "test response",
		Model:        "test-model",
		Provider:     ProviderOllama,
		InputTokens:  10,
		OutputTokens: 20,
	}

	// Set
	err := c.Set(ctx, "key1", resp, 0)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	// Get - should hit
	cached, ok := c.Get(ctx, "key1")
	if !ok {
		t.Fatal("Get() returned false, want true")
	}
	if cached.Content != resp.Content {
		t.Errorf("cached.Content = %s, want %s", cached.Content, resp.Content)
	}

	// Get non-existent - should miss
	_, ok = c.Get(ctx, "nonexistent")
	if ok {
		t.Error("Get(nonexistent) returned true, want false")
	}
}

func TestMemoryCache_Expiration(t *testing.T) {
	c := NewMemoryCache(100, 1*time.Hour)
	ctx := context.Background()

	resp := &Response{Content: "test"}

	// Set with very short TTL
	err := c.Set(ctx, "expiring", resp, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Should be expired
	_, ok := c.Get(ctx, "expiring")
	if ok {
		t.Error("Get() should return false for expired entry")
	}
}

func TestMemoryCache_Eviction(t *testing.T) {
	c := NewMemoryCache(3, 1*time.Hour)
	ctx := context.Background()

	// Fill cache to capacity
	for i := 0; i < 3; i++ {
		c.Set(ctx, string(rune('a'+i)), &Response{Content: string(rune('a' + i))}, 0)
		time.Sleep(1 * time.Millisecond) // Ensure different expiry times
	}

	// Add one more - should evict oldest
	c.Set(ctx, "d", &Response{Content: "d"}, 0)

	// Check size
	stats := c.Stats()
	if stats.Size != 3 {
		t.Errorf("Size = %d, want 3", stats.Size)
	}
}

func TestMemoryCache_Stats(t *testing.T) {
	c := NewMemoryCache(100, 1*time.Hour)
	ctx := context.Background()

	// Initial stats
	stats := c.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Size != 0 {
		t.Errorf("initial stats should be zeros, got %+v", stats)
	}

	// Add entry
	c.Set(ctx, "key1", &Response{Content: "test"}, 0)

	// Hit
	c.Get(ctx, "key1")
	stats = c.Stats()
	if stats.Hits != 1 {
		t.Errorf("Hits = %d, want 1", stats.Hits)
	}

	// Miss
	c.Get(ctx, "nonexistent")
	stats = c.Stats()
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}

	if stats.Size != 1 {
		t.Errorf("Size = %d, want 1", stats.Size)
	}
}

func TestGenerateCacheKey(t *testing.T) {
	req1 := &Request{
		Tier:        Tier1,
		System:      "test system",
		Messages:    []Message{{Role: "user", Content: "hello"}},
		Temperature: 0.5,
	}

	req2 := &Request{
		Tier:        Tier1,
		System:      "test system",
		Messages:    []Message{{Role: "user", Content: "hello"}},
		Temperature: 0.5,
	}

	req3 := &Request{
		Tier:        Tier1,
		System:      "test system",
		Messages:    []Message{{Role: "user", Content: "different"}},
		Temperature: 0.5,
	}

	key1 := GenerateCacheKey(req1)
	key2 := GenerateCacheKey(req2)
	key3 := GenerateCacheKey(req3)

	// Same request should produce same key
	if key1 != key2 {
		t.Errorf("identical requests should produce same key")
	}

	// Different request should produce different key
	if key1 == key3 {
		t.Errorf("different requests should produce different keys")
	}

	// Key should be a hex string of correct length (SHA256 = 64 hex chars)
	if len(key1) != 64 {
		t.Errorf("key length = %d, want 64", len(key1))
	}
}

func TestNullCache(t *testing.T) {
	c := &NullCache{}
	ctx := context.Background()

	// Set should not error
	err := c.Set(ctx, "key", &Response{Content: "test"}, 0)
	if err != nil {
		t.Errorf("NullCache.Set() error: %v", err)
	}

	// Get should always return false
	_, ok := c.Get(ctx, "key")
	if ok {
		t.Error("NullCache.Get() should always return false")
	}

	// Stats should be empty
	stats := c.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Size != 0 {
		t.Errorf("NullCache.Stats() should return zeros, got %+v", stats)
	}
}

func TestCreateCache(t *testing.T) {
	tests := []struct {
		cacheType string
		wantType  string
	}{
		{"memory", "*llm.MemoryCache"},
		{"none", "*llm.NullCache"},
		{"", "*llm.NullCache"},
		{"unknown", "*llm.MemoryCache"}, // Falls back to memory
	}

	for _, tt := range tests {
		c := CreateCache(tt.cacheType, 100, 1*time.Hour)
		if c == nil {
			t.Errorf("CreateCache(%s) returned nil", tt.cacheType)
			continue
		}
	}
}

func TestCachedRouter(t *testing.T) {
	// Create a mock cache
	cache := NewMemoryCache(100, 1*time.Hour)

	// Create cached router (router is nil, but we're just testing caching)
	cr := NewCachedRouter(nil, cache, 1*time.Hour)

	if cr.router != nil {
		t.Error("router should be nil in this test")
	}

	if cr.cache != cache {
		t.Error("cache not set correctly")
	}

	// Test CacheStats
	stats := cr.CacheStats()
	if stats.Size != 0 {
		t.Errorf("initial CacheStats.Size = %d, want 0", stats.Size)
	}

	// Test GetRouter
	if cr.GetRouter() != nil {
		t.Error("GetRouter() should return nil")
	}
}

func TestCachedRouter_DefaultTTL(t *testing.T) {
	cache := NewMemoryCache(100, 1*time.Hour)

	// Test with zero TTL - should default to 24h
	cr := NewCachedRouter(nil, cache, 0)
	if cr.ttl != 24*time.Hour {
		t.Errorf("default ttl = %v, want 24h", cr.ttl)
	}
}

// Test concurrent access
func TestMemoryCache_Concurrent(t *testing.T) {
	c := NewMemoryCache(1000, 1*time.Hour)
	ctx := context.Background()

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			c.Set(ctx, string(rune(i)), &Response{Content: "test"}, 0)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			c.Get(ctx, string(rune(i)))
		}
		done <- true
	}()

	// Wait for both
	<-done
	<-done

	// Should not panic or deadlock
}

// =============================================================================
// Additional Edge Case Tests
// =============================================================================

func TestGenerateCacheKey_EmptyRequest(t *testing.T) {
	req := &Request{}
	key := GenerateCacheKey(req)

	if key == "" {
		t.Error("GenerateCacheKey() should return non-empty key even for empty request")
	}
	if len(key) != 64 {
		t.Errorf("key length = %d, want 64", len(key))
	}
}

func TestGenerateCacheKey_DifferentTiers(t *testing.T) {
	req1 := &Request{Tier: Tier1, System: "test"}
	req2 := &Request{Tier: Tier2, System: "test"}
	req3 := &Request{Tier: Tier3, System: "test"}

	key1 := GenerateCacheKey(req1)
	key2 := GenerateCacheKey(req2)
	key3 := GenerateCacheKey(req3)

	if key1 == key2 || key2 == key3 || key1 == key3 {
		t.Error("Different tiers should produce different cache keys")
	}
}

func TestGenerateCacheKey_DifferentTemperatures(t *testing.T) {
	req1 := &Request{Tier: Tier1, Temperature: 0.0}
	req2 := &Request{Tier: Tier1, Temperature: 0.5}
	req3 := &Request{Tier: Tier1, Temperature: 1.0}

	key1 := GenerateCacheKey(req1)
	key2 := GenerateCacheKey(req2)
	key3 := GenerateCacheKey(req3)

	if key1 == key2 || key2 == key3 {
		t.Error("Different temperatures should produce different cache keys")
	}
}

func TestGenerateCacheKey_MultipleMessages(t *testing.T) {
	req1 := &Request{
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	}
	req2 := &Request{
		Messages: []Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}

	key1 := GenerateCacheKey(req1)
	key2 := GenerateCacheKey(req2)

	if key1 == key2 {
		t.Error("Different message counts should produce different cache keys")
	}
}

func TestMemoryCache_SetWithCustomTTL(t *testing.T) {
	c := NewMemoryCache(100, 24*time.Hour)
	ctx := context.Background()

	resp := &Response{Content: "test"}

	// Set with very short TTL
	err := c.Set(ctx, "short-ttl", resp, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	// Should be available immediately
	_, ok := c.Get(ctx, "short-ttl")
	if !ok {
		t.Error("Entry should be available immediately after set")
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Should be expired now
	_, ok = c.Get(ctx, "short-ttl")
	if ok {
		t.Error("Entry should be expired after TTL")
	}
}

func TestMemoryCache_LongKeyPreview(t *testing.T) {
	c := NewMemoryCache(100, 1*time.Hour)
	ctx := context.Background()

	// Use a very long key
	longKey := "this-is-a-very-long-cache-key-that-exceeds-16-characters"
	resp := &Response{Content: "test"}

	// Should not panic and should work correctly
	err := c.Set(ctx, longKey, resp, 0)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	cached, ok := c.Get(ctx, longKey)
	if !ok {
		t.Fatal("Get() returned false, want true")
	}
	if cached.Content != "test" {
		t.Errorf("Content = %s, want test", cached.Content)
	}
}

func TestMemoryCache_ShortKeyPreview(t *testing.T) {
	c := NewMemoryCache(100, 1*time.Hour)
	ctx := context.Background()

	// Use a short key (less than 16 chars)
	shortKey := "short"
	resp := &Response{Content: "test"}

	err := c.Set(ctx, shortKey, resp, 0)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	cached, ok := c.Get(ctx, shortKey)
	if !ok {
		t.Fatal("Get() returned false, want true")
	}
	if cached.Content != "test" {
		t.Errorf("Content = %s, want test", cached.Content)
	}
}

func TestCacheStats_Fields(t *testing.T) {
	stats := CacheStats{
		Hits:   100,
		Misses: 50,
		Size:   75,
	}

	if stats.Hits != 100 {
		t.Errorf("Hits = %d, want 100", stats.Hits)
	}
	if stats.Misses != 50 {
		t.Errorf("Misses = %d, want 50", stats.Misses)
	}
	if stats.Size != 75 {
		t.Errorf("Size = %d, want 75", stats.Size)
	}
}

func TestNewRedisCache_ReturnsNil(t *testing.T) {
	// NewRedisCache currently returns nil (not implemented)
	cache := NewRedisCache("localhost:6379", "", 0, 1*time.Hour)
	if cache != nil {
		t.Error("NewRedisCache() should return nil (not implemented)")
	}
}

func TestCreateCache_AllTypes(t *testing.T) {
	tests := []struct {
		cacheType string
		isNull    bool
	}{
		{"memory", false},
		{"none", true},
		{"", true},
		{"unknown", false}, // Falls back to memory
		{"MEMORY", false},  // Case sensitivity test - falls back to memory
	}

	for _, tt := range tests {
		t.Run(tt.cacheType, func(t *testing.T) {
			c := CreateCache(tt.cacheType, 100, 1*time.Hour)
			if c == nil {
				t.Error("CreateCache should never return nil")
				return
			}

			// Check if it's a NullCache by testing behavior
			ctx := context.Background()
			c.Set(ctx, "test", &Response{Content: "test"}, 0)
			_, ok := c.Get(ctx, "test")

			if tt.isNull && ok {
				t.Error("Expected NullCache but got different implementation")
			}
		})
	}
}
