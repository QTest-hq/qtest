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

// TestDefaultConfig tests that default config has sensible values
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("expected rate limiting to be enabled by default")
	}
	if cfg.DefaultPerMinute != 100 {
		t.Errorf("DefaultPerMinute = %d, want 100", cfg.DefaultPerMinute)
	}
	if cfg.DefaultPerHour != 1000 {
		t.Errorf("DefaultPerHour = %d, want 1000", cfg.DefaultPerHour)
	}
	if cfg.IPPerMinute != 20 {
		t.Errorf("IPPerMinute = %d, want 20", cfg.IPPerMinute)
	}
	if cfg.IPPerHour != 100 {
		t.Errorf("IPPerHour = %d, want 100", cfg.IPPerHour)
	}
	if cfg.StorageBackend != "memory" {
		t.Errorf("StorageBackend = %s, want memory", cfg.StorageBackend)
	}
}

// TestStandardWindows tests window generation
func TestStandardWindows(t *testing.T) {
	tests := []struct {
		name      string
		perMinute int
		perHour   int
		wantCount int
	}{
		{
			name:      "both windows",
			perMinute: 60,
			perHour:   600,
			wantCount: 2,
		},
		{
			name:      "only per minute",
			perMinute: 60,
			perHour:   0,
			wantCount: 1,
		},
		{
			name:      "only per hour",
			perMinute: 0,
			perHour:   600,
			wantCount: 1,
		},
		{
			name:      "no windows",
			perMinute: 0,
			perHour:   0,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			windows := StandardWindows(tt.perMinute, tt.perHour)
			if len(windows) != tt.wantCount {
				t.Errorf("got %d windows, want %d", len(windows), tt.wantCount)
			}

			for _, w := range windows {
				if w.Name != "per_minute" && w.Name != "per_hour" {
					t.Errorf("unexpected window name: %s", w.Name)
				}
				if w.Name == "per_minute" && w.Duration != time.Minute {
					t.Errorf("per_minute window duration = %v, want %v", w.Duration, time.Minute)
				}
				if w.Name == "per_hour" && w.Duration != time.Hour {
					t.Errorf("per_hour window duration = %v, want %v", w.Duration, time.Hour)
				}
			}
		})
	}
}

