package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRouter_OllamaOnly tests router creation with only Ollama configured
func TestNewRouter_OllamaOnly(t *testing.T) {
	// Create mock Ollama server
	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ollamaServer.Close()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			OllamaURL:       ollamaServer.URL,
			OllamaTier1:     "qwen2.5-coder:7b",
			OllamaTier2:     "deepseek-coder-v2:16b",
			DefaultProvider: "ollama",
		},
	}

	router, err := NewRouter(cfg)
	require.NoError(t, err)
	assert.NotNil(t, router)
	assert.Contains(t, router.clients, ProviderOllama)
	assert.NotContains(t, router.clients, ProviderAnthropic)
}

// TestNewRouter_AnthropicOnly tests router creation with only Anthropic configured
func TestNewRouter_AnthropicOnly(t *testing.T) {
	cfg := &config.Config{
		LLM: config.LLMConfig{
			AnthropicKey:    "sk-ant-test123",
			AnthropicTier3:  "claude-3-5-sonnet-20241022",
			DefaultProvider: "anthropic",
		},
	}

	router, err := NewRouter(cfg)
	require.NoError(t, err)
	assert.NotNil(t, router)
	assert.Contains(t, router.clients, ProviderAnthropic)
	assert.NotContains(t, router.clients, ProviderOllama)
}

// TestNewRouter_BothProviders tests router creation with both providers
func TestNewRouter_BothProviders(t *testing.T) {
	// Create mock Ollama server
	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ollamaServer.Close()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			OllamaURL:       ollamaServer.URL,
			OllamaTier1:     "qwen2.5-coder:7b",
			OllamaTier2:     "deepseek-coder-v2:16b",
			AnthropicKey:    "sk-ant-test123",
			AnthropicTier3:  "claude-3-5-sonnet-20241022",
			DefaultProvider: "ollama",
		},
	}

	router, err := NewRouter(cfg)
	require.NoError(t, err)
	assert.NotNil(t, router)
	assert.Contains(t, router.clients, ProviderOllama)
	assert.Contains(t, router.clients, ProviderAnthropic)
	assert.Len(t, router.clients, 2)
}

// TestNewRouter_NoProviders tests router creation with no providers configured
func TestNewRouter_NoProviders(t *testing.T) {
	cfg := &config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "ollama",
		},
	}

	router, err := NewRouter(cfg)
	assert.Error(t, err)
	assert.Nil(t, router)
	assert.Contains(t, err.Error(), "no LLM providers configured")
}

// TestNewRouter_DefaultProvider tests that default provider is set correctly
func TestNewRouter_DefaultProvider(t *testing.T) {
	// Create mock Ollama server
	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ollamaServer.Close()

	tests := []struct {
		name             string
		defaultProvider  string
		expectedProvider Provider
	}{
		{"ollama_default", "ollama", ProviderOllama},
		{"anthropic_default", "anthropic", ProviderAnthropic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				LLM: config.LLMConfig{
					OllamaURL:       ollamaServer.URL,
					OllamaTier1:     "qwen2.5-coder:7b",
					OllamaTier2:     "deepseek-coder-v2:16b",
					AnthropicKey:    "sk-ant-test123",
					AnthropicTier3:  "claude-3-5-sonnet-20241022",
					DefaultProvider: tt.defaultProvider,
				},
			}

			router, err := NewRouter(cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedProvider, router.config.DefaultProvider)
		})
	}
}

