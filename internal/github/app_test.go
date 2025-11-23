package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// generateTestKey creates a test RSA private key
func generateTestKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	pemBytes := pem.EncodeToMemory(pemBlock)

	return key, string(pemBytes)
}

func TestNewApp_ValidConfig(t *testing.T) {
	_, pemKey := generateTestKey(t)

	cfg := &AppConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	}

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if app.appID != 12345 {
		t.Errorf("expected appID 12345, got %d", app.appID)
	}
}

func TestNewApp_MissingAppID(t *testing.T) {
	_, pemKey := generateTestKey(t)

	cfg := &AppConfig{
		AppID:         0,
		PrivateKeyPEM: pemKey,
	}

	_, err := NewApp(cfg)
	if err == nil {
		t.Fatal("expected error for missing app_id")
	}
	if !strings.Contains(err.Error(), "app_id is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewApp_MissingPrivateKey(t *testing.T) {
	cfg := &AppConfig{
		AppID:         12345,
		PrivateKeyPEM: "",
	}

	_, err := NewApp(cfg)
	if err == nil {
		t.Fatal("expected error for missing private_key")
	}
	if !strings.Contains(err.Error(), "private_key is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewApp_InvalidPrivateKey(t *testing.T) {
	cfg := &AppConfig{
		AppID:         12345,
		PrivateKeyPEM: "not a valid key",
	}

	_, err := NewApp(cfg)
	if err == nil {
		t.Fatal("expected error for invalid private_key")
	}
}

func TestApp_GenerateJWT(t *testing.T) {
	_, pemKey := generateTestKey(t)

	app, err := NewApp(&AppConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	jwt, err := app.generateJWT()
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	// JWT should have 3 parts
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Errorf("expected 3 JWT parts, got %d", len(parts))
	}

	// Verify JWT is not empty
	if jwt == "" {
		t.Error("JWT should not be empty")
	}
}

func TestApp_ListInstallations(t *testing.T) {
	_, pemKey := generateTestKey(t)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/installations" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Error("expected Bearer token")
		}

		installations := []Installation{
			{
				ID: 123,
				Account: Account{
					ID:    1,
					Login: "testorg",
					Type:  "Organization",
				},
				RepositorySelection: "all",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(installations)
	}))
	defer server.Close()

	app, _ := NewApp(&AppConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	app.baseURL = server.URL

	installations, err := app.ListInstallations(context.Background())
	if err != nil {
		t.Fatalf("failed to list installations: %v", err)
	}

	if len(installations) != 1 {
		t.Errorf("expected 1 installation, got %d", len(installations))
	}
	if installations[0].ID != 123 {
		t.Errorf("expected installation ID 123, got %d", installations[0].ID)
	}
}

func TestApp_GetInstallationToken(t *testing.T) {
	_, pemKey := generateTestKey(t)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/installations/123/access_tokens" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		token := InstallationToken{
			Token:     "ghs_test_token_12345",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			Permissions: Perms{
				"contents": "read",
				"metadata": "read",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(token)
	}))
	defer server.Close()

	app, _ := NewApp(&AppConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	app.baseURL = server.URL

	token, err := app.GetInstallationToken(context.Background(), 123)
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}

	if token.Token != "ghs_test_token_12345" {
		t.Errorf("unexpected token: %s", token.Token)
	}
}

func TestApp_GetInstallationToken_Cached(t *testing.T) {
	_, pemKey := generateTestKey(t)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		token := InstallationToken{
			Token:     "ghs_cached_token",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(token)
	}))
	defer server.Close()

	app, _ := NewApp(&AppConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	app.baseURL = server.URL

	// First call should hit the server
	_, err := app.GetInstallationToken(context.Background(), 123)
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}

	// Second call should use cache
	_, err = app.GetInstallationToken(context.Background(), 123)
	if err != nil {
		t.Fatalf("failed to get cached token: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 server call (cached), got %d", callCount)
	}
}

