// Package auth provides JWT token generation and validation
package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidTokenType = errors.New("invalid token type")
	ErrTokenRevoked     = errors.New("token has been revoked")
	ErrMissingClaims    = errors.New("missing required claims")
)

// TokenType represents the type of JWT token
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// QTestClaims represents the custom claims for QTest JWT tokens
type QTestClaims struct {
	jwt.RegisteredClaims
	UserID         string   `json:"uid"`
	OrganizationID string   `json:"oid,omitempty"`
	Scopes         []string `json:"scopes,omitempty"`
	SessionID      string   `json:"sid"`
	TokenType      string   `json:"type"`
	GitHubLogin    string   `json:"ghl,omitempty"`
}

// TokenPair represents an access and refresh token pair
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	PrivateKeyPath string
	PrivateKeyPEM  string
	PublicKeyPath  string
	Issuer         string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	KeyID          string
}

// DefaultJWTConfig returns default JWT configuration
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		Issuer:     "https://api.qtest.io",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
		KeyID:      "v1",
	}
}

// JWTService handles JWT token generation and validation
type JWTService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	issuer     string
	keyID      string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewJWTService creates a new JWT service from configuration
func NewJWTService(cfg JWTConfig) (*JWTService, error) {
	var privateKey *rsa.PrivateKey
	var publicKey *rsa.PublicKey
	var err error

	// Load private key
	if cfg.PrivateKeyPEM != "" {
		privateKey, err = parsePrivateKey([]byte(cfg.PrivateKeyPEM))
	} else if cfg.PrivateKeyPath != "" {
		data, readErr := os.ReadFile(cfg.PrivateKeyPath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", readErr)
		}
		privateKey, err = parsePrivateKey(data)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Load public key or derive from private key
	if cfg.PublicKeyPath != "" {
		data, readErr := os.ReadFile(cfg.PublicKeyPath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read public key file: %w", readErr)
		}
		publicKey, err = parsePublicKey(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
	} else if privateKey != nil {
		publicKey = &privateKey.PublicKey
	}

	// Set defaults
	if cfg.Issuer == "" {
		cfg.Issuer = "https://api.qtest.io"
	}
	if cfg.AccessTTL == 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL == 0 {
		cfg.RefreshTTL = 7 * 24 * time.Hour
	}
	if cfg.KeyID == "" {
		cfg.KeyID = "v1"
	}

	return &JWTService{
		privateKey: privateKey,
		publicKey:  publicKey,
		issuer:     cfg.Issuer,
		keyID:      cfg.KeyID,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
	}, nil
}

// NewJWTServiceWithKeys creates a JWT service with provided keys
func NewJWTServiceWithKeys(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey, cfg JWTConfig) *JWTService {
	if publicKey == nil && privateKey != nil {
		publicKey = &privateKey.PublicKey
	}

	if cfg.Issuer == "" {
		cfg.Issuer = "https://api.qtest.io"
	}
	if cfg.AccessTTL == 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL == 0 {
		cfg.RefreshTTL = 7 * 24 * time.Hour
	}
	if cfg.KeyID == "" {
		cfg.KeyID = "v1"
	}

	return &JWTService{
		privateKey: privateKey,
		publicKey:  publicKey,
		issuer:     cfg.Issuer,
		keyID:      cfg.KeyID,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
	}
}

// GenerateTokenPair generates both access and refresh tokens
func (s *JWTService) GenerateTokenPair(userID, orgID uuid.UUID, scopes []string, sessionID, githubLogin string) (*TokenPair, error) {
	if s.privateKey == nil {
		return nil, errors.New("private key not configured")
	}

	now := time.Now()
	accessExpiry := now.Add(s.accessTTL)
	refreshExpiry := now.Add(s.refreshTTL)

	// Generate access token
	accessClaims := QTestClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
		UserID:         userID.String(),
		OrganizationID: orgID.String(),
		Scopes:         scopes,
		SessionID:      sessionID,
		TokenType:      string(TokenTypeAccess),
		GitHubLogin:    githubLogin,
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, accessClaims)
	accessToken.Header["kid"] = s.keyID
	accessTokenString, err := accessToken.SignedString(s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate refresh token
	refreshClaims := QTestClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(refreshExpiry),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
		UserID:         userID.String(),
		OrganizationID: orgID.String(),
		Scopes:         scopes,
		SessionID:      sessionID,
		TokenType:      string(TokenTypeRefresh),
		GitHubLogin:    githubLogin,
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodRS256, refreshClaims)
	refreshToken.Header["kid"] = s.keyID
	refreshTokenString, err := refreshToken.SignedString(s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.accessTTL.Seconds()),
		ExpiresAt:    accessExpiry,
	}, nil
}