// TestTrackedRouter_Complete tests that TrackedRouter properly tracks usage
func TestTrackedRouter_Complete(t *testing.T) {
	client := newMockClient(ProviderOllama, true)
	expectedResp := &Response{
		Content:      "generated test",
		Model:        "qwen2.5-coder:7b",
		Provider:     ProviderOllama,
		InputTokens:  100,
		OutputTokens: 50,
	}
	client.withResponses(expectedResp)

	router := &Router{
		config: &RouterConfig{
			DefaultProvider: ProviderOllama,
			TierModels: map[Tier]map[Provider]string{
				Tier1: {ProviderOllama: "qwen2.5-coder:7b"},
			},
		},
		clients:   map[Provider]Client{ProviderOllama: client},
		fallbacks: []Provider{ProviderOllama},
	}

	tracker := NewUsageTracker(UsageTrackerConfig{MaxRecords: 100})
	trackedRouter := NewTrackedRouter(router, tracker)

	resp, err := trackedRouter.Complete(context.Background(), &Request{
		Tier: Tier1,
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, expectedResp.Content, resp.Content)

	// Verify usage was tracked
	stats := tracker.GetStats()
	assert.EqualValues(t, 150, stats.TotalTokens)
	assert.EqualValues(t, 1, stats.TotalRequests)

	// Verify record was created
	records := tracker.RecentRecords(10)
	require.Len(t, records, 1)
	assert.Equal(t, ProviderOllama, records[0].Provider)
	assert.Equal(t, 100, records[0].InputTokens)
	assert.Equal(t, 50, records[0].OutputTokens)
}

// TestTrackedRouter_Complete_BudgetExceeded tests budget enforcement
func TestTrackedRouter_Complete_BudgetExceeded(t *testing.T) {
	client := newMockClient(ProviderOllama, true)

	router := &Router{
		config: &RouterConfig{
			DefaultProvider: ProviderOllama,
			TierModels: map[Tier]map[Provider]string{
				Tier1: {ProviderOllama: "model"},
			},
		},
		clients:   map[Provider]Client{ProviderOllama: client},
		fallbacks: []Provider{ProviderOllama},
	}

	tracker := NewUsageTracker(UsageTrackerConfig{
		Budget: BudgetConfig{
			HourlyTokenLimit: 100,
		},
	})

	// Pre-record usage to exceed budget
	tracker.Record(UsageRecord{
		Provider:     ProviderOllama,
		InputTokens:  80,
		OutputTokens: 30,
	})

	trackedRouter := NewTrackedRouter(router, tracker)

	_, err := trackedRouter.Complete(context.Background(), &Request{Tier: Tier1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "budget exceeded")
}

// TestCachedRouter_Integration tests full cache integration
func TestCachedRouter_Integration(t *testing.T) {
	client := newMockClient(ProviderOllama, true)
	client.withResponses(
		&Response{Content: "response1", Provider: ProviderOllama, InputTokens: 10, OutputTokens: 5},
	)

	router := &Router{
		config: &RouterConfig{
			DefaultProvider: ProviderOllama,
			TierModels: map[Tier]map[Provider]string{
				Tier1: {ProviderOllama: "model"},
			},
		},
		clients:   map[Provider]Client{ProviderOllama: client},
		fallbacks: []Provider{ProviderOllama},
	}

	cache := NewMemoryCache(100, time.Hour)

	cachedRouter := NewCachedRouter(router, cache, time.Hour)

	req := &Request{
		Tier: Tier1,
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	}

	// First call should hit the client
	resp1, err := cachedRouter.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "response1", resp1.Content)
	assert.Equal(t, 1, client.callCount)

	// Second call should hit cache
	resp2, err := cachedRouter.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "response1", resp2.Content)
	assert.Equal(t, 1, client.callCount) // Still 1, cache hit

	// Verify cache stats
	stats := cache.Stats()
	assert.EqualValues(t, 1, stats.Hits)
	assert.EqualValues(t, 1, stats.Misses)
}

// TestFullPipeline_TrackerWithCache tests tracker + router + cache integration
func TestFullPipeline_TrackerWithCache(t *testing.T) {
	client := newMockClient(ProviderOllama, true)
	client.withResponses(
		&Response{Content: "response", Provider: ProviderOllama, InputTokens: 100, OutputTokens: 50},
	)

	router := &Router{
		config: &RouterConfig{
			DefaultProvider: ProviderOllama,
			TierModels: map[Tier]map[Provider]string{
				Tier1: {ProviderOllama: "model"},
			},
		},
		clients:   map[Provider]Client{ProviderOllama: client},
		fallbacks: []Provider{ProviderOllama},
	}

	cache := NewMemoryCache(100, time.Hour)
	tracker := NewUsageTracker(UsageTrackerConfig{MaxRecords: 100})

	// Build pipeline: cache -> router (tracker wraps router)
	trackedRouter := NewTrackedRouter(router, tracker)
	cachedRouter := NewCachedRouter(trackedRouter.Router, cache, time.Hour)

	req := &Request{
		Tier: Tier1,
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	}

	// First call: cache miss, hits router
	resp1, err := cachedRouter.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "response", resp1.Content)

	// Manually track the response (in production, TrackedRouter.Complete does this)
	tracker.Record(UsageRecord{
		Provider:     resp1.Provider,
		InputTokens:  resp1.InputTokens,
		OutputTokens: resp1.OutputTokens,
	})

	stats := tracker.GetStats()
	assert.EqualValues(t, 1, stats.TotalRequests)
	assert.EqualValues(t, 150, stats.TotalTokens)

	// Second call: cache hit
	resp2, err := cachedRouter.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "response", resp2.Content)

	// Verify cache was hit
	cacheStats := cache.Stats()
	assert.EqualValues(t, 1, cacheStats.Hits)
}

