package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
)

func generateTestKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test RSA keys: %v", err)
	}
	return privateKey, &privateKey.PublicKey
}

func TestNewJWTServiceWithKeys(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)

	tests := []struct {
		name       string
		privateKey *rsa.PrivateKey
		publicKey  *rsa.PublicKey
		cfg        JWTConfig
		wantSign   bool
		wantVerify bool
	}{
		{
			name:       "with both keys",
			privateKey: privateKey,
			publicKey:  publicKey,
			cfg:        DefaultJWTConfig(),
			wantSign:   true,
			wantVerify: true,
		},
		{
			name:       "with only private key (derives public)",
			privateKey: privateKey,
			publicKey:  nil,
			cfg:        DefaultJWTConfig(),
			wantSign:   true,
			wantVerify: true,
		},
		{
			name:       "with only public key",
			privateKey: nil,
			publicKey:  publicKey,
			cfg:        DefaultJWTConfig(),
			wantSign:   false,
			wantVerify: true,
		},
		{
			name:       "with no keys",
			privateKey: nil,
			publicKey:  nil,
			cfg:        DefaultJWTConfig(),
			wantSign:   false,
			wantVerify: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewJWTServiceWithKeys(tt.privateKey, tt.publicKey, tt.cfg)
			if svc.CanSign() != tt.wantSign {
				t.Errorf("CanSign() = %v, want %v", svc.CanSign(), tt.wantSign)
			}
			if svc.CanVerify() != tt.wantVerify {
				t.Errorf("CanVerify() = %v, want %v", svc.CanVerify(), tt.wantVerify)
			}
		})
	}
}

func TestJWTService_GenerateTokenPair(t *testing.T) {
	privateKey, _ := generateTestKeys(t)
	svc := NewJWTServiceWithKeys(privateKey, nil, JWTConfig{
		Issuer:     "test-issuer",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
		KeyID:      "test-v1",
	})

	userID := uuid.New()
	orgID := uuid.New()
	scopes := []string{"read:repos", "write:tests"}
	sessionID := "test-session-123"
	githubLogin := "testuser"

	pair, err := svc.GenerateTokenPair(userID, orgID, scopes, sessionID, githubLogin)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("AccessToken should not be empty")
	}
	if pair.RefreshToken == "" {
		t.Error("RefreshToken should not be empty")
	}
	if pair.TokenType != "Bearer" {
		t.Errorf("TokenType = %v, want Bearer", pair.TokenType)
	}
	if pair.ExpiresIn != int((15 * time.Minute).Seconds()) {
		t.Errorf("ExpiresIn = %v, want %v", pair.ExpiresIn, int((15*time.Minute).Seconds()))
	}
}

func TestJWTService_GenerateTokenPair_NoPrivateKey(t *testing.T) {
	_, publicKey := generateTestKeys(t)
	svc := NewJWTServiceWithKeys(nil, publicKey, DefaultJWTConfig())

	_, err := svc.GenerateTokenPair(uuid.New(), uuid.New(), nil, "session", "user")
	if err == nil {
		t.Error("GenerateTokenPair() should fail without private key")
	}
}

func TestJWTService_ValidateAccessToken(t *testing.T) {
	privateKey, _ := generateTestKeys(t)
	svc := NewJWTServiceWithKeys(privateKey, nil, JWTConfig{
		Issuer:     "test-issuer",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
		KeyID:      "test-v1",
	})

	userID := uuid.New()
	orgID := uuid.New()
	scopes := []string{"read:repos"}
	sessionID := "session-123"
	githubLogin := "testuser"

	pair, err := svc.GenerateTokenPair(userID, orgID, scopes, sessionID, githubLogin)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Test valid access token
	claims, err := svc.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	if claims.UserID != userID.String() {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID.String())
	}
	if claims.OrganizationID != orgID.String() {
		t.Errorf("OrganizationID = %v, want %v", claims.OrganizationID, orgID.String())
	}
	if claims.SessionID != sessionID {
		t.Errorf("SessionID = %v, want %v", claims.SessionID, sessionID)
	}
	if claims.GitHubLogin != githubLogin {
		t.Errorf("GitHubLogin = %v, want %v", claims.GitHubLogin, githubLogin)
	}
	if claims.TokenType != string(TokenTypeAccess) {
		t.Errorf("TokenType = %v, want %v", claims.TokenType, TokenTypeAccess)
	}

	// Test using refresh token as access token (should fail)
	_, err = svc.ValidateAccessToken(pair.RefreshToken)
	if err != ErrInvalidTokenType {
		t.Errorf("ValidateAccessToken(refreshToken) error = %v, want %v", err, ErrInvalidTokenType)
	}
}

func TestJWTService_ValidateRefreshToken(t *testing.T) {
	privateKey, _ := generateTestKeys(t)
	svc := NewJWTServiceWithKeys(privateKey, nil, DefaultJWTConfig())

	userID := uuid.New()
	orgID := uuid.New()

	pair, err := svc.GenerateTokenPair(userID, orgID, nil, "session-123", "testuser")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Test valid refresh token
	claims, err := svc.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken() error = %v", err)
	}

	if claims.TokenType != string(TokenTypeRefresh) {
		t.Errorf("TokenType = %v, want %v", claims.TokenType, TokenTypeRefresh)
	}

	// Test using access token as refresh token (should fail)
	_, err = svc.ValidateRefreshToken(pair.AccessToken)
	if err != ErrInvalidTokenType {
		t.Errorf("ValidateRefreshToken(accessToken) error = %v, want %v", err, ErrInvalidTokenType)
	}
}

