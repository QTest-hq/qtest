package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// APIKey represents an API key for programmatic access
type APIKey struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	UserID         uuid.UUID  `json:"user_id"`
	Name           string     `json:"name"`
	KeyPrefix      string     `json:"key_prefix"` // First 8 chars of the key for display
	KeyHash        string     `json:"-"`          // SHA256 hash of the full key
	Scopes         []string   `json:"scopes"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// APIKeyWithSecret is returned when creating a new key (only time the secret is available)
type APIKeyWithSecret struct {
	APIKey
	Secret string `json:"secret"` // The actual API key secret (only returned on creation)
}

// APIKeyScope represents the permissions an API key has
type APIKeyScope string

const (
	ScopeReadRepos    APIKeyScope = "repos:read"
	ScopeWriteRepos   APIKeyScope = "repos:write"
	ScopeReadRuns     APIKeyScope = "runs:read"
	ScopeWriteRuns    APIKeyScope = "runs:write"
	ScopeReadTests    APIKeyScope = "tests:read"
	ScopeWriteTests   APIKeyScope = "tests:write"
	ScopeReadJobs     APIKeyScope = "jobs:read"
	ScopeWriteJobs    APIKeyScope = "jobs:write"
	ScopeReadMutation APIKeyScope = "mutation:read"
	ScopeAdmin        APIKeyScope = "admin"
)

// GenerateAPIKey generates a new API key with prefix "qtest_"
func GenerateAPIKey() (string, string, string, error) {
	// Generate 32 random bytes
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Create the full key with prefix
	fullKey := "qtest_" + hex.EncodeToString(b)

	// Get prefix for display (first 8 chars after qtest_)
	prefix := fullKey[:14] // "qtest_" + 8 chars

	// Hash the full key for storage
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := hex.EncodeToString(hash[:])

	return fullKey, prefix, keyHash, nil
}

// HashAPIKey hashes an API key for comparison
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// CreateAPIKey creates a new API key
func (s *Store) CreateAPIKey(ctx context.Context, orgID, userID uuid.UUID, name string, scopes []string, expiresAt *time.Time) (*APIKeyWithSecret, error) {
	secret, prefix, keyHash, err := GenerateAPIKey()
	if err != nil {
		return nil, err
	}

	key := &APIKey{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Name:           name,
		KeyPrefix:      prefix,
		KeyHash:        keyHash,
		Scopes:         scopes,
		ExpiresAt:      expiresAt,
		CreatedAt:      time.Now(),
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO api_keys (id, organization_id, user_id, name, key_prefix, key_hash, scopes, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, key.ID, key.OrganizationID, key.UserID, key.Name, key.KeyPrefix, key.KeyHash, key.Scopes, key.ExpiresAt, key.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return &APIKeyWithSecret{
		APIKey: *key,
		Secret: secret,
	}, nil
}

// GetAPIKeyByHash retrieves an API key by its hash
func (s *Store) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	key := &APIKey{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, organization_id, user_id, name, key_prefix, key_hash, scopes, expires_at, last_used_at, revoked_at, created_at
		FROM api_keys
		WHERE key_hash = $1
	`, keyHash).Scan(&key.ID, &key.OrganizationID, &key.UserID, &key.Name, &key.KeyPrefix, &key.KeyHash,
		&key.Scopes, &key.ExpiresAt, &key.LastUsedAt, &key.RevokedAt, &key.CreatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return key, nil
}

// ValidateAPIKey validates an API key and returns it if valid
func (s *Store) ValidateAPIKey(ctx context.Context, apiKey string) (*APIKey, error) {
	keyHash := HashAPIKey(apiKey)

	key, err := s.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, nil
	}

	// Check if revoked
	if key.RevokedAt != nil {
		return nil, nil
	}

	// Check if expired
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return nil, nil
	}

	// Update last used timestamp
	go func() {
		s.pool.Exec(context.Background(), `
			UPDATE api_keys SET last_used_at = $2 WHERE id = $1
		`, key.ID, time.Now())
	}()

	return key, nil
}

