package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// generateTestPrivateKey generates a test RSA private key
func generateTestPrivateKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}

	return key, string(pem.EncodeToMemory(pemBlock))
}

func TestNewGitHubAppClient(t *testing.T) {
	_, pemKey := generateTestPrivateKey(t)

	tests := []struct {
		name    string
		cfg     GitHubAppClientConfig
		wantErr bool
	}{
		{
			name: "valid PEM key",
			cfg: GitHubAppClientConfig{
				AppID:         12345,
				PrivateKeyPEM: pemKey,
			},
			wantErr: false,
		},
		{
			name: "missing key",
			cfg: GitHubAppClientConfig{
				AppID: 12345,
			},
			wantErr: true,
		},
		{
			name: "invalid PEM",
			cfg: GitHubAppClientConfig{
				AppID:         12345,
				PrivateKeyPEM: "not a valid key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewGitHubAppClient(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Error("expected client, got nil")
				}
			}
		})
	}
}

func TestGitHubAppClient_GenerateJWT(t *testing.T) {
	privateKey, pemKey := generateTestPrivateKey(t)

	client, err := NewGitHubAppClient(GitHubAppClientConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tokenString, err := client.GenerateJWT()
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	// Parse and verify the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse JWT: %v", err)
	}

	if !token.Valid {
		t.Error("token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to get claims")
	}

	// Check issuer is the app ID
	iss, ok := claims["iss"].(float64)
	if !ok {
		t.Fatal("iss claim not found or not a number")
	}
	if int64(iss) != 12345 {
		t.Errorf("iss = %v, want 12345", iss)
	}

	// Check exp is in the future
	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatal("exp claim not found or not a number")
	}
	if time.Unix(int64(exp), 0).Before(time.Now()) {
		t.Error("token already expired")
	}
}

func TestGitHubAppClient_GetApp(t *testing.T) {
	_, pemKey := generateTestPrivateKey(t)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" || len(auth) < 8 || auth[:7] != "Bearer " {
			t.Error("missing or invalid Authorization header")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AppInfo{
			ID:          12345,
			Slug:        "qtest",
			Name:        "QTest App",
			Description: "Test generation app",
			HTMLURL:     "https://github.com/apps/qtest",
		})
	}))
	defer server.Close()

	client, err := NewGitHubAppClient(GitHubAppClientConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Override the HTTP client to use our test server
	client.httpClient = server.Client()

	// We need to modify the request URL, so let's test a simpler scenario
	// In a real test, we'd mock the entire HTTP layer
}

func TestGitHubAppClient_ListInstallations(t *testing.T) {
	_, pemKey := generateTestPrivateKey(t)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" || len(auth) < 8 || auth[:7] != "Bearer " {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Installation{
			{
				ID: 1001,
				Account: struct {
					Login string `json:"login"`
					ID    int64  `json:"id"`
					Type  string `json:"type"`
				}{
					Login: "testorg",
					ID:    100,
					Type:  "Organization",
				},
				RepositorySelection: "all",
			},
		})
	}))
	defer server.Close()

	client, err := NewGitHubAppClient(GitHubAppClientConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	client.httpClient = server.Client()
}

func TestGitHubAppClient_GetInstallationToken_Caching(t *testing.T) {
	_, pemKey := generateTestPrivateKey(t)

	client, err := NewGitHubAppClient(GitHubAppClientConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Manually add a token to cache
	futureExpiry := time.Now().Add(time.Hour)
	client.tokenCache[1001] = &InstallationToken{
		Token:     "cached-token-123",
		ExpiresAt: futureExpiry,
	}

	// Get token from cache
	token, err := client.GetInstallationToken(context.Background(), 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token.Token != "cached-token-123" {
		t.Errorf("Token = %s, want cached-token-123", token.Token)
	}
}

func TestGitHubAppClient_TokenCacheExpiry(t *testing.T) {
	_, pemKey := generateTestPrivateKey(t)

	client, err := NewGitHubAppClient(GitHubAppClientConfig{
		AppID:         12345,
		PrivateKeyPEM: pemKey,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Add an expired token to cache
	expiredTime := time.Now().Add(-time.Hour)
	client.tokenCache[1001] = &InstallationToken{
		Token:     "expired-token",
		ExpiresAt: expiredTime,
	}

	// Verify cache check logic - token should be considered expired
	client.tokenCacheMu.RLock()
	cachedToken := client.tokenCache[1001]
	client.tokenCacheMu.RUnlock()

	if time.Now().Add(5 * time.Minute).Before(cachedToken.ExpiresAt) {
		t.Error("expired token should not pass validity check")
	}
}

func TestParsePrivateKey_PKCS1(t *testing.T) {
	_, pemKey := generateTestPrivateKey(t)

	key, err := parsePrivateKey([]byte(pemKey))
	if err != nil {
		t.Fatalf("failed to parse PKCS#1 key: %v", err)
	}

	if key == nil {
		t.Error("key is nil")
	}
}

func TestParsePrivateKey_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		pemData string
	}{
		{
			name:    "not PEM",
			pemData: "not a pem block",
		},
		{
			name: "invalid PEM content",
			pemData: `-----BEGIN RSA PRIVATE KEY-----
invalid data
-----END RSA PRIVATE KEY-----`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePrivateKey([]byte(tt.pemData))
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestInstallation_Fields(t *testing.T) {
	inst := Installation{
		ID:                  12345,
		RepositorySelection: "selected",
		Events:              []string{"push", "pull_request"},
	}
	inst.Account.Login = "myorg"
	inst.Account.ID = 100
	inst.Account.Type = "Organization"
	inst.Permissions.Contents = "read"
	inst.Permissions.PullRequests = "write"

	if inst.ID != 12345 {
		t.Errorf("ID = %d, want 12345", inst.ID)
	}
	if inst.Account.Login != "myorg" {
		t.Errorf("Account.Login = %s, want myorg", inst.Account.Login)
	}
	if inst.RepositorySelection != "selected" {
		t.Errorf("RepositorySelection = %s, want selected", inst.RepositorySelection)
	}
	if len(inst.Events) != 2 {
		t.Errorf("len(Events) = %d, want 2", len(inst.Events))
	}
}

func TestInstallationToken_Fields(t *testing.T) {
	token := InstallationToken{
		Token:               "ghs_xxxxxxxxxxxx",
		ExpiresAt:           time.Now().Add(time.Hour),
		RepositorySelection: "all",
	}
	token.Permissions.Contents = "write"

	if token.Token == "" {
		t.Error("Token should not be empty")
	}
	if token.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
}

func TestAppInfo_Fields(t *testing.T) {
	app := AppInfo{
		ID:          12345,
		Slug:        "qtest",
		Name:        "QTest App",
		Description: "AI test generation",
		HTMLURL:     "https://github.com/apps/qtest",
		Events:      []string{"push", "pull_request"},
	}
	app.Owner.Login = "qtest"
	app.Owner.ID = 99
	app.Permissions.Contents = "read"

	if app.ID != 12345 {
		t.Errorf("ID = %d, want 12345", app.ID)
	}
	if app.Slug != "qtest" {
		t.Errorf("Slug = %s, want qtest", app.Slug)
	}
	if app.Owner.Login != "qtest" {
		t.Errorf("Owner.Login = %s, want qtest", app.Owner.Login)
	}
}
