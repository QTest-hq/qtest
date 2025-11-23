package db

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateAPIKey(t *testing.T) {
	fullKey, prefix, keyHash, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}

	// Check full key format: qtest_ + 64 hex chars = 70 total
	if !strings.HasPrefix(fullKey, "qtest_") {
		t.Errorf("fullKey should start with 'qtest_', got %s", fullKey[:10])
	}
	if len(fullKey) != 70 {
		t.Errorf("fullKey length = %d, want 70", len(fullKey))
	}

	// Check prefix format: qtest_ + 8 chars = 14 total
	if len(prefix) != 14 {
		t.Errorf("prefix length = %d, want 14", len(prefix))
	}
	if !strings.HasPrefix(prefix, "qtest_") {
		t.Errorf("prefix should start with 'qtest_', got %s", prefix)
	}

	// Check hash is 64 hex chars (SHA256)
	if len(keyHash) != 64 {
		t.Errorf("keyHash length = %d, want 64", len(keyHash))
	}

	// Verify prefix is part of fullKey
	if !strings.HasPrefix(fullKey, prefix) {
		t.Errorf("fullKey should start with prefix")
	}
}

func TestGenerateAPIKey_Uniqueness(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		fullKey, _, _, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey() error = %v", err)
		}
		if keys[fullKey] {
			t.Errorf("duplicate key generated: %s", fullKey)
		}
		keys[fullKey] = true
	}
}

func TestHashAPIKey(t *testing.T) {
	key := "qtest_abcdef1234567890"

	// Hash should be consistent
	hash1 := HashAPIKey(key)
	hash2 := HashAPIKey(key)
	if hash1 != hash2 {
		t.Errorf("HashAPIKey should return consistent results")
	}

	// Hash should be 64 hex chars (SHA256)
	if len(hash1) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash1))
	}

	// Different keys should produce different hashes
	hash3 := HashAPIKey("qtest_different_key")
	if hash1 == hash3 {
		t.Errorf("different keys should produce different hashes")
	}
}

func TestAPIKey_HasScope(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		check    APIKeyScope
		expected bool
	}{
		{
			name:     "has exact scope",
			scopes:   []string{"repos:read"},
			check:    ScopeReadRepos,
			expected: true,
		},
		{
			name:     "has admin scope grants all",
			scopes:   []string{"admin"},
			check:    ScopeReadRepos,
			expected: true,
		},
		{
			name:     "has admin scope grants jobs",
			scopes:   []string{"admin"},
			check:    ScopeWriteJobs,
			expected: true,
		},
		{
			name:     "missing scope",
			scopes:   []string{"repos:write"},
			check:    ScopeReadRepos,
			expected: false,
		},
		{
			name:     "has multiple scopes",
			scopes:   []string{"repos:read", "repos:write", "jobs:read"},
			check:    ScopeWriteRepos,
			expected: true,
		},
		{
			name:     "empty scopes",
			scopes:   []string{},
			check:    ScopeReadRepos,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{Scopes: tt.scopes}
			if got := key.HasScope(tt.check); got != tt.expected {
				t.Errorf("HasScope(%s) = %v, want %v", tt.check, got, tt.expected)
			}
		})
	}
}

func TestAPIKey_IsValid(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name     string
		key      *APIKey
		expected bool
	}{
		{
			name:     "valid key - no expiry",
			key:      &APIKey{},
			expected: true,
		},
		{
			name:     "valid key - future expiry",
			key:      &APIKey{ExpiresAt: &future},
			expected: true,
		},
		{
			name:     "expired key",
			key:      &APIKey{ExpiresAt: &past},
			expected: false,
		},
		{
			name:     "revoked key",
			key:      &APIKey{RevokedAt: &past},
			expected: false,
		},
		{
			name:     "revoked and expired",
			key:      &APIKey{ExpiresAt: &past, RevokedAt: &past},
			expected: false,
		},
		{
			name:     "valid expiry but revoked",
			key:      &APIKey{ExpiresAt: &future, RevokedAt: &past},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.key.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAPIKeyScope_Constants(t *testing.T) {
	// Verify scope string values
	scopes := map[APIKeyScope]string{
		ScopeReadRepos:    "repos:read",
		ScopeWriteRepos:   "repos:write",
		ScopeReadRuns:     "runs:read",
		ScopeWriteRuns:    "runs:write",
		ScopeReadTests:    "tests:read",
		ScopeWriteTests:   "tests:write",
		ScopeReadJobs:     "jobs:read",
		ScopeWriteJobs:    "jobs:write",
		ScopeReadMutation: "mutation:read",
		ScopeAdmin:        "admin",
	}

	for scope, expected := range scopes {
		if string(scope) != expected {
			t.Errorf("scope %v = %s, want %s", scope, string(scope), expected)
		}
	}
}
