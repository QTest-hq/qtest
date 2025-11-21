package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewGitHubProvider(t *testing.T) {
	cfg := GitHubConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/callback",
	}

	provider := NewGitHubProvider(cfg)

	if provider.clientID != cfg.ClientID {
		t.Errorf("ClientID = %s, want %s", provider.clientID, cfg.ClientID)
	}
	if provider.redirectURL != cfg.RedirectURL {
		t.Errorf("RedirectURL = %s, want %s", provider.redirectURL, cfg.RedirectURL)
	}
	// Default scopes should be set
	if len(provider.scopes) == 0 {
		t.Error("Expected default scopes to be set")
	}
}

func TestNewGitHubProvider_CustomScopes(t *testing.T) {
	cfg := GitHubConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/callback",
		Scopes:       []string{"read:user"},
	}

	provider := NewGitHubProvider(cfg)

	if len(provider.scopes) != 1 {
		t.Errorf("Expected 1 scope, got %d", len(provider.scopes))
	}
	if provider.scopes[0] != "read:user" {
		t.Errorf("Scope = %s, want read:user", provider.scopes[0])
	}
}

func TestGitHubProvider_AuthURL(t *testing.T) {
	provider := NewGitHubProvider(GitHubConfig{
		ClientID:    "test-client",
		RedirectURL: "http://localhost/callback",
	})

	url, state, err := provider.AuthURL()
	if err != nil {
		t.Fatalf("AuthURL failed: %v", err)
	}

	if url == "" {
		t.Error("Expected non-empty URL")
	}
	if state == "" {
		t.Error("Expected non-empty state")
	}

	// URL should contain client_id
	if !contains(url, "client_id=test-client") {
		t.Error("URL should contain client_id")
	}

	// URL should contain state
	if !contains(url, "state=") {
		t.Error("URL should contain state")
	}
}

func TestStateStore(t *testing.T) {
	store := newStateStore(time.Minute)

	// Generate state
	state, err := store.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if state == "" {
		t.Error("Expected non-empty state")
	}

	// Validate should succeed first time
	if !store.Validate(state) {
		t.Error("First validation should succeed")
	}

	// Validate should fail second time (one-time use)
	if store.Validate(state) {
		t.Error("Second validation should fail (one-time use)")
	}
}

func TestStateStore_Expiry(t *testing.T) {
	store := newStateStore(1 * time.Millisecond) // Very short TTL

	state, _ := store.Generate()

	// Wait for expiry
	time.Sleep(5 * time.Millisecond)

	if store.Validate(state) {
		t.Error("Validation should fail after expiry")
	}
}

func TestStateStore_InvalidState(t *testing.T) {
	store := newStateStore(time.Minute)

	if store.Validate("nonexistent") {
		t.Error("Validation should fail for nonexistent state")
	}
}

func TestNewSessionStore(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{
		TTL:        time.Hour,
		MaxPerUser: 3,
	})

	if store.ttl != time.Hour {
		t.Errorf("TTL = %v, want 1h", store.ttl)
	}
	if store.maxPerUser != 3 {
		t.Errorf("MaxPerUser = %d, want 3", store.maxPerUser)
	}
}

func TestNewSessionStore_Defaults(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{})

	if store.ttl != 24*time.Hour {
		t.Errorf("Default TTL = %v, want 24h", store.ttl)
	}
	if store.maxPerUser != 5 {
		t.Errorf("Default MaxPerUser = %d, want 5", store.maxPerUser)
	}
}

func TestSessionStore_Create(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{TTL: time.Hour})

	userID := uuid.New()
	user := &GitHubUser{Login: "testuser", ID: 12345}

	session, err := store.Create(userID, user, "access-token", "refresh-token")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if session.ID == "" {
		t.Error("Session ID should not be empty")
	}
	if session.UserID != userID {
		t.Error("UserID mismatch")
	}
	if session.GitHubUser.Login != "testuser" {
		t.Error("GitHubUser mismatch")
	}
	if session.AccessToken != "access-token" {
		t.Error("AccessToken mismatch")
	}
}

