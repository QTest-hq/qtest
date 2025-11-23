package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/rs/zerolog/log"
)

// RateLimiter provides HTTP middleware for rate limiting
type RateLimiter struct {
	config  *Config
	storage Storage
}

// New creates a new RateLimiter with the given configuration
func New(cfg *Config) (*RateLimiter, error) {
	if cfg == nil {
		defaultCfg := DefaultConfig()
		cfg = &defaultCfg
	}

	var storage Storage
	switch cfg.StorageBackend {
	case "memory", "":
		storage = NewMemoryStorage()
	case "redis":
		if cfg.RedisURL == "" {
			return nil, fmt.Errorf("redis URL required for redis storage backend")
		}
		var err error
		storage, err = NewRedisStorage(cfg.RedisURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create redis storage: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown storage backend: %s", cfg.StorageBackend)
	}

	return &RateLimiter{
		config:  cfg,
		storage: storage,
	}, nil
}

// Middleware returns a Chi-compatible middleware handler
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			result := rl.check(r)
			rl.setHeaders(w, result)

			if !result.Allowed {
				rl.writeError(w, result)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// check performs the rate limit check based on request context
func (rl *RateLimiter) check(r *http.Request) *Result {
	ctx := r.Context()

	// Determine key type and limits based on authentication
	keyType, keyID, perMinute, perHour := rl.resolveKey(ctx, r)

	// Build the rate limit key
	keyPrefix := fmt.Sprintf("%s:%s", keyType, keyID)

	// Check both windows, return the most restrictive failure
	windows := StandardWindows(perMinute, perHour)

	for _, window := range windows {
		key := fmt.Sprintf("%s:%s", keyPrefix, window.Name)
		count, resetAt, err := rl.storage.Increment(ctx, key, window.Duration, window.Limit)
		if err != nil {
			log.Error().Err(err).Str("key", key).Msg("rate limit storage error")
			// On error, allow the request but don't track it
			continue
		}

		if count > int64(window.Limit) {
			return &Result{
				Allowed:    false,
				Remaining:  0,
				Limit:      window.Limit,
				ResetAt:    resetAt,
				RetryAfter: time.Until(resetAt),
				WindowType: window.Name,
			}
		}
	}

	// Find the most restrictive window to report remaining
	var mostRestrictive *Result
	for _, window := range windows {
		key := fmt.Sprintf("%s:%s", keyPrefix, window.Name)
		count, resetAt, _ := rl.storage.Get(ctx, key)

		remaining := window.Limit - int(count)
		if remaining < 0 {
			remaining = 0
		}

		result := &Result{
			Allowed:    true,
			Remaining:  remaining,
			Limit:      window.Limit,
			ResetAt:    resetAt,
			WindowType: window.Name,
		}

		// Return the window with fewest remaining requests
		if mostRestrictive == nil || result.Remaining < mostRestrictive.Remaining {
			mostRestrictive = result
		}
	}

	if mostRestrictive != nil {
		return mostRestrictive
	}

	return &Result{Allowed: true}
}

// resolveKey determines the rate limit key and limits based on auth context
func (rl *RateLimiter) resolveKey(ctx context.Context, r *http.Request) (KeyType, string, int, int) {
	// Check for API key authentication first
	if apiKey, ok := auth.GetAPIKeyFromContext(ctx); ok {
		return KeyTypeAPIKey, apiKey.ID.String(), rl.config.DefaultPerMinute, rl.config.DefaultPerHour
	}

	// Check for session-based authentication
	if session, ok := auth.GetSessionFromContext(ctx); ok {
		return KeyTypeUser, session.UserID.String(), rl.config.DefaultPerMinute, rl.config.DefaultPerHour
	}

	// Fall back to IP-based rate limiting
	ip := getClientIP(r)
	return KeyTypeIP, ip, rl.config.IPPerMinute, rl.config.IPPerHour
}

// setHeaders sets the standard rate limit response headers
func (rl *RateLimiter) setHeaders(w http.ResponseWriter, result *Result) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

	if !result.Allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())+1))
	}
}

// writeError writes a 429 Too Many Requests response
func (rl *RateLimiter) writeError(w http.ResponseWriter, result *Result) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)

	response := map[string]interface{}{
		"error":               "rate_limit_exceeded",
		"message":             fmt.Sprintf("Rate limit exceeded. Please retry after %d seconds.", int(result.RetryAfter.Seconds())+1),
		"retry_after_seconds": int(result.RetryAfter.Seconds()) + 1,
		"limit_type":          result.WindowType,
		"limit":               result.Limit,
		"reset_at":            result.ResetAt.Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)

	log.Warn().
		Str("window", result.WindowType).
		Int("limit", result.Limit).
		Msg("rate limit exceeded")
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (set by RealIP middleware)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// Close releases resources used by the rate limiter
func (rl *RateLimiter) Close() error {
	if rl.storage != nil {
		return rl.storage.Close()
	}
	return nil
}

// ResetKey resets the rate limit for a specific key (admin use)
func (rl *RateLimiter) ResetKey(ctx context.Context, keyType KeyType, keyID string) error {
	keyPrefix := fmt.Sprintf("%s:%s", keyType, keyID)

	// Reset all windows
	for _, name := range []string{"per_minute", "per_hour"} {
		key := fmt.Sprintf("%s:%s", keyPrefix, name)
		if err := rl.storage.Reset(ctx, key); err != nil {
			return err
		}
	}

	return nil
}
