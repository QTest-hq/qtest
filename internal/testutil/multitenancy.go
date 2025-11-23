// Package testutil provides utilities for integration testing
package testutil

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/QTest-hq/qtest/internal/db"
)

// CreateTestUser creates a user for testing
func CreateTestUser(t *testing.T, store *db.Store, githubID int64, login string) *db.User {
	t.Helper()

	user := &db.User{
		GitHubID:    githubID,
		GitHubLogin: login,
	}

	if err := store.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return user
}

// CreateTestOrganization creates an organization with the user as owner
func CreateTestOrganization(t *testing.T, store *db.Store, ownerID uuid.UUID, name, slug string) *db.Organization {
	t.Helper()

	org := &db.Organization{
		Name:    name,
		Slug:    slug,
		OwnerID: ownerID,
	}

	if err := store.CreateOrganization(context.Background(), org); err != nil {
		t.Fatalf("failed to create test organization: %v", err)
	}

	return org
}

// CreateTestPersonalOrg creates a personal organization for testing
func CreateTestPersonalOrg(t *testing.T, store *db.Store, ownerID uuid.UUID, login string) *db.Organization {
	t.Helper()

	org := &db.Organization{
		Name:       login,
		Slug:       login,
		OwnerID:    ownerID,
		IsPersonal: true,
	}

	if err := store.CreateOrganization(context.Background(), org); err != nil {
		t.Fatalf("failed to create test personal org: %v", err)
	}

	return org
}

// CreateTestAPIKey creates an API key for testing
func CreateTestAPIKey(t *testing.T, store *db.Store, orgID, userID uuid.UUID, name string, scopes []string) *db.APIKeyWithSecret {
	t.Helper()

	key, err := store.CreateAPIKey(context.Background(), orgID, userID, name, scopes, nil)
	if err != nil {
		t.Fatalf("failed to create test API key: %v", err)
	}

	return key
}

// CreateTestSession creates an authenticated session for testing
func CreateTestSession(t *testing.T, sessions *auth.SessionStore, userID uuid.UUID, githubUser *auth.GitHubUser) *auth.Session {
	t.Helper()

	session, err := sessions.Create(userID, githubUser, "test-access-token", "")
	if err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}

	return session
}

// AuthenticatedRequest creates an HTTP request with session auth
func AuthenticatedRequest(t *testing.T, method, url string, body io.Reader, session *auth.Session) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.ID)

	return req
}

// APIKeyRequest creates an HTTP request with API key auth
func APIKeyRequest(t *testing.T, method, url string, body io.Reader, apiKey string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	return req
}

// WithContext adds session and user to request context
func WithContext(r *http.Request, session *auth.Session, user *auth.GitHubUser) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, auth.SessionKey, session)
	if user != nil {
		ctx = context.WithValue(ctx, auth.UserKey, user)
	}
	return r.WithContext(ctx)
}

// WithAPIKeyContext adds API key info to request context
func WithAPIKeyContext(r *http.Request, keyInfo *auth.APIKeyInfo) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, auth.APIKeyKey, keyInfo)
	// Also add synthetic session for handlers that expect it
	ctx = context.WithValue(ctx, auth.SessionKey, &auth.Session{
		UserID:    keyInfo.UserID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	})
	return r.WithContext(ctx)
}

// TestGitHubUser creates a test GitHub user for session testing
func TestGitHubUser(login string, id int64) *auth.GitHubUser {
	return &auth.GitHubUser{
		ID:    id,
		Login: login,
	}
}

// MockAPIKeyValidator implements auth.APIKeyValidator for testing
type MockAPIKeyValidator struct {
	ValidateFunc func(ctx context.Context, key string) (*auth.APIKeyInfo, error)
}

// ValidateAPIKeyForAuth implements auth.APIKeyValidator
func (m *MockAPIKeyValidator) ValidateAPIKeyForAuth(ctx context.Context, key string) (*auth.APIKeyInfo, error) {
	if m.ValidateFunc != nil {
		return m.ValidateFunc(ctx, key)
	}
	return nil, nil
}