func TestSessionStore_Get(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{TTL: time.Hour})

	userID := uuid.New()
	user := &GitHubUser{Login: "testuser"}
	created, _ := store.Create(userID, user, "token", "")

	// Get should succeed
	session, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if session.ID != created.ID {
		t.Error("Session ID mismatch")
	}
}

func TestSessionStore_Get_NotFound(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{TTL: time.Hour})

	_, err := store.Get("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{TTL: time.Hour})

	userID := uuid.New()
	user := &GitHubUser{Login: "testuser"}
	session, _ := store.Create(userID, user, "token", "")

	store.Delete(session.ID)

	_, err := store.Get(session.ID)
	if err != ErrSessionNotFound {
		t.Error("Session should not exist after deletion")
	}
}

func TestSessionStore_GetByUserID(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{TTL: time.Hour})

	userID := uuid.New()
	user := &GitHubUser{Login: "testuser"}
	created, _ := store.Create(userID, user, "token", "")

	session, err := store.GetByUserID(userID)
	if err != nil {
		t.Fatalf("GetByUserID failed: %v", err)
	}
	if session.ID != created.ID {
		t.Error("Session ID mismatch")
	}
}

func TestSessionStore_Extend(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{TTL: time.Hour})

	userID := uuid.New()
	user := &GitHubUser{Login: "testuser"}
	session, _ := store.Create(userID, user, "token", "")

	originalExpiry := session.ExpiresAt

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	if err := store.Extend(session.ID); err != nil {
		t.Fatalf("Extend failed: %v", err)
	}

	extended, _ := store.Get(session.ID)
	if !extended.ExpiresAt.After(originalExpiry) {
		t.Error("Expiry should be extended")
	}
}

func TestSessionStore_Count(t *testing.T) {
	store := NewSessionStore(SessionStoreConfig{TTL: time.Hour})

	if store.Count() != 0 {
		t.Error("Initial count should be 0")
	}

	user := &GitHubUser{Login: "user1"}
	store.Create(uuid.New(), user, "token", "")

	if store.Count() != 1 {
		t.Error("Count should be 1")
	}

	store.Create(uuid.New(), user, "token", "")

	if store.Count() != 2 {
		t.Error("Count should be 2")
	}
}

