// Package auth provides refresh token management with rotation
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

var (
	ErrTokenNotFound      = errors.New("refresh token not found")
	ErrTokenAlreadyUsed   = errors.New("refresh token already used - possible token theft")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// RefreshToken represents a stored refresh token
type RefreshToken struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	TokenHash      string     `json:"-"` // Never expose
	FamilyID       uuid.UUID  `json:"family_id"`
	ExpiresAt      time.Time  `json:"expires_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	ReplacedBy     *uuid.UUID `json:"replaced_by,omitempty"`
	IPAddress      string     `json:"ip_address,omitempty"`
	UserAgent      string     `json:"user_agent,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// RefreshService manages refresh token storage and rotation
type RefreshService struct {
	pool       *pgxpool.Pool
	jwtService *JWTService
}

// NewRefreshService creates a new refresh token service
func NewRefreshService(pool *pgxpool.Pool, jwtService *JWTService) *RefreshService {
	return &RefreshService{
		pool:       pool,
		jwtService: jwtService,
	}
}

// StoreRefreshToken stores a new refresh token
func (s *RefreshService) StoreRefreshToken(ctx context.Context, token *RefreshToken, rawToken string) error {
	token.TokenHash = hashToken(rawToken)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, organization_id, token_hash, family_id, expires_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, token.ID, token.UserID, token.OrganizationID, token.TokenHash, token.FamilyID, token.ExpiresAt, token.IPAddress, token.UserAgent)

	return err
}

// GetByHash retrieves a refresh token by its hash
func (s *RefreshService) GetByHash(ctx context.Context, rawToken string) (*RefreshToken, error) {
	tokenHash := hashToken(rawToken)

	token := &RefreshToken{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, organization_id, token_hash, family_id, expires_at, revoked_at, replaced_by, ip_address, user_agent, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(
		&token.ID, &token.UserID, &token.OrganizationID, &token.TokenHash,
		&token.FamilyID, &token.ExpiresAt, &token.RevokedAt, &token.ReplacedBy,
		&token.IPAddress, &token.UserAgent, &token.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	return token, nil
}

// RotateToken rotates a refresh token - marks the old one as replaced and creates a new one
func (s *RefreshService) RotateToken(ctx context.Context, oldTokenID uuid.UUID, newToken *RefreshToken, rawNewToken string) error {
	newToken.TokenHash = hashToken(rawNewToken)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert new token
	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, organization_id, token_hash, family_id, expires_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, newToken.ID, newToken.UserID, newToken.OrganizationID, newToken.TokenHash,
		newToken.FamilyID, newToken.ExpiresAt, newToken.IPAddress, newToken.UserAgent)
	if err != nil {
		return fmt.Errorf("failed to insert new token: %w", err)
	}

	// Mark old token as replaced
	_, err = tx.Exec(ctx, `
		UPDATE refresh_tokens SET replaced_by = $1 WHERE id = $2
	`, newToken.ID, oldTokenID)
	if err != nil {
		return fmt.Errorf("failed to update old token: %w", err)
	}

	return tx.Commit(ctx)
}

// RevokeTokenFamily revokes all tokens in a family (used when token reuse is detected)
func (s *RefreshService) RevokeTokenFamily(ctx context.Context, familyID uuid.UUID) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = $1 WHERE family_id = $2 AND revoked_at IS NULL
	`, now, familyID)

	if err != nil {
		return fmt.Errorf("failed to revoke token family: %w", err)
	}

	log.Warn().
		Str("family_id", familyID.String()).
		Msg("revoked entire refresh token family due to potential token theft")

	return nil
}

// RevokeToken revokes a single token
func (s *RefreshService) RevokeToken(ctx context.Context, tokenID uuid.UUID) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = $1 WHERE id = $2
	`, now, tokenID)
	return err
}

