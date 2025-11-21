package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/rs/zerolog/log"
)

// Retry configuration
const (
	defaultMaxRetries = 3
	initialBackoff    = 2 * time.Second
	maxBackoff        = 30 * time.Second
	backoffMultiplier = 2.0
)

// Router routes LLM requests to appropriate providers based on tier and availability
type Router struct {
	config    *RouterConfig
	clients   map[Provider]Client
	fallbacks []Provider // Fallback order
}

// NewRouter creates a new LLM router from config
func NewRouter(cfg *config.Config) (*Router, error) {
	r := &Router{
		clients:   make(map[Provider]Client),
		fallbacks: []Provider{ProviderOllama, ProviderAnthropic, ProviderOpenAI},
	}

	// Build router config from application config
	r.config = &RouterConfig{
		DefaultProvider: Provider(cfg.LLM.DefaultProvider),
		Providers:       make(map[Provider]ProviderConfig),
		TierModels:      make(map[Tier]map[Provider]string),
	}

	// Configure Ollama (always enabled if URL is set)
	if cfg.LLM.OllamaURL != "" {
		r.config.Providers[ProviderOllama] = ProviderConfig{
			Enabled: true,
			BaseURL: cfg.LLM.OllamaURL,
		}

		ollamaModels := map[Tier]string{
			Tier1: cfg.LLM.OllamaTier1,
			Tier2: cfg.LLM.OllamaTier2,
		}

		r.clients[ProviderOllama] = NewOllamaClient(cfg.LLM.OllamaURL, ollamaModels)

		// Add to tier models
		r.config.TierModels[Tier1] = map[Provider]string{
			ProviderOllama: cfg.LLM.OllamaTier1,
		}
		r.config.TierModels[Tier2] = map[Provider]string{
			ProviderOllama: cfg.LLM.OllamaTier2,
		}
	}

	// Configure Anthropic (for Tier 3 and fallback)
	if cfg.LLM.AnthropicKey != "" {
		r.config.Providers[ProviderAnthropic] = ProviderConfig{
			Enabled: true,
			APIKey:  cfg.LLM.AnthropicKey,
		}

		r.clients[ProviderAnthropic] = NewAnthropicClient(cfg.LLM.AnthropicKey, map[Tier]string{
			Tier1: "claude-3-haiku-20240307",
			Tier2: "claude-3-5-sonnet-20241022",
			Tier3: cfg.LLM.AnthropicTier3,
		})

		// Tier 3 uses Anthropic by default
		if r.config.TierModels[Tier3] == nil {
			r.config.TierModels[Tier3] = make(map[Provider]string)
		}
		r.config.TierModels[Tier3][ProviderAnthropic] = cfg.LLM.AnthropicTier3
	}

	// Validate at least one provider is configured
	if len(r.clients) == 0 {
		return nil, fmt.Errorf("no LLM providers configured")
	}

	return r, nil
}

// Complete sends a completion request, routing to appropriate provider with retry logic
func (r *Router) Complete(ctx context.Context, req *Request) (*Response, error) {
	// Get providers that support this tier
	providers := r.getProvidersForTier(req.Tier)
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available for tier %d", req.Tier)
	}

	// Try each provider in order
	var lastErr error
	for _, provider := range providers {
		client, ok := r.clients[provider]
		if !ok {
			continue
		}

		if !client.Available() {
			log.Debug().Str("provider", string(provider)).Msg("provider not available, trying next")
			continue
		}

		log.Debug().
			Str("provider", string(provider)).
			Int("tier", int(req.Tier)).
			Msg("routing request to provider")

		// Try with retries
		resp, err := r.completeWithRetry(ctx, client, provider, req)
		if err != nil {
			log.Warn().
				Err(err).
				Str("provider", string(provider)).
				Msg("provider failed after retries, trying next")
			lastErr = err
			continue
		}

		return resp, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
	}

	return nil, fmt.Errorf("no available providers for tier %d", req.Tier)
}

// completeWithRetry attempts completion with exponential backoff retry
func (r *Router) completeWithRetry(ctx context.Context, client Client, provider Provider, req *Request) (*Response, error) {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= defaultMaxRetries; attempt++ {
		if attempt > 0 {
			log.Debug().
				Str("provider", string(provider)).
				Int("attempt", attempt+1).
				Dur("backoff", backoff).
				Msg("retrying after backoff")

			// Wait with context awareness
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}

			// Increase backoff for next attempt
			backoff = time.Duration(float64(backoff) * backoffMultiplier)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		resp, err := client.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			log.Debug().
				Err(err).
				Str("provider", string(provider)).
				Msg("non-retryable error, stopping retries")
			return nil, err
		}

		log.Debug().
			Err(err).
			Str("provider", string(provider)).
			Int("attempt", attempt+1).
			Int("max_retries", defaultMaxRetries).
			Msg("retryable error occurred")
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isRetryableError determines if an error warrants a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network errors are retryable
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Timeout errors are retryable
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "EOF") {
		return true
	}

	// 5xx server errors are retryable
	if strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "server error") ||
		strings.Contains(errStr, "internal error") {
		return true
	}

	// Rate limiting is retryable
	if strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") {
		return true
	}

	// 4xx client errors are NOT retryable (except 429)
	if strings.Contains(errStr, "400") ||
		strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "404") {
		return false
	}

	// Default: retry unknown errors
	return true
}

// getProvidersForTier returns providers that can handle the given tier, in priority order
func (r *Router) getProvidersForTier(tier Tier) []Provider {
	providers := make([]Provider, 0)

	// Check configured tier models first
	if tierModels, ok := r.config.TierModels[tier]; ok {
		// Add default provider first if it supports this tier
		if _, hasDefault := tierModels[r.config.DefaultProvider]; hasDefault {
			providers = append(providers, r.config.DefaultProvider)
		}

		// Add other providers for this tier
		for provider := range tierModels {
			if provider != r.config.DefaultProvider {
				providers = append(providers, provider)
			}
		}
	}

	// Add fallbacks that aren't already in the list
	for _, fallback := range r.fallbacks {
		found := false
		for _, p := range providers {
			if p == fallback {
				found = true
				break
			}
		}
		if !found && r.clients[fallback] != nil {
			providers = append(providers, fallback)
		}
	}

	return providers
}

// HealthCheck verifies at least one provider is available
func (r *Router) HealthCheck() error {
	for provider, client := range r.clients {
		if client.Available() {
			log.Debug().Str("provider", string(provider)).Msg("provider available")
			return nil
		}
	}
	return fmt.Errorf("no LLM providers available")
}