func TestSession_IsExpired(t *testing.T) {
	session := &Session{
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if session.IsExpired() {
		t.Error("Session should not be expired")
	}

	session.ExpiresAt = time.Now().Add(-time.Hour)
	if !session.IsExpired() {
		t.Error("Session should be expired")
	}
}

func TestGetSessionFromContext(t *testing.T) {
	session := &Session{ID: "test-session"}
	ctx := context.WithValue(context.Background(), SessionKey, session)

	retrieved, ok := GetSessionFromContext(ctx)
	if !ok {
		t.Error("Should find session in context")
	}
	if retrieved.ID != session.ID {
		t.Error("Session ID mismatch")
	}
}

func TestGetSessionFromContext_NotPresent(t *testing.T) {
	ctx := context.Background()

	_, ok := GetSessionFromContext(ctx)
	if ok {
		t.Error("Should not find session in empty context")
	}
}

func TestGetUserFromContext(t *testing.T) {
	user := &GitHubUser{Login: "testuser"}
	ctx := context.WithValue(context.Background(), UserKey, user)

	retrieved, ok := GetUserFromContext(ctx)
	if !ok {
		t.Error("Should find user in context")
	}
	if retrieved.Login != user.Login {
		t.Error("User mismatch")
	}
}

func TestMiddleware_RequireAuth(t *testing.T) {
	sessions := NewSessionStore(SessionStoreConfig{TTL: time.Hour})
	middleware := NewMiddleware(sessions, nil)

	// Create a test session
	userID := uuid.New()
	user := &GitHubUser{Login: "testuser"}
	session, _ := sessions.Create(userID, user, "token", "")

	// Handler that checks for session
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := GetSessionFromContext(r.Context())
		if !ok {
			t.Error("Session should be in context")
			return
		}
		if s.ID != session.ID {
			t.Error("Session ID mismatch")
		}
		w.WriteHeader(http.StatusOK)
	})

	// Test with valid session
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+session.ID)
	rec := httptest.NewRecorder()

	middleware.RequireAuth(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_RequireAuth_NoAuth(t *testing.T) {
	sessions := NewSessionStore(SessionStoreConfig{TTL: time.Hour})
	middleware := NewMiddleware(sessions, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called without auth")
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()

	middleware.RequireAuth(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_RequireAuth_Cookie(t *testing.T) {
	sessions := NewSessionStore(SessionStoreConfig{TTL: time.Hour})
	middleware := NewMiddleware(sessions, nil)

	userID := uuid.New()
	user := &GitHubUser{Login: "testuser"}
	session, _ := sessions.Create(userID, user, "token", "")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "qtest_session", Value: session.ID})
	rec := httptest.NewRecorder()

	middleware.RequireAuth(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 with cookie auth, got %d", rec.Code)
	}
}

func TestMiddleware_OptionalAuth(t *testing.T) {
	sessions := NewSessionStore(SessionStoreConfig{TTL: time.Hour})
	middleware := NewMiddleware(sessions, nil)

	var hasSession bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasSession = GetSessionFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Without auth - should still proceed
	req := httptest.NewRequest("GET", "/optional", nil)
	rec := httptest.NewRecorder()

	middleware.OptionalAuth(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
	if hasSession {
		t.Error("Should not have session without auth")
	}

	// With auth - should have session
	userID := uuid.New()
	user := &GitHubUser{Login: "testuser"}
	session, _ := sessions.Create(userID, user, "token", "")

	req = httptest.NewRequest("GET", "/optional", nil)
	req.Header.Set("Authorization", "Bearer "+session.ID)
	rec = httptest.NewRecorder()

	middleware.OptionalAuth(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
	if !hasSession {
		t.Error("Should have session with auth")
	}
}

func TestNewHandlers(t *testing.T) {
	github := NewGitHubProvider(GitHubConfig{
		ClientID: "test",
	})
	sessions := NewSessionStore(SessionStoreConfig{})

	handlers := NewHandlers(github, sessions)

	if handlers.github == nil {
		t.Error("GitHub provider should not be nil")
	}
	if handlers.sessions == nil {
		t.Error("Session store should not be nil")
	}
}

func TestGitHubUser_Fields(t *testing.T) {
	user := GitHubUser{
		ID:        12345,
		Login:     "testuser",
		Name:      "Test User",
		Email:     "test@example.com",
		AvatarURL: "https://avatars.githubusercontent.com/u/12345",
	}

	if user.ID != 12345 {
		t.Error("ID mismatch")
	}
	if user.Login != "testuser" {
		t.Error("Login mismatch")
	}
	if user.Name != "Test User" {
		t.Error("Name mismatch")
	}
}

func TestGitHubRepo_Fields(t *testing.T) {
	repo := GitHubRepo{
		ID:            67890,
		Name:          "test-repo",
		FullName:      "testuser/test-repo",
		Private:       false,
		HTMLURL:       "https://github.com/testuser/test-repo",
		CloneURL:      "https://github.com/testuser/test-repo.git",
		DefaultBranch: "main",
		Description:   "A test repository",
	}

	if repo.ID != 67890 {
		t.Error("ID mismatch")
	}
	if repo.FullName != "testuser/test-repo" {
		t.Error("FullName mismatch")
	}
	if repo.DefaultBranch != "main" {
		t.Error("DefaultBranch mismatch")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
