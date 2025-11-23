// Package ratelimit provides rate limiting middleware for the QTest API
package ratelimit

import (
	"context"
	"time"
)

// Config holds rate limiting configuration
type Config struct {
	// Enabled toggles rate limiting
	Enabled bool

	// Default limits for authenticated users
	DefaultPerMinute int
	DefaultPerHour   int

	// IP-based limits for unauthenticated requests
	IPPerMinute int
	IPPerHour   int

	// Storage backend: "memory" or "redis"
	StorageBackend string

	// Redis URL (required if StorageBackend is "redis")
	RedisURL string
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		Enabled:          true,
		DefaultPerMinute: 100,
		DefaultPerHour:   1000,
		IPPerMinute:      20,
		IPPerHour:        100,
		StorageBackend:   "memory",
	}
}

// Result represents the outcome of a rate limit check
type Result struct {
	// Allowed indicates if the request should proceed
	Allowed bool

	// Remaining requests in the current window
	Remaining int

	// Limit is the total allowed requests in the window
	Limit int

	// ResetAt is when the current window expires
	ResetAt time.Time

	// RetryAfter is how long to wait before retrying (only set when not allowed)
	RetryAfter time.Duration

	// WindowType identifies the limit window ("per_minute" or "per_hour")
	WindowType string
}

// KeyType identifies what the rate limit key represents
type KeyType string

const (
	KeyTypeAPIKey KeyType = "apikey"
	KeyTypeOrg    KeyType = "org"
	KeyTypeUser   KeyType = "user"
	KeyTypeIP     KeyType = "ip"
)

// Storage is the interface for rate limit counter storage
type Storage interface {
	// Increment increments the counter for a key and returns the new count
	// along with the window reset time. Returns error if storage fails.
	Increment(ctx context.Context, key string, window time.Duration, limit int) (count int64, resetAt time.Time, err error)

	// Get returns the current count and reset time for a key without incrementing
	Get(ctx context.Context, key string) (count int64, resetAt time.Time, err error)

	// Reset clears the counter for a key
	Reset(ctx context.Context, key string) error

	// Close releases any resources
	Close() error
}

// Window represents a rate limit time window
type Window struct {
	Duration time.Duration
	Limit    int
	Name     string // "per_minute", "per_hour"
}

// StandardWindows returns the default rate limit windows
func StandardWindows(perMinute, perHour int) []Window {
	windows := make([]Window, 0, 2)
	if perMinute > 0 {
		windows = append(windows, Window{
			Duration: time.Minute,
			Limit:    perMinute,
			Name:     "per_minute",
		})
	}
	if perHour > 0 {
		windows = append(windows, Window{
			Duration: time.Hour,
			Limit:    perHour,
			Name:     "per_hour",
		})
	}
	return windows
}