func TestApp_InvalidateTokenCache(t *testing.T) {
	_, pemKey := generateTestKey(t)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		token := InstallationToken{
			Token:     "ghs_token",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(token)
	}))
	defer server.Close()

	app, _ := NewApp(&AppConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	app.baseURL = server.URL

	// First call
	_, _ = app.GetInstallationToken(context.Background(), 123)

	// Invalidate cache
	app.InvalidateTokenCache(123)

	// Second call should hit server again
	_, _ = app.GetInstallationToken(context.Background(), 123)

	if callCount != 2 {
		t.Errorf("expected 2 server calls after invalidation, got %d", callCount)
	}
}

func TestApp_GetInstallationForRepo(t *testing.T) {
	_, pemKey := generateTestKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/testowner/testrepo/installation" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		installation := Installation{
			ID: 456,
			Account: Account{
				Login: "testowner",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(installation)
	}))
	defer server.Close()

	app, _ := NewApp(&AppConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	app.baseURL = server.URL

	installation, err := app.GetInstallationForRepo(context.Background(), "testowner", "testrepo")
	if err != nil {
		t.Fatalf("failed to get installation: %v", err)
	}

	if installation.ID != 456 {
		t.Errorf("expected ID 456, got %d", installation.ID)
	}
}

func TestApp_GetInstallationForRepo_NotInstalled(t *testing.T) {
	_, pemKey := generateTestKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	app, _ := NewApp(&AppConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	app.baseURL = server.URL

	_, err := app.GetInstallationForRepo(context.Background(), "testowner", "testrepo")
	if err == nil {
		t.Fatal("expected error for not installed")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestApp_ListInstallationRepos(t *testing.T) {
	_, pemKey := generateTestKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access_tokens") {
			// Token request
			token := InstallationToken{
				Token:     "ghs_token",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			}
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(token)
			return
		}

		if r.URL.Path != "/installation/repositories" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		result := struct {
			TotalCount   int          `json:"total_count"`
			Repositories []Repository `json:"repositories"`
		}{
			TotalCount: 2,
			Repositories: []Repository{
				{ID: 1, Name: "repo1", FullName: "org/repo1"},
				{ID: 2, Name: "repo2", FullName: "org/repo2"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	app, _ := NewApp(&AppConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	app.baseURL = server.URL

	repos, err := app.ListInstallationRepos(context.Background(), 123)
	if err != nil {
		t.Fatalf("failed to list repos: %v", err)
	}

	if len(repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(repos))
	}
}

func TestApp_GetAppInfo(t *testing.T) {
	_, pemKey := generateTestKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		info := AppInfo{
			ID:          12345,
			Slug:        "qtest",
			Name:        "QTest",
			Description: "AI-powered test generation",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	app, _ := NewApp(&AppConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	app.baseURL = server.URL

	info, err := app.GetAppInfo(context.Background())
	if err != nil {
		t.Fatalf("failed to get app info: %v", err)
	}

	if info.Slug != "qtest" {
		t.Errorf("expected slug 'qtest', got %s", info.Slug)
	}
}

func TestApp_CreateServicesForInstallation(t *testing.T) {
	_, pemKey := generateTestKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := InstallationToken{
			Token:     "ghs_service_token",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(token)
	}))
	defer server.Close()

	app, _ := NewApp(&AppConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	app.baseURL = server.URL

	ctx := context.Background()

	// Test RepoService creation
	repoSvc, err := app.CreateRepoServiceForInstallation(ctx, 123, "/tmp")
	if err != nil {
		t.Fatalf("failed to create repo service: %v", err)
	}
	if repoSvc.token != "ghs_service_token" {
		t.Errorf("unexpected token in repo service")
	}

	// Test PRService creation
	prSvc, err := app.CreatePRServiceForInstallation(ctx, 123)
	if err != nil {
		t.Fatalf("failed to create PR service: %v", err)
	}
	if prSvc.token != "ghs_service_token" {
		t.Errorf("unexpected token in PR service")
	}
}

func TestBase64URLEncode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"f", "Zg"},
		{"fo", "Zm8"},
		{"foo", "Zm9v"},
		{"foobar", "Zm9vYmFy"},
	}

	for _, tc := range tests {
		result := base64URLEncode([]byte(tc.input))
		if result != tc.expected {
			t.Errorf("base64URLEncode(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}
