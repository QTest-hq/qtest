package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/google/uuid"
)

func TestNew(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		rl, err := New(nil)
		if err != nil {
			t.Fatalf("New(nil) error = %v", err)
		}
		defer rl.Close()

		if rl.config.DefaultPerMinute != 100 {
			t.Errorf("DefaultPerMinute = %d, want 100", rl.config.DefaultPerMinute)
		}
	})

	t.Run("custom config", func(t *testing.T) {
		cfg := &Config{
			Enabled:          true,
			DefaultPerMinute: 50,
			DefaultPerHour:   500,
			IPPerMinute:      10,
			IPPerHour:        50,
			StorageBackend:   "memory",
		}

		rl, err := New(cfg)
		if err != nil {
			t.Fatalf("New(cfg) error = %v", err)
		}
		defer rl.Close()

		if rl.config.DefaultPerMinute != 50 {
			t.Errorf("DefaultPerMinute = %d, want 50", rl.config.DefaultPerMinute)
		}
	})

	t.Run("unknown backend", func(t *testing.T) {
		cfg := &Config{
			StorageBackend: "unknown",
		}

		_, err := New(cfg)
		if err == nil {
			t.Error("New() should fail for unknown storage backend")
		}
	})
}

func TestMemoryStorage_Increment(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	ctx := context.Background()
	window := time.Minute

	// First increment
	count1, resetAt1, err := storage.Increment(ctx, "test-key", window, 100)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if count1 != 1 {
		t.Errorf("count = %d, want 1", count1)
	}
	if resetAt1.Before(time.Now()) {
		t.Error("resetAt should be in the future")
	}

	// Second increment
	count2, _, err := storage.Increment(ctx, "test-key", window, 100)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if count2 != 2 {
		t.Errorf("count = %d, want 2", count2)
	}
}

func TestMemoryStorage_Get(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	ctx := context.Background()

	// Get non-existent key
	count, _, err := storage.Get(ctx, "non-existent")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// Increment then get
	storage.Increment(ctx, "test-key", time.Minute, 100)
	storage.Increment(ctx, "test-key", time.Minute, 100)

	count, _, err = storage.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestMemoryStorage_Reset(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	ctx := context.Background()

	// Increment then reset
	storage.Increment(ctx, "test-key", time.Minute, 100)
	storage.Increment(ctx, "test-key", time.Minute, 100)

	if err := storage.Reset(ctx, "test-key"); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	count, _, _ := storage.Get(ctx, "test-key")
	if count != 0 {
		t.Errorf("count after reset = %d, want 0", count)
	}
}

func TestRateLimiter_Middleware_Disabled(t *testing.T) {
	cfg := &Config{
		Enabled: false,
	}

	rl, _ := New(cfg)
	defer rl.Close()

	handler := rl.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRateLimiter_Middleware_IPBased(t *testing.T) {
	cfg := &Config{
		Enabled:     true,
		IPPerMinute: 3,
		IPPerHour:   10,
	}

	rl, _ := New(cfg)
	defer rl.Close()

	handler := rl.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make requests up to the limit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}

		// Check rate limit headers
		if rec.Header().Get("X-RateLimit-Limit") == "" {
			t.Error("X-RateLimit-Limit header not set")
		}
		if rec.Header().Get("X-RateLimit-Remaining") == "" {
			t.Error("X-RateLimit-Remaining header not set")
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	// Check Retry-After header
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header not set on 429 response")
	}
}

func TestRateLimiter_Middleware_WithAPIKey(t *testing.T) {
	cfg := &Config{
		Enabled:          true,
		DefaultPerMinute: 5,
		DefaultPerHour:   50,
		IPPerMinute:      2,
		IPPerHour:        10,
	}

	rl, _ := New(cfg)
	defer rl.Close()

	apiKeyID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()

	apiKey := &auth.APIKeyInfo{
		ID:             apiKeyID,
		OrganizationID: orgID,
		UserID:         userID,
		Scopes:         []string{"repos:read"},
	}

	handler := rl.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Authenticated requests should use higher limits
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), auth.APIKeyKey, apiKey)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	// 6th request should be rate limited for API key
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), auth.APIKeyKey, apiKey)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_ResetKey(t *testing.T) {
	cfg := &Config{
		Enabled:          true,
		DefaultPerMinute: 3,
		DefaultPerHour:   10,
	}

	rl, _ := New(cfg)
	defer rl.Close()

	ctx := context.Background()

	// Simulate some requests via storage
	keyPrefix := "apikey:test-key"
	rl.storage.Increment(ctx, keyPrefix+":per_minute", time.Minute, 3)
	rl.storage.Increment(ctx, keyPrefix+":per_minute", time.Minute, 3)
	rl.storage.Increment(ctx, keyPrefix+":per_minute", time.Minute, 3)

	// Verify we're at the limit
	count, _, _ := rl.storage.Get(ctx, keyPrefix+":per_minute")
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	// Reset the key
	if err := rl.ResetKey(ctx, KeyTypeAPIKey, "test-key"); err != nil {
		t.Fatalf("ResetKey() error = %v", err)
	}

	// Verify reset
	count, _, _ = rl.storage.Get(ctx, keyPrefix+":per_minute")
	if count != 0 {
		t.Errorf("count after reset = %d, want 0", count)
	}
}

func TestStandardWindows(t *testing.T) {
	windows := StandardWindows(100, 1000)

	if len(windows) != 2 {
		t.Fatalf("len(windows) = %d, want 2", len(windows))
	}

	if windows[0].Name != "per_minute" || windows[0].Limit != 100 {
		t.Errorf("windows[0] = %+v, want per_minute with limit 100", windows[0])
	}

	if windows[1].Name != "per_hour" || windows[1].Limit != 1000 {
		t.Errorf("windows[1] = %+v, want per_hour with limit 1000", windows[1])
	}
}

func TestStandardWindows_Empty(t *testing.T) {
	windows := StandardWindows(0, 0)
	if len(windows) != 0 {
		t.Errorf("len(windows) = %d, want 0", len(windows))
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name        string
		xff         string
		xri         string
		remoteAddr  string
		expectedIP  string
	}{
		{
			name:       "X-Forwarded-For",
			xff:        "1.2.3.4",
			remoteAddr: "127.0.0.1:12345",
			expectedIP: "1.2.3.4",
		},
		{
			name:       "X-Real-IP",
			xri:        "5.6.7.8",
			remoteAddr: "127.0.0.1:12345",
			expectedIP: "5.6.7.8",
		},
		{
			name:       "RemoteAddr fallback",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1:12345",
		},
		{
			name:       "X-Forwarded-For takes precedence",
			xff:        "1.2.3.4",
			xri:        "5.6.7.8",
			remoteAddr: "127.0.0.1:12345",
			expectedIP: "1.2.3.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			ip := getClientIP(req)
			if ip != tt.expectedIP {
				t.Errorf("getClientIP() = %s, want %s", ip, tt.expectedIP)
			}
		})
	}
}

func TestResult(t *testing.T) {
	result := &Result{
		Allowed:    true,
		Remaining:  50,
		Limit:      100,
		ResetAt:    time.Now().Add(time.Minute),
		WindowType: "per_minute",
	}

	if !result.Allowed {
		t.Error("result.Allowed should be true")
	}
	if result.Remaining != 50 {
		t.Errorf("result.Remaining = %d, want 50", result.Remaining)
	}
}
