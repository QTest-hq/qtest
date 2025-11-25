package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/resilience"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRouter creates a Router with all required fields for testing
func newTestRouter(config *RouterConfig, clients map[Provider]Client, fallbacks []Provider) *Router {
	return &Router{
		config:         config,
		clients:        clients,
		fallbacks:      fallbacks,
		circuitBreaker: resilience.NewCircuitBreakerManager(),
	}
}

// mockClient is a test double for Client interface
type mockClient struct {
	name      Provider
	available bool
	responses []*Response
	errors    []error
	callCount int
}

func newMockClient(name Provider, available bool) *mockClient {
	return &mockClient{
		name:      name,
		available: available,
	}
}

func (m *mockClient) Name() Provider {
	return m.name
}

func (m *mockClient) Available() bool {
	return m.available
}

func (m *mockClient) Complete(ctx context.Context, req *Request) (*Response, error) {
	m.callCount++

	// Check context cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	idx := m.callCount - 1

	if idx < len(m.errors) && m.errors[idx] != nil {
		return nil, m.errors[idx]
	}

	if idx < len(m.responses) {
		return m.responses[idx], nil
	}

	// Default response
	return &Response{
		Content:  "test response",
		Model:    "test-model",
		Provider: m.name,
	}, nil
}

func (m *mockClient) withResponses(responses ...*Response) *mockClient {
	m.responses = responses
	return m
}

