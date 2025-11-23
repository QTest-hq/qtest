package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// Tests for API key functionality (new multi-tenancy features)

func TestAPIKeyInfo_HasScope(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		check    string
		expected bool
	}{
		{
			name:     "has exact scope",
			scopes:   []string{"repos:read"},
			check:    "repos:read",
			expected: true,
		},
		{
			name:     "admin grants all",
			scopes:   []string{"admin"},
			check:    "repos:read",
			expected: true,
		},
		{
			name:     "missing scope",
			scopes:   []string{"repos:write"},
			check:    "repos:read",
			expected: false,
		},
		{
			name:     "multiple scopes - has match",
			scopes:   []string{"repos:read", "jobs:write"},
			check:    "jobs:write",
			expected: true,
		},
		{
			name:     "empty scopes",
			scopes:   []string{},
			check:    "repos:read",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &APIKeyInfo{Scopes: tt.scopes}
			if got := info.HasScope(tt.check); got != tt.expected {
				t.Errorf("HasScope(%s) = %v, want %v", tt.check, got, tt.expected)
			}
		})
	}
}

func TestAPIKeyInfo_HasAnyScope(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		check    []string
		expected bool
	}{
		{
			name:     "has first scope",
			scopes:   []string{"repos:read"},
			check:    []string{"repos:read", "repos:write"},
			expected: true,
		},
		{
			name:     "has second scope",
			scopes:   []string{"repos:write"},
			check:    []string{"repos:read", "repos:write"},
			expected: true,
		},
		{
			name:     "has none",
			scopes:   []string{"jobs:read"},
			check:    []string{"repos:read", "repos:write"},
			expected: false,
		},
		{
			name:     "admin grants all",
			scopes:   []string{"admin"},
			check:    []string{"repos:read", "repos:write"},
			expected: true,
		},
		{
			name:     "empty check",
			scopes:   []string{"repos:read"},
			check:    []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &APIKeyInfo{Scopes: tt.scopes}
			if got := info.HasAnyScope(tt.check...); got != tt.expected {
				t.Errorf("HasAnyScope(%v) = %v, want %v", tt.check, got, tt.expected)
			}
		})
	}
}

func TestGetAPIKeyFromContext(t *testing.T) {
	info := &APIKeyInfo{ID: uuid.New(), Scopes: []string{"repos:read"}}
	ctx := context.WithValue(context.Background(), APIKeyKey, info)

	retrieved, ok := GetAPIKeyFromContext(ctx)
	if !ok {
		t.Error("GetAPIKeyFromContext() should return ok=true")
	}
	if retrieved.ID != info.ID {
		t.Errorf("retrieved ID = %s, want %s", retrieved.ID, info.ID)
	}
}

func TestGetAPIKeyFromContext_NotFound(t *testing.T) {
	ctx := context.Background()

	_, ok := GetAPIKeyFromContext(ctx)
	if ok {
		t.Error("GetAPIKeyFromContext() should return ok=false for empty context")
	}
}

func TestMiddleware_RequireAuth_WithAPIKey(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{})
	middleware := NewMiddleware(store, nil)

	userID := uuid.New()
	orgID := uuid.New()
	keyID := uuid.New()

	// Set up mock API key validator
	middleware.SetAPIKeyValidator(&mockAPIKeyValidator{
		validateFunc: func(ctx context.Context, key string) (*APIKeyInfo, error) {
			if key == "qtest_valid_key" {
				return &APIKeyInfo{
					ID:             keyID,
					OrganizationID: orgID,
					UserID:         userID,
					Scopes:         []string{"repos:read"},
				}, nil
			}
			return nil, nil
		},
	})

	var gotAPIKey *APIKeyInfo
	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey, _ = GetAPIKeyFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "qtest_valid_key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotAPIKey == nil || gotAPIKey.ID != keyID {
		t.Error("API key info should be in context")
	}
}

func TestMiddleware_RequireAuth_WithBearerAPIKey(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{})
	middleware := NewMiddleware(store, nil)

	userID := uuid.New()
	orgID := uuid.New()

	middleware.SetAPIKeyValidator(&mockAPIKeyValidator{
		validateFunc: func(ctx context.Context, key string) (*APIKeyInfo, error) {
			if key == "qtest_bearer_key" {
				return &APIKeyInfo{
					ID:             uuid.New(),
					OrganizationID: orgID,
					UserID:         userID,
					Scopes:         []string{"repos:read"},
				}, nil
			}
			return nil, nil
		},
	})

	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer qtest_bearer_key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_RequireAuth_InvalidAPIKey(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{})
	middleware := NewMiddleware(store, nil)

	middleware.SetAPIKeyValidator(&mockAPIKeyValidator{
		validateFunc: func(ctx context.Context, key string) (*APIKeyInfo, error) {
			return nil, nil // Key not found
		},
	})

	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with invalid API key")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "qtest_invalid_key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_OptionalAuth_WithAPIKey(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{})
	middleware := NewMiddleware(store, nil)

	userID := uuid.New()
	orgID := uuid.New()

	middleware.SetAPIKeyValidator(&mockAPIKeyValidator{
		validateFunc: func(ctx context.Context, key string) (*APIKeyInfo, error) {
			if key == "qtest_optional_key" {
				return &APIKeyInfo{
					ID:             uuid.New(),
					OrganizationID: orgID,
					UserID:         userID,
					Scopes:         []string{"repos:read"},
				}, nil
			}
			return nil, nil
		},
	})

	var gotAPIKey *APIKeyInfo
	handler := middleware.OptionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey, _ = GetAPIKeyFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "qtest_optional_key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotAPIKey == nil {
		t.Error("API key info should be in context with optional auth")
	}
}

func TestMiddleware_APIKeyPrecedesSession(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{})
	middleware := NewMiddleware(store, nil)

	// Create a session
	sessionUserID := uuid.New()
	githubUser := &GitHubUser{ID: 12345, Login: "sessionuser"}
	session, _ := store.Create(sessionUserID, githubUser, "token", "")

	// Set up API key validator
	apiKeyUserID := uuid.New()
	middleware.SetAPIKeyValidator(&mockAPIKeyValidator{
		validateFunc: func(ctx context.Context, key string) (*APIKeyInfo, error) {
			if key == "qtest_api_key" {
				return &APIKeyInfo{
					ID:             uuid.New(),
					OrganizationID: uuid.New(),
					UserID:         apiKeyUserID,
					Scopes:         []string{"repos:read"},
				}, nil
			}
			return nil, nil
		},
	})

	var gotAPIKey *APIKeyInfo
	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey, _ = GetAPIKeyFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Request with both API key and session - API key should take precedence
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "qtest_api_key")
	req.Header.Set("Authorization", "Bearer "+session.ID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotAPIKey == nil {
		t.Error("API key should take precedence over session")
	}
	if gotAPIKey != nil && gotAPIKey.UserID != apiKeyUserID {
		t.Error("should use API key user, not session user")
	}
}

// mockAPIKeyValidator for testing
type mockAPIKeyValidator struct {
	validateFunc func(ctx context.Context, key string) (*APIKeyInfo, error)
}

func (m *mockAPIKeyValidator) ValidateAPIKeyForAuth(ctx context.Context, key string) (*APIKeyInfo, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, key)
	}
	return nil, nil
}
