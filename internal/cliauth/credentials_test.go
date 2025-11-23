package cliauth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCredentialsManager_SaveAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	mgr := NewCredentialsManagerWithDir(tmpDir)

	// Test save
	creds := &Credentials{
		APIKey:         "qtest_test123456789012345",
		APIServer:      "http://localhost:8080",
		OrganizationID: "org-123",
		UserID:         "user-456",
		Username:       "testuser",
		Email:          "test@example.com",
		CreatedAt:      time.Now(),
	}

	if err := mgr.Save(creds); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists with correct permissions
	info, err := os.Stat(filepath.Join(tmpDir, "credentials.json"))
	if err != nil {
		t.Fatalf("Credentials file not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected 0600 permissions, got %o", info.Mode().Perm())
	}

	// Test load
	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.APIKey != creds.APIKey {
		t.Errorf("APIKey mismatch: expected %s, got %s", creds.APIKey, loaded.APIKey)
	}
	if loaded.APIServer != creds.APIServer {
		t.Errorf("APIServer mismatch: expected %s, got %s", creds.APIServer, loaded.APIServer)
	}
	if loaded.Username != creds.Username {
		t.Errorf("Username mismatch: expected %s, got %s", creds.Username, loaded.Username)
	}
}

func TestCredentialsManager_LoadNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCredentialsManagerWithDir(tmpDir)

	_, err := mgr.Load()
	if err != ErrNoCredentials {
		t.Errorf("Expected ErrNoCredentials, got %v", err)
	}
}

func TestCredentialsManager_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCredentialsManagerWithDir(tmpDir)

	// Save first
	creds := &Credentials{
		APIKey:    "qtest_test123456789012345",
		CreatedAt: time.Now(),
	}
	if err := mgr.Save(creds); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify exists
	if !mgr.Exists() {
		t.Fatal("Credentials should exist after save")
	}

	// Delete
	if err := mgr.Delete(); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	if mgr.Exists() {
		t.Error("Credentials should not exist after delete")
	}

	// Delete again should not error
	if err := mgr.Delete(); err != nil {
		t.Errorf("Second delete should not error: %v", err)
	}
}

func TestCredentialsManager_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCredentialsManagerWithDir(tmpDir)

	// Should not exist initially
	if mgr.Exists() {
		t.Error("Credentials should not exist initially")
	}

	// Save
	creds := &Credentials{
		APIKey:    "qtest_test123456789012345",
		CreatedAt: time.Now(),
	}
	if err := mgr.Save(creds); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Should exist now
	if !mgr.Exists() {
		t.Error("Credentials should exist after save")
	}
}

func TestCredentials_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		creds    Credentials
		expected bool
	}{
		{
			name: "not expired - zero time",
			creds: Credentials{
				APIKey:    "qtest_test123456789012345",
				ExpiresAt: time.Time{}, // Zero time means no expiry
			},
			expected: false,
		},
		{
			name: "not expired - future time",
			creds: Credentials{
				APIKey:    "qtest_test123456789012345",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expected: false,
		},
		{
			name: "expired - past time",
			creds: Credentials{
				APIKey:    "qtest_test123456789012345",
				ExpiresAt: time.Now().Add(-time.Hour),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.creds.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid key",
			apiKey:  "qtest_abcdefghij123456789",
			wantErr: false,
		},
		{
			name:    "empty key",
			apiKey:  "",
			wantErr: true,
		},
		{
			name:    "missing prefix",
			apiKey:  "abcdefghij123456789012345",
			wantErr: true,
		},
		{
			name:    "wrong prefix",
			apiKey:  "test_abcdefghij1234567890",
			wantErr: true,
		},
		{
			name:    "too short",
			apiKey:  "qtest_abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAPIKey(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAPIKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultAPIServer(t *testing.T) {
	// Test with env var unset
	os.Unsetenv("QTEST_API_URL")
	if got := DefaultAPIServer(); got != "http://localhost:8080" {
		t.Errorf("DefaultAPIServer() = %v, want http://localhost:8080", got)
	}

	// Test with env var set
	os.Setenv("QTEST_API_URL", "https://api.qtest.io")
	defer os.Unsetenv("QTEST_API_URL")

	if got := DefaultAPIServer(); got != "https://api.qtest.io" {
		t.Errorf("DefaultAPIServer() = %v, want https://api.qtest.io", got)
	}
}

func TestGetAPIKey_FromEnv(t *testing.T) {
	// Set env var
	os.Setenv("QTEST_API_KEY", "qtest_env123456789012345")
	defer os.Unsetenv("QTEST_API_KEY")

	key, err := GetAPIKey()
	if err != nil {
		t.Fatalf("GetAPIKey() error = %v", err)
	}
	if key != "qtest_env123456789012345" {
		t.Errorf("GetAPIKey() = %v, want qtest_env123456789012345", key)
	}
}

func TestGetCredentials_FromEnv(t *testing.T) {
	os.Setenv("QTEST_API_KEY", "qtest_env123456789012345")
	defer os.Unsetenv("QTEST_API_KEY")

	creds, err := GetCredentials()
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}
	if creds.APIKey != "qtest_env123456789012345" {
		t.Errorf("APIKey = %v, want qtest_env123456789012345", creds.APIKey)
	}
}

func TestNewAPIClient(t *testing.T) {
	client := NewAPIClient("http://localhost:8080/", "qtest_test123")
	if client.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %v, want http://localhost:8080 (no trailing slash)", client.baseURL)
	}
	if client.apiKey != "qtest_test123" {
		t.Errorf("apiKey = %v, want qtest_test123", client.apiKey)
	}
}
