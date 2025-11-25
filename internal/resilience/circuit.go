// Package resilience provides circuit breaker and other resilience patterns.
package resilience

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sony/gobreaker"
)

// CircuitBreakerManager manages named circuit breakers for different services.
type CircuitBreakerManager struct {
	breakers map[string]*gobreaker.CircuitBreaker
	mu       sync.RWMutex
	settings gobreaker.Settings
}

// DefaultSettings returns default circuit breaker settings
func DefaultSettings() gobreaker.Settings {
	return gobreaker.Settings{
		Name:        "default",
		MaxRequests: 3,                // Max requests in half-open state
		Interval:    10 * time.Second, // Interval to clear counts
		Timeout:     30 * time.Second, // Time before trying again after open
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip at 50% failure rate with minimum 5 requests
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Warn().
				Str("breaker", name).
				Str("from", from.String()).
				Str("to", to.String()).
				Msg("circuit breaker state changed")
		},
	}
}

// NewCircuitBreakerManager creates a new circuit breaker manager with default settings
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*gobreaker.CircuitBreaker),
		settings: DefaultSettings(),
	}
}

// NewCircuitBreakerManagerWithSettings creates a manager with custom settings
func NewCircuitBreakerManagerWithSettings(settings gobreaker.Settings) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*gobreaker.CircuitBreaker),
		settings: settings,
	}
}

// Get returns the circuit breaker for the given name, creating one if needed
func (m *CircuitBreakerManager) Get(name string) *gobreaker.CircuitBreaker {
	m.mu.RLock()
	cb, exists := m.breakers[name]
	m.mu.RUnlock()

	if exists {
		return cb
	}

	// Create new breaker
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists = m.breakers[name]; exists {
		return cb
	}

	settings := m.settings
	settings.Name = name
	cb = gobreaker.NewCircuitBreaker(settings)
	m.breakers[name] = cb

	log.Info().Str("breaker", name).Msg("circuit breaker created")
	return cb
}

// GetWithSettings returns a circuit breaker with custom settings for the given name
func (m *CircuitBreakerManager) GetWithSettings(name string, settings gobreaker.Settings) *gobreaker.CircuitBreaker {
	m.mu.RLock()
	cb, exists := m.breakers[name]
	m.mu.RUnlock()

	if exists {
		return cb
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if cb, exists = m.breakers[name]; exists {
		return cb
	}

	settings.Name = name
	cb = gobreaker.NewCircuitBreaker(settings)
	m.breakers[name] = cb

	log.Info().Str("breaker", name).Msg("circuit breaker created with custom settings")
	return cb
}

// Execute runs the given function through the named circuit breaker
func (m *CircuitBreakerManager) Execute(name string, fn func() (interface{}, error)) (interface{}, error) {
	cb := m.Get(name)
	return cb.Execute(fn)
}

// State returns the current state of the named circuit breaker
func (m *CircuitBreakerManager) State(name string) gobreaker.State {
	cb := m.Get(name)
	return cb.State()
}

// IsOpen returns true if the named circuit breaker is open
func (m *CircuitBreakerManager) IsOpen(name string) bool {
	return m.State(name) == gobreaker.StateOpen
}

// Reset resets all circuit breakers (useful for testing)
func (m *CircuitBreakerManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.breakers = make(map[string]*gobreaker.CircuitBreaker)
}

// Common circuit breaker names
const (
	BreakerLLMOllama    = "llm:ollama"
	BreakerLLMAnthropic = "llm:anthropic"
	BreakerLLMOpenAI    = "llm:openai"
	BreakerGitHubAPI    = "github:api"
	BreakerGitHubClone  = "github:clone"
)

// Global circuit breaker manager instance
var defaultManager = NewCircuitBreakerManager()

// Default returns the default circuit breaker manager
func Default() *CircuitBreakerManager {
	return defaultManager
}

// Execute runs the given function through the default manager's named circuit breaker
func Execute(name string, fn func() (interface{}, error)) (interface{}, error) {
	return defaultManager.Execute(name, fn)
}

// IsOpen returns true if the named circuit breaker in the default manager is open
func IsOpen(name string) bool {
	return defaultManager.IsOpen(name)
}