// TestMemoryStorage tests the in-memory storage implementation
func TestMemoryStorage(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()
	ctx := context.Background()

	t.Run("increment new key", func(t *testing.T) {
		count, resetAt, err := storage.Increment(ctx, "test:key1", time.Minute, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
		if resetAt.Before(time.Now()) {
			t.Error("resetAt should be in the future")
		}
	})

	t.Run("increment existing key", func(t *testing.T) {
		storage.Increment(ctx, "test:key2", time.Minute, 10)
		count, _, err := storage.Increment(ctx, "test:key2", time.Minute, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
	})

	t.Run("get existing key", func(t *testing.T) {
		storage.Increment(ctx, "test:key3", time.Minute, 10)
		count, _, err := storage.Get(ctx, "test:key3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})

	t.Run("get non-existent key", func(t *testing.T) {
		count, _, err := storage.Get(ctx, "test:nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
	})

	t.Run("reset key", func(t *testing.T) {
		storage.Increment(ctx, "test:key4", time.Minute, 10)
		err := storage.Reset(ctx, "test:key4")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		count, _, _ := storage.Get(ctx, "test:key4")
		if count != 0 {
			t.Errorf("count after reset = %d, want 0", count)
		}
	})
}

// TestNew tests rate limiter creation
func TestNew(t *testing.T) {
	t.Run("nil config uses defaults", func(t *testing.T) {
		rl, err := New(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer rl.Close()

		if rl.config == nil {
			t.Error("expected config to be set")
		}
	})

	t.Run("memory backend", func(t *testing.T) {
		cfg := &Config{
			StorageBackend: "memory",
			Enabled:        true,
		}
		rl, err := New(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer rl.Close()
	})

	t.Run("empty backend defaults to memory", func(t *testing.T) {
		cfg := &Config{
			StorageBackend: "",
			Enabled:        true,
		}
		rl, err := New(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer rl.Close()
	})

	t.Run("unknown backend returns error", func(t *testing.T) {
		cfg := &Config{
			StorageBackend: "unknown",
		}
		_, err := New(cfg)
		if err == nil {
			t.Error("expected error for unknown backend")
		}
	})

	t.Run("redis backend without URL returns error", func(t *testing.T) {
		cfg := &Config{
			StorageBackend: "redis",
			RedisURL:       "",
		}
		_, err := New(cfg)
		if err == nil {
			t.Error("expected error for redis without URL")
		}
	})
}

// TestMiddleware_Disabled tests that disabled rate limiter passes requests through
func TestMiddleware_Disabled(t *testing.T) {
	cfg := &Config{
		Enabled:        false,
		StorageBackend: "memory",
	}
	rl, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rl.Close()

	handler := rl.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestMiddleware_Enabled tests rate limiting behavior
func TestMiddleware_Enabled(t *testing.T) {
	cfg := &Config{
		Enabled:          true,
		StorageBackend:   "memory",
		DefaultPerMinute: 5,
		DefaultPerHour:   50,
		IPPerMinute:      2,
		IPPerHour:        10,
	}
	rl, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rl.Close()

	handler := rl.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("sets rate limit headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("X-RateLimit-Limit") == "" {
			t.Error("expected X-RateLimit-Limit header")
		}
		if rr.Header().Get("X-RateLimit-Remaining") == "" {
			t.Error("expected X-RateLimit-Remaining header")
		}
		if rr.Header().Get("X-RateLimit-Reset") == "" {
			t.Error("expected X-RateLimit-Reset header")
		}
	})

	t.Run("blocks after limit exceeded", func(t *testing.T) {
		// Use unique IP for this test
		ip := "10.0.0.1:12345"

		// First 2 requests should succeed (IPPerMinute = 2)
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = ip
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("request %d: status = %d, want %d", i+1, rr.Code, http.StatusOK)
			}
		}

		// Third request should be rate limited
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusTooManyRequests {
			t.Errorf("request after limit: status = %d, want %d", rr.Code, http.StatusTooManyRequests)
		}

		// Should have Retry-After header
		if rr.Header().Get("Retry-After") == "" {
			t.Error("expected Retry-After header when rate limited")
		}
	})
}

// TestGetClientIP tests IP extraction from request
func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		xri        string
		remoteAddr string
		expected   string
	}{
		{
			name:       "X-Forwarded-For takes precedence",
			xff:        "203.0.113.195",
			xri:        "203.0.113.100",
			remoteAddr: "127.0.0.1:12345",
			expected:   "203.0.113.195",
		},
		{
			name:       "X-Real-IP as fallback",
			xff:        "",
			xri:        "203.0.113.100",
			remoteAddr: "127.0.0.1:12345",
			expected:   "203.0.113.100",
		},
		{
			name:       "RemoteAddr as last resort",
			xff:        "",
			xri:        "",
			remoteAddr: "192.168.1.1:54321",
			expected:   "192.168.1.1:54321",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}
			req.RemoteAddr = tt.remoteAddr

			result := getClientIP(req)
			if result != tt.expected {
				t.Errorf("getClientIP() = %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestResolveKey tests key resolution based on auth context
func TestResolveKey(t *testing.T) {
	cfg := DefaultConfig()
	rl := &RateLimiter{config: &cfg}

	t.Run("API key auth", func(t *testing.T) {
		apiKeyInfo := &auth.APIKeyInfo{
			ID:             uuid.New(),
			OrganizationID: uuid.New(),
			UserID:         uuid.New(),
		}
		ctx := context.WithValue(context.Background(), auth.APIKeyKey, apiKeyInfo)
		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)

		keyType, keyID, _, _ := rl.resolveKey(ctx, req)
		if keyType != KeyTypeAPIKey {
			t.Errorf("keyType = %s, want %s", keyType, KeyTypeAPIKey)
		}
		if keyID != apiKeyInfo.ID.String() {
			t.Error("keyID should be API key ID")
		}
	})

	t.Run("session auth", func(t *testing.T) {
		session := &auth.Session{
			ID:     "test-session",
			UserID: uuid.New(),
		}
		ctx := context.WithValue(context.Background(), auth.SessionKey, session)
		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)

		keyType, keyID, _, _ := rl.resolveKey(ctx, req)
		if keyType != KeyTypeUser {
			t.Errorf("keyType = %s, want %s", keyType, KeyTypeUser)
		}
		if keyID != session.UserID.String() {
			t.Error("keyID should be user ID")
		}
	})

	t.Run("IP fallback", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		ctx := req.Context()

		keyType, keyID, perMinute, perHour := rl.resolveKey(ctx, req)
		if keyType != KeyTypeIP {
			t.Errorf("keyType = %s, want %s", keyType, KeyTypeIP)
		}
		if keyID != "192.168.1.1:12345" {
			t.Errorf("keyID = %s, want IP address", keyID)
		}
		if perMinute != cfg.IPPerMinute {
			t.Errorf("perMinute = %d, want %d", perMinute, cfg.IPPerMinute)
		}
		if perHour != cfg.IPPerHour {
			t.Errorf("perHour = %d, want %d", perHour, cfg.IPPerHour)
		}
	})
}

// TestResetKey tests key reset functionality
func TestResetKey(t *testing.T) {
	cfg := &Config{
		Enabled:          true,
		StorageBackend:   "memory",
		DefaultPerMinute: 10,
		DefaultPerHour:   100,
		IPPerMinute:      5,
		IPPerHour:        50,
	}
	rl, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rl.Close()

	ctx := context.Background()

	// Make some requests to increment counter
	handler := rl.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "10.10.10.10:12345"
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}

	// Reset the key
	err = rl.ResetKey(ctx, KeyTypeIP, ip)
	if err != nil {
		t.Fatalf("ResetKey failed: %v", err)
	}

	// Should be able to make requests again
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = ip
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("after reset: status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestResult_Fields tests Result struct
func TestResult_Fields(t *testing.T) {
	result := &Result{
		Allowed:    true,
		Remaining:  50,
		Limit:      100,
		ResetAt:    time.Now().Add(time.Minute),
		RetryAfter: 0,
		WindowType: "per_minute",
	}

	if !result.Allowed {
		t.Error("expected Allowed to be true")
	}
	if result.Remaining != 50 {
		t.Errorf("Remaining = %d, want 50", result.Remaining)
	}
	if result.Limit != 100 {
		t.Errorf("Limit = %d, want 100", result.Limit)
	}
	if result.WindowType != "per_minute" {
		t.Errorf("WindowType = %s, want per_minute", result.WindowType)
	}
}