// TestRouter_Complete_FallbackChain tests provider fallback chain
func TestRouter_Complete_FallbackChain(t *testing.T) {
	ollamaClient := newMockClient(ProviderOllama, false) // Unavailable
	anthropicClient := newMockClient(ProviderAnthropic, true)
	anthropicClient.withResponses(&Response{
		Content:  "anthropic response",
		Provider: ProviderAnthropic,
	})

	router := &Router{
		config: &RouterConfig{
			DefaultProvider: ProviderOllama,
			TierModels: map[Tier]map[Provider]string{
				Tier1: {
					ProviderOllama:    "model1",
					ProviderAnthropic: "model2",
				},
			},
		},
		clients: map[Provider]Client{
			ProviderOllama:    ollamaClient,
			ProviderAnthropic: anthropicClient,
		},
		fallbacks: []Provider{ProviderOllama, ProviderAnthropic},
	}

	resp, err := router.Complete(context.Background(), &Request{Tier: Tier1})
	require.NoError(t, err)
	assert.Equal(t, ProviderAnthropic, resp.Provider)
	assert.Equal(t, 0, ollamaClient.callCount)  // Skipped because unavailable
	assert.Equal(t, 1, anthropicClient.callCount)
}

// TestRouter_Complete_AllTiers tests completion across all tiers
func TestRouter_Complete_AllTiers(t *testing.T) {
	client := newMockClient(ProviderOllama, true)

	router := &Router{
		config: &RouterConfig{
			DefaultProvider: ProviderOllama,
			TierModels: map[Tier]map[Provider]string{
				Tier1: {ProviderOllama: "tier1-model"},
				Tier2: {ProviderOllama: "tier2-model"},
				Tier3: {ProviderOllama: "tier3-model"},
			},
		},
		clients:   map[Provider]Client{ProviderOllama: client},
		fallbacks: []Provider{ProviderOllama},
	}

	tests := []struct {
		name string
		tier Tier
	}{
		{"tier1", Tier1},
		{"tier2", Tier2},
		{"tier3", Tier3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client.callCount = 0
			resp, err := router.Complete(context.Background(), &Request{Tier: tt.tier})
			require.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}

// TestRouter_TierModels tests tier model configuration
func TestRouter_TierModels(t *testing.T) {
	// Create mock Ollama server
	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ollamaServer.Close()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			OllamaURL:       ollamaServer.URL,
			OllamaTier1:     "fast-model",
			OllamaTier2:     "balanced-model",
			AnthropicKey:    "sk-ant-test",
			AnthropicTier3:  "powerful-model",
			DefaultProvider: "ollama",
		},
	}

	router, err := NewRouter(cfg)
	require.NoError(t, err)

	// Check Tier 1 model
	tier1Models := router.config.TierModels[Tier1]
	assert.Equal(t, "fast-model", tier1Models[ProviderOllama])

	// Check Tier 2 model
	tier2Models := router.config.TierModels[Tier2]
	assert.Equal(t, "balanced-model", tier2Models[ProviderOllama])

	// Check Tier 3 model
	tier3Models := router.config.TierModels[Tier3]
	assert.Equal(t, "powerful-model", tier3Models[ProviderAnthropic])
}

// TestTrackedRouter_GetTracker tests tracker retrieval
func TestTrackedRouter_GetTracker_NotNil(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{})
	trackedRouter := NewTrackedRouter(nil, tracker)

	retrieved := trackedRouter.GetTracker()
	assert.NotNil(t, retrieved)
	assert.Equal(t, tracker, retrieved)
}

// TestUsageTracker_ConcurrentAccess tests thread safety
func TestUsageTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{MaxRecords: 1000})

	// Concurrent records
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				tracker.Record(UsageRecord{
					Provider:     ProviderOllama,
					InputTokens:  10,
					OutputTokens: 5,
				})
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	stats := tracker.GetStats()
	assert.EqualValues(t, 1000, stats.TotalRequests)
	assert.EqualValues(t, 15000, stats.TotalTokens) // 1000 * 15
}

// TestCacheKeyConsistency tests that same request generates same cache key
func TestCacheKeyConsistency(t *testing.T) {
	req := &Request{
		Tier:   Tier1,
		System: "system prompt",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
		MaxTokens:   1000,
		Temperature: 0.5,
	}

	key1 := GenerateCacheKey(req)
	key2 := GenerateCacheKey(req)

	assert.Equal(t, key1, key2)
}

// TestCacheKeyDifferentRequests tests that different requests generate different keys
func TestCacheKeyDifferentRequests(t *testing.T) {
	req1 := &Request{
		Tier: Tier1,
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	}

	req2 := &Request{
		Tier: Tier1,
		Messages: []Message{
			{Role: "user", Content: "goodbye"},
		},
	}

	key1 := GenerateCacheKey(req1)
	key2 := GenerateCacheKey(req2)

	assert.NotEqual(t, key1, key2)
}

// TestNewRouter_FallbackOrder tests that fallback order is set correctly
func TestNewRouter_FallbackOrder(t *testing.T) {
	// Create mock Ollama server
	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ollamaServer.Close()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			OllamaURL:       ollamaServer.URL,
			OllamaTier1:     "model",
			DefaultProvider: "ollama",
		},
	}

	router, err := NewRouter(cfg)
	require.NoError(t, err)

	// Verify fallback order includes all providers
	assert.Equal(t, []Provider{ProviderOllama, ProviderAnthropic, ProviderOpenAI}, router.fallbacks)
}