// ListAPIKeysByOrg lists all API keys for an organization
func (s *Store) ListAPIKeysByOrg(ctx context.Context, orgID uuid.UUID) ([]APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, organization_id, user_id, name, key_prefix, key_hash, scopes, expires_at, last_used_at, revoked_at, created_at
		FROM api_keys
		WHERE organization_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	keys := make([]APIKey, 0)
	for rows.Next() {
		var key APIKey
		err := rows.Scan(&key.ID, &key.OrganizationID, &key.UserID, &key.Name, &key.KeyPrefix, &key.KeyHash,
			&key.Scopes, &key.ExpiresAt, &key.LastUsedAt, &key.RevokedAt, &key.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, nil
}

// ListAPIKeysByUser lists all API keys created by a user
func (s *Store) ListAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, organization_id, user_id, name, key_prefix, key_hash, scopes, expires_at, last_used_at, revoked_at, created_at
		FROM api_keys
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	keys := make([]APIKey, 0)
	for rows.Next() {
		var key APIKey
		err := rows.Scan(&key.ID, &key.OrganizationID, &key.UserID, &key.Name, &key.KeyPrefix, &key.KeyHash,
			&key.Scopes, &key.ExpiresAt, &key.LastUsedAt, &key.RevokedAt, &key.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, nil
}

// RevokeAPIKey revokes an API key
func (s *Store) RevokeAPIKey(ctx context.Context, keyID, userID uuid.UUID) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $3
		WHERE id = $1 AND (user_id = $2 OR organization_id IN (
			SELECT organization_id FROM organization_members
			WHERE user_id = $2 AND role IN ('owner', 'admin')
		))
	`, keyID, userID, time.Now())

	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("API key not found or permission denied")
	}

	return nil
}

// SetAPIKeyGracePeriod sets a grace period expiration on an API key
// This is used when rotating keys to allow the old key to work during transition
func (s *Store) SetAPIKeyGracePeriod(ctx context.Context, keyID uuid.UUID, graceEnds time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET expires_at = $2
		WHERE id = $1 AND revoked_at IS NULL
	`, keyID, graceEnds)

	if err != nil {
		return fmt.Errorf("failed to set grace period on API key: %w", err)
	}

	return nil
}

// GetAPIKeyByID retrieves an API key by its ID
func (s *Store) GetAPIKeyByID(ctx context.Context, keyID uuid.UUID) (*APIKey, error) {
	key := &APIKey{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, organization_id, user_id, name, key_prefix, key_hash, scopes, expires_at, last_used_at, revoked_at, created_at
		FROM api_keys
		WHERE id = $1
	`, keyID).Scan(&key.ID, &key.OrganizationID, &key.UserID, &key.Name, &key.KeyPrefix, &key.KeyHash,
		&key.Scopes, &key.ExpiresAt, &key.LastUsedAt, &key.RevokedAt, &key.CreatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return key, nil
}

// HasScope checks if an API key has a specific scope
func (k *APIKey) HasScope(scope APIKeyScope) bool {
	for _, s := range k.Scopes {
		if s == string(scope) || s == string(ScopeAdmin) {
			return true
		}
	}
	return false
}

// IsValid checks if an API key is valid (not revoked and not expired)
func (k *APIKey) IsValid() bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return false
	}
	return true
}

// APIKeyValidatorAdapter adapts the Store to implement auth.APIKeyValidator
type APIKeyValidatorAdapter struct {
	store *Store
}

// NewAPIKeyValidatorAdapter creates a new adapter
func NewAPIKeyValidatorAdapter(store *Store) *APIKeyValidatorAdapter {
	return &APIKeyValidatorAdapter{store: store}
}

// APIKeyAuthInfo matches auth.APIKeyInfo to avoid import cycles
type APIKeyAuthInfo struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Scopes         []string
}

// ValidateAPIKeyForAuth validates an API key and returns its info for the auth package
// This must be wrapped by the server to return auth.APIKeyInfo
func (a *APIKeyValidatorAdapter) ValidateAPIKeyForAuth(ctx context.Context, apiKey string) (*APIKeyAuthInfo, error) {
	key, err := a.store.ValidateAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, nil
	}

	return &APIKeyAuthInfo{
		ID:             key.ID,
		OrganizationID: key.OrganizationID,
		UserID:         key.UserID,
		Scopes:         key.Scopes,
	}, nil
}