func TestJWTService_ValidateToken_Invalid(t *testing.T) {
	privateKey, _ := generateTestKeys(t)
	svc := NewJWTServiceWithKeys(privateKey, nil, DefaultJWTConfig())

	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"malformed token", "not.a.token"},
		{"garbage", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.garbage.moregarbge"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ValidateAccessToken(tt.token)
			if err == nil {
				t.Error("ValidateAccessToken() should fail for invalid token")
			}
		})
	}
}

func TestJWTService_ValidateToken_WrongKey(t *testing.T) {
	privateKey1, _ := generateTestKeys(t)
	privateKey2, _ := generateTestKeys(t)

	svc1 := NewJWTServiceWithKeys(privateKey1, nil, DefaultJWTConfig())
	svc2 := NewJWTServiceWithKeys(privateKey2, nil, DefaultJWTConfig())

	pair, err := svc1.GenerateTokenPair(uuid.New(), uuid.New(), nil, "session", "user")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Try to validate with different key
	_, err = svc2.ValidateAccessToken(pair.AccessToken)
	if err == nil {
		t.Error("ValidateAccessToken() should fail with wrong key")
	}
}

func TestJWTService_GetTokenID(t *testing.T) {
	privateKey, _ := generateTestKeys(t)
	svc := NewJWTServiceWithKeys(privateKey, nil, DefaultJWTConfig())

	pair, err := svc.GenerateTokenPair(uuid.New(), uuid.New(), nil, "session", "user")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	jti, err := svc.GetTokenID(pair.AccessToken)
	if err != nil {
		t.Fatalf("GetTokenID() error = %v", err)
	}

	// JTI should be a valid UUID
	if _, err := uuid.Parse(jti); err != nil {
		t.Errorf("JTI is not a valid UUID: %v", jti)
	}
}

func TestJWTService_GenerateAccessToken(t *testing.T) {
	privateKey, _ := generateTestKeys(t)
	svc := NewJWTServiceWithKeys(privateKey, nil, JWTConfig{
		AccessTTL: 30 * time.Minute,
	})

	userID := uuid.New()
	orgID := uuid.New()

	token, expiry, err := svc.GenerateAccessToken(userID, orgID, []string{"read"}, "session", "user")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	if token == "" {
		t.Error("token should not be empty")
	}

	// Expiry should be approximately 30 minutes from now
	expectedExpiry := time.Now().Add(30 * time.Minute)
	if expiry.Before(expectedExpiry.Add(-1*time.Second)) || expiry.After(expectedExpiry.Add(1*time.Second)) {
		t.Errorf("expiry = %v, want approximately %v", expiry, expectedExpiry)
	}

	// Validate the token
	claims, err := svc.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}
	if claims.UserID != userID.String() {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID.String())
	}
}

func TestDefaultJWTConfig(t *testing.T) {
	cfg := DefaultJWTConfig()

	if cfg.Issuer != "https://api.qtest.io" {
		t.Errorf("Issuer = %v, want https://api.qtest.io", cfg.Issuer)
	}
	if cfg.AccessTTL != 15*time.Minute {
		t.Errorf("AccessTTL = %v, want 15m", cfg.AccessTTL)
	}
	if cfg.RefreshTTL != 7*24*time.Hour {
		t.Errorf("RefreshTTL = %v, want 7 days", cfg.RefreshTTL)
	}
	if cfg.KeyID != "v1" {
		t.Errorf("KeyID = %v, want v1", cfg.KeyID)
	}
}

func TestJWTService_IsConfigured(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)

	tests := []struct {
		name       string
		privateKey *rsa.PrivateKey
		publicKey  *rsa.PublicKey
		want       bool
	}{
		{"both keys", privateKey, publicKey, true},
		{"private only", privateKey, nil, true},
		{"public only", nil, publicKey, true},
		{"no keys", nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewJWTServiceWithKeys(tt.privateKey, tt.publicKey, DefaultJWTConfig())
			if svc.IsConfigured() != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", svc.IsConfigured(), tt.want)
			}
		})
	}
}

func TestQTestClaims(t *testing.T) {
	privateKey, _ := generateTestKeys(t)
	svc := NewJWTServiceWithKeys(privateKey, nil, DefaultJWTConfig())

	// Test with all optional fields populated
	userID := uuid.New()
	orgID := uuid.New()
	scopes := []string{"read:repos", "write:tests", "admin:org"}
	sessionID := "session-abc-123"
	githubLogin := "octocat"

	pair, err := svc.GenerateTokenPair(userID, orgID, scopes, sessionID, githubLogin)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := svc.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	// Check all custom claims
	if len(claims.Scopes) != len(scopes) {
		t.Errorf("Scopes length = %d, want %d", len(claims.Scopes), len(scopes))
	}
	for i, scope := range scopes {
		if claims.Scopes[i] != scope {
			t.Errorf("Scopes[%d] = %v, want %v", i, claims.Scopes[i], scope)
		}
	}

	// Check registered claims
	if claims.Issuer != "https://api.qtest.io" {
		t.Errorf("Issuer = %v, want https://api.qtest.io", claims.Issuer)
	}
	if claims.Subject != userID.String() {
		t.Errorf("Subject = %v, want %v", claims.Subject, userID.String())
	}
}