// RevokeUserTokens revokes all tokens for a user (e.g., on password change or logout all)
func (s *RefreshService) RevokeUserTokens(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL
	`, now, userID)
	return err
}

// Refresh performs the token refresh with rotation
func (s *RefreshService) Refresh(ctx context.Context, refreshToken string, ip, userAgent string) (*TokenPair, error) {
	// 1. Validate the JWT structure
	claims, err := s.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRefreshToken, err)
	}

	// 2. Look up the token in storage
	storedToken, err := s.GetByHash(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}

	// 3. Check if token is revoked
	if storedToken.RevokedAt != nil {
		return nil, ErrTokenRevoked
	}

	// 4. CRITICAL: Check for token reuse (already rotated)
	if storedToken.ReplacedBy != nil {
		// This token was already used and rotated - possible token theft!
		// Revoke the entire token family
		if err := s.RevokeTokenFamily(ctx, storedToken.FamilyID); err != nil {
			log.Error().Err(err).Msg("failed to revoke token family on reuse detection")
		}
		return nil, ErrTokenAlreadyUsed
	}

	// 5. Check expiration (should be caught by JWT validation, but double-check)
	if time.Now().After(storedToken.ExpiresAt) {
		return nil, ErrExpiredToken
	}

	// 6. Parse user and org IDs
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID in claims: %w", err)
	}

	var orgID uuid.UUID
	if claims.OrganizationID != "" {
		orgID, _ = uuid.Parse(claims.OrganizationID)
	}

	// 7. Generate new token pair
	newPair, err := s.jwtService.GenerateTokenPair(userID, orgID, claims.Scopes, claims.SessionID, claims.GitHubLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new tokens: %w", err)
	}

	// 8. Extract expiry from the new refresh token
	newRefreshClaims, err := s.jwtService.ValidateRefreshToken(newPair.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new refresh token: %w", err)
	}

	// 9. Store new token and mark old as rotated
	newTokenRecord := &RefreshToken{
		ID:             uuid.New(),
		UserID:         userID,
		OrganizationID: storedToken.OrganizationID,
		FamilyID:       storedToken.FamilyID, // Keep the same family
		ExpiresAt:      newRefreshClaims.ExpiresAt.Time,
		IPAddress:      ip,
		UserAgent:      userAgent,
	}

	if err := s.RotateToken(ctx, storedToken.ID, newTokenRecord, newPair.RefreshToken); err != nil {
		return nil, fmt.Errorf("failed to rotate token: %w", err)
	}

	return newPair, nil
}

// CreateInitialTokens creates a new token pair and stores the refresh token
func (s *RefreshService) CreateInitialTokens(ctx context.Context, userID, orgID uuid.UUID, scopes []string, sessionID, githubLogin, ip, userAgent string) (*TokenPair, error) {
	// Generate token pair
	pair, err := s.jwtService.GenerateTokenPair(userID, orgID, scopes, sessionID, githubLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Parse refresh token to get expiry
	refreshClaims, err := s.jwtService.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to parse refresh token: %w", err)
	}

	// Store refresh token with new family ID
	tokenRecord := &RefreshToken{
		ID:             uuid.New(),
		UserID:         userID,
		OrganizationID: &orgID,
		FamilyID:       uuid.New(), // New family for new login
		ExpiresAt:      refreshClaims.ExpiresAt.Time,
		IPAddress:      ip,
		UserAgent:      userAgent,
	}

	if err := s.StoreRefreshToken(ctx, tokenRecord, pair.RefreshToken); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return pair, nil
}

// AddToBlacklist adds an access token JTI to the blacklist
func (s *RefreshService) AddToBlacklist(ctx context.Context, tokenJTI string, expiresAt time.Time, reason string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO token_blacklist (token_jti, expires_at, reason)
		VALUES ($1, $2, $3)
		ON CONFLICT (token_jti) DO NOTHING
	`, tokenJTI, expiresAt, reason)
	return err
}

// IsBlacklisted checks if a token JTI is blacklisted
func (s *RefreshService) IsBlacklisted(ctx context.Context, tokenJTI string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM token_blacklist WHERE token_jti = $1 AND expires_at > NOW())
	`, tokenJTI).Scan(&exists)
	return exists, err
}

// CleanupExpired removes expired tokens and blacklist entries
func (s *RefreshService) CleanupExpired(ctx context.Context) (int64, int64, error) {
	// Cleanup blacklist
	blacklistResult, err := s.pool.Exec(ctx, `DELETE FROM token_blacklist WHERE expires_at < NOW()`)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to cleanup blacklist: %w", err)
	}

	// Cleanup old refresh tokens (keep for 7 days after expiry for audit)
	refreshResult, err := s.pool.Exec(ctx, `
		DELETE FROM refresh_tokens WHERE expires_at < NOW() - INTERVAL '7 days'
	`)
	if err != nil {
		return blacklistResult.RowsAffected(), 0, fmt.Errorf("failed to cleanup refresh tokens: %w", err)
	}

	return blacklistResult.RowsAffected(), refreshResult.RowsAffected(), nil
}

// hashToken creates a SHA256 hash of a token
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