func (m *mockClient) withErrors(errs ...error) *mockClient {
	m.errors = errs
	return m
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil_error", nil, false},
		{"timeout", errors.New("request timeout"), true},
		{"deadline_exceeded", errors.New("deadline exceeded"), true},
		{"connection_refused", errors.New("connection refused"), true},
		{"connection_reset", errors.New("connection reset by peer"), true},
		{"eof", errors.New("unexpected EOF"), true},
		{"500_error", errors.New("server returned status 500"), true},
		{"502_error", errors.New("bad gateway 502"), true},
		{"503_error", errors.New("service unavailable 503"), true},
		{"504_error", errors.New("gateway timeout 504"), true},
		{"server_error", errors.New("internal server error"), true},
		{"rate_limit_429", errors.New("status 429: too many requests"), true},
		{"rate_limit_text", errors.New("rate limit exceeded"), true},
		{"400_bad_request", errors.New("400 bad request"), false},
		{"401_unauthorized", errors.New("401 unauthorized"), false},
		{"403_forbidden", errors.New("403 forbidden"), false},
		{"404_not_found", errors.New("404 not found"), false},
		{"unknown_error", errors.New("some unknown error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRouter_Complete_Success(t *testing.T) {
	client := newMockClient(ProviderOllama, true)
	expectedResp := &Response{
		Content:  "generated test",
		Model:    "qwen2.5-coder:7b",
		Provider: ProviderOllama,
	}
	client.withResponses(expectedResp)

	router := newTestRouter(
		&RouterConfig{
			DefaultProvider: ProviderOllama,
			TierModels: map[Tier]map[Provider]string{
				Tier1: {ProviderOllama: "qwen2.5-coder:7b"},
			},
		},
		map[Provider]Client{ProviderOllama: client},
		[]Provider{ProviderOllama},
	)

	resp, err := router.Complete(context.Background(), &Request{Tier: Tier1})
	require.NoError(t, err)
	assert.Equal(t, expectedResp.Content, resp.Content)
	assert.Equal(t, 1, client.callCount)
}

func TestRouter_Complete_ProviderUnavailable_Fallback(t *testing.T) {
	unavailableClient := newMockClient(ProviderOllama, false)
	availableClient := newMockClient(ProviderAnthropic, true)

	router := newTestRouter(
		&RouterConfig{
			DefaultProvider: ProviderOllama,
			TierModels: map[Tier]map[Provider]string{
				Tier1: {
					ProviderOllama:    "model1",
					ProviderAnthropic: "model2",
				},
			},
		},
		map[Provider]Client{
			ProviderOllama:    unavailableClient,
			ProviderAnthropic: availableClient,
		},
		[]Provider{ProviderOllama, ProviderAnthropic},
	)

	resp, err := router.Complete(context.Background(), &Request{Tier: Tier1})
	require.NoError(t, err)
	assert.Equal(t, ProviderAnthropic, resp.Provider)
	assert.Equal(t, 0, unavailableClient.callCount) // Should skip unavailable
	assert.Equal(t, 1, availableClient.callCount)
}

func TestRouter_Complete_NoProviders(t *testing.T) {
	router := newTestRouter(
		&RouterConfig{
			TierModels: map[Tier]map[Provider]string{},
		},
		map[Provider]Client{},
		[]Provider{},
	)

	_, err := router.Complete(context.Background(), &Request{Tier: Tier1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no providers available")
}

func TestRouter_Complete_AllProvidersFail(t *testing.T) {
	client := newMockClient(ProviderOllama, true)
	// All calls fail with non-retryable error
	client.withErrors(
		errors.New("401 unauthorized"),
	)

	router := newTestRouter(
		&RouterConfig{
			DefaultProvider: ProviderOllama,
			TierModels: map[Tier]map[Provider]string{
				Tier1: {ProviderOllama: "model"},
			},
		},
		map[Provider]Client{ProviderOllama: client},
		[]Provider{ProviderOllama},
	)

	_, err := router.Complete(context.Background(), &Request{Tier: Tier1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all providers failed")
}

func TestRouter_CompleteWithRetry_SuccessOnRetry(t *testing.T) {
	client := newMockClient(ProviderOllama, true)
	// First call fails with retryable error, second succeeds
	client.withErrors(
		errors.New("timeout"),
		nil, // Success
	)

	router := newTestRouter(
		&RouterConfig{},
		map[Provider]Client{ProviderOllama: client},
		[]Provider{ProviderOllama},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := router.completeWithRetry(ctx, client, ProviderOllama, &Request{Tier: Tier1})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 2, client.callCount) // First failed, second succeeded
}

func TestRouter_CompleteWithRetry_NonRetryableError(t *testing.T) {
	client := newMockClient(ProviderOllama, true)
	client.withErrors(errors.New("401 unauthorized"))

	router := newTestRouter(
		&RouterConfig{},
		map[Provider]Client{ProviderOllama: client},
		[]Provider{ProviderOllama},
	)

	_, err := router.completeWithRetry(context.Background(), client, ProviderOllama, &Request{Tier: Tier1})
	assert.Error(t, err)
	assert.Equal(t, 1, client.callCount) // Should not retry
}

func TestRouter_CompleteWithRetry_MaxRetriesExceeded(t *testing.T) {
	client := newMockClient(ProviderOllama, true)
	// All retries fail
	client.withErrors(
		errors.New("timeout"),
		errors.New("timeout"),
		errors.New("timeout"),
		errors.New("timeout"),
		errors.New("timeout"), // More than max retries
	)

	router := newTestRouter(
		&RouterConfig{},
		map[Provider]Client{ProviderOllama: client},
		[]Provider{ProviderOllama},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err := router.completeWithRetry(ctx, client, ProviderOllama, &Request{Tier: Tier1})
	assert.Error(t, err)
	// Circuit breaker may open before max retries due to failure threshold
	assert.True(t, client.callCount >= 3, "Expected at least 3 attempts")
}

func TestRouter_CompleteWithRetry_ContextCancellation(t *testing.T) {
	client := newMockClient(ProviderOllama, true)
	client.withErrors(errors.New("timeout"))

	router := newTestRouter(
		&RouterConfig{},
		map[Provider]Client{ProviderOllama: client},
		[]Provider{ProviderOllama},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := router.completeWithRetry(ctx, client, ProviderOllama, &Request{Tier: Tier1})
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestRouter_GetProvidersForTier(t *testing.T) {
	ollamaClient := newMockClient(ProviderOllama, true)
	anthropicClient := newMockClient(ProviderAnthropic, true)

	router := newTestRouter(
		&RouterConfig{
			DefaultProvider: ProviderOllama,
			TierModels: map[Tier]map[Provider]string{
				Tier1: {ProviderOllama: "model1"},
				Tier2: {
					ProviderOllama:    "model2",
					ProviderAnthropic: "model3",
				},
				Tier3: {ProviderAnthropic: "model4"},
			},
		},
		map[Provider]Client{
			ProviderOllama:    ollamaClient,
			ProviderAnthropic: anthropicClient,
		},
		[]Provider{ProviderOllama, ProviderAnthropic},
	)

	t.Run("tier1_returns_ollama_first", func(t *testing.T) {
		providers := router.getProvidersForTier(Tier1)
		assert.Contains(t, providers, ProviderOllama)
	})

	t.Run("tier2_returns_both", func(t *testing.T) {
		providers := router.getProvidersForTier(Tier2)
		assert.Contains(t, providers, ProviderOllama)
		assert.Contains(t, providers, ProviderAnthropic)
	})

	t.Run("tier3_returns_anthropic_with_fallbacks", func(t *testing.T) {
		providers := router.getProvidersForTier(Tier3)
		assert.Contains(t, providers, ProviderAnthropic)
	})

	t.Run("default_provider_first", func(t *testing.T) {
		providers := router.getProvidersForTier(Tier2)
		assert.Equal(t, ProviderOllama, providers[0]) // Default should be first
	})
}

func TestRouter_HealthCheck_Available(t *testing.T) {
	client := newMockClient(ProviderOllama, true)

	router := newTestRouter(
		&RouterConfig{},
		map[Provider]Client{ProviderOllama: client},
		[]Provider{ProviderOllama},
	)

	err := router.HealthCheck()
	assert.NoError(t, err)
}

func TestRouter_HealthCheck_Unavailable(t *testing.T) {
	client := newMockClient(ProviderOllama, false)

	router := newTestRouter(
		&RouterConfig{},
		map[Provider]Client{ProviderOllama: client},
		[]Provider{ProviderOllama},
	)

	err := router.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no LLM providers available")
}

func TestRouter_HealthCheck_EmptyClients(t *testing.T) {
	router := newTestRouter(
		&RouterConfig{},
		map[Provider]Client{},
		[]Provider{},
	)

	err := router.HealthCheck()
	assert.Error(t, err)
}

func TestBackoffConstants(t *testing.T) {
	// Verify backoff configuration is sensible
	assert.Equal(t, 3, defaultMaxRetries)
	assert.Equal(t, 2*time.Second, initialBackoff)
	assert.Equal(t, 30*time.Second, maxBackoff)
	assert.Equal(t, 2.0, backoffMultiplier)
}

func TestRouter_Complete_ContextTimeout(t *testing.T) {
	client := newMockClient(ProviderOllama, true)
	// Simulate slow response
	client.withErrors(errors.New("timeout"))

	router := newTestRouter(
		&RouterConfig{
			DefaultProvider: ProviderOllama,
			TierModels: map[Tier]map[Provider]string{
				Tier1: {ProviderOllama: "model"},
			},
		},
		map[Provider]Client{ProviderOllama: client},
		[]Provider{ProviderOllama},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond) // Ensure timeout

	_, err := router.Complete(ctx, &Request{Tier: Tier1})
	assert.Error(t, err)
}
