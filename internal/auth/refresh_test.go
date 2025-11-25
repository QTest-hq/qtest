package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashToken(t *testing.T) {
	tests := []struct {
		name   string
		token  string
	}{
		{"short token", "abc123"},
		{"uuid token", uuid.New().String()},
		{"long token", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := hashToken(tt.token)

			// Hash should be deterministic
			hash2 := hashToken(tt.token)
			if hash != hash2 {
				t.Error("hashToken() should be deterministic")
			}

			// Hash should be 64 chars (SHA256 = 32 bytes = 64 hex chars)
			if len(hash) != 64 {
				t.Errorf("hash length = %d, want 64", len(hash))
			}

			// Different tokens should have different hashes
			otherHash := hashToken(tt.token + "x")
			if hash == otherHash {
				t.Error("different tokens should have different hashes")
			}
		})
	}
}

func TestNewRefreshService(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	jwtSvc := NewJWTServiceWithKeys(privateKey, nil, DefaultJWTConfig())
	svc := NewRefreshService(nil, jwtSvc)

	if svc.jwtService != jwtSvc {
		t.Error("jwtService not set correctly")
	}
}

func TestRefreshToken_Struct(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	orgID := uuid.New()
	familyID := uuid.New()

	token := RefreshToken{
		ID:             uuid.New(),
		UserID:         userID,
		OrganizationID: &orgID,
		TokenHash:      "abc123hash",
		FamilyID:       familyID,
		ExpiresAt:      now.Add(7 * 24 * time.Hour),
		CreatedAt:      now,
	}

	if token.UserID != userID {
		t.Errorf("UserID = %v, want %v", token.UserID, userID)
	}
	if *token.OrganizationID != orgID {
		t.Errorf("OrganizationID = %v, want %v", *token.OrganizationID, orgID)
	}
	if token.FamilyID != familyID {
		t.Errorf("FamilyID = %v, want %v", token.FamilyID, familyID)
	}
	if token.RevokedAt != nil {
		t.Error("RevokedAt should be nil")
	}
	if token.ReplacedBy != nil {
		t.Error("ReplacedBy should be nil")
	}
}

func TestRefreshTokenErrors(t *testing.T) {
	// Test error constants
	if ErrTokenNotFound.Error() != "refresh token not found" {
		t.Errorf("ErrTokenNotFound = %q, want %q", ErrTokenNotFound.Error(), "refresh token not found")
	}
	if ErrTokenAlreadyUsed.Error() != "refresh token already used - possible token theft" {
		t.Errorf("ErrTokenAlreadyUsed = %q", ErrTokenAlreadyUsed.Error())
	}
	if ErrInvalidRefreshToken.Error() != "invalid refresh token" {
		t.Errorf("ErrInvalidRefreshToken = %q", ErrInvalidRefreshToken.Error())
	}
}

func TestRefreshToken_RevokedState(t *testing.T) {
	now := time.Now()
	replacementID := uuid.New()

	token := RefreshToken{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		FamilyID:   uuid.New(),
		ExpiresAt:  now.Add(24 * time.Hour),
		RevokedAt:  &now,
		ReplacedBy: &replacementID,
		CreatedAt:  now.Add(-1 * time.Hour),
	}

	// Token is revoked
	if token.RevokedAt == nil {
		t.Error("RevokedAt should not be nil")
	}

	// Token was replaced
	if token.ReplacedBy == nil {
		t.Error("ReplacedBy should not be nil")
	}
	if *token.ReplacedBy != replacementID {
		t.Errorf("ReplacedBy = %v, want %v", *token.ReplacedBy, replacementID)
	}
}

func TestTokenPair_Struct(t *testing.T) {
	now := time.Now()
	pair := TokenPair{
		AccessToken:  "access.token.here",
		RefreshToken: "refresh.token.here",
		TokenType:    "Bearer",
		ExpiresIn:    900,
		ExpiresAt:    now.Add(15 * time.Minute),
	}

	if pair.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want Bearer", pair.TokenType)
	}
	if pair.ExpiresIn != 900 {
		t.Errorf("ExpiresIn = %d, want 900", pair.ExpiresIn)
	}
}

// Note: Full integration tests for StoreRefreshToken, GetByHash, RotateToken,
// RevokeTokenFamily, Refresh, etc. require a database connection.
// These should be run as part of integration tests with a test database setup.
//
// The tests above verify:
// - Hash function correctness
// - Service initialization
// - Struct construction and field access
// - Error constant values
//
// For database-dependent tests, use:
//   go test -tags=integration ./internal/auth/... -run TestRefresh