// GenerateAccessToken generates only an access token
func (s *JWTService) GenerateAccessToken(userID, orgID uuid.UUID, scopes []string, sessionID, githubLogin string) (string, time.Time, error) {
	if s.privateKey == nil {
		return "", time.Time{}, errors.New("private key not configured")
	}

	now := time.Now()
	expiry := now.Add(s.accessTTL)

	claims := QTestClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiry),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
		UserID:         userID.String(),
		OrganizationID: orgID.String(),
		Scopes:         scopes,
		SessionID:      sessionID,
		TokenType:      string(TokenTypeAccess),
		GitHubLogin:    githubLogin,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.keyID
	tokenString, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiry, nil
}

// ValidateAccessToken validates an access token and returns the claims
func (s *JWTService) ValidateAccessToken(tokenString string) (*QTestClaims, error) {
	claims, err := s.validateToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != string(TokenTypeAccess) {
		return nil, ErrInvalidTokenType
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token and returns the claims
func (s *JWTService) ValidateRefreshToken(tokenString string) (*QTestClaims, error) {
	claims, err := s.validateToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != string(TokenTypeRefresh) {
		return nil, ErrInvalidTokenType
	}

	return claims, nil
}

// validateToken validates a JWT token and returns the claims
func (s *JWTService) validateToken(tokenString string) (*QTestClaims, error) {
	if s.publicKey == nil {
		return nil, errors.New("public key not configured")
	}

	token, err := jwt.ParseWithClaims(tokenString, &QTestClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.publicKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*QTestClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Validate required claims
	if claims.UserID == "" {
		return nil, ErrMissingClaims
	}

	return claims, nil
}

// GetTokenID extracts the JTI (token ID) from a token without full validation.
//
// SECURITY WARNING: This function uses ParseUnverified and does NOT validate the
// token signature. It should ONLY be used for non-security-critical operations
// such as logging or metrics. NEVER use this function to make authorization
// decisions or to check token revocation status - always validate the token
// first using ValidateAccessToken or ValidateRefreshToken.
//
// For security-critical operations, use ValidateAccessToken first, then extract
// the JTI from the validated claims.
func (s *JWTService) GetTokenID(tokenString string) (string, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &QTestClaims{})
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(*QTestClaims)
	if !ok {
		return "", ErrInvalidToken
	}

	return claims.ID, nil
}

// IsConfigured returns true if the JWT service has keys configured
func (s *JWTService) IsConfigured() bool {
	return s.privateKey != nil || s.publicKey != nil
}

// CanSign returns true if the service can sign tokens
func (s *JWTService) CanSign() bool {
	return s.privateKey != nil
}

// CanVerify returns true if the service can verify tokens
func (s *JWTService) CanVerify() bool {
	return s.publicKey != nil
}

// Note: parsePrivateKey is defined in github_app.go and shared across the auth package

// parsePublicKey parses an RSA public key from PEM data
func parsePublicKey(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	// Try PKIX first
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("key is not an RSA public key")
		}
		return rsaPub, nil
	}

	// Try PKCS#1
	return x509.ParsePKCS1PublicKey(block.Bytes)
}
