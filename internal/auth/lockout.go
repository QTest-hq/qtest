// Package auth provides authentication and authorization functionality
package auth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// IdentifierType represents the type of identifier being tracked for lockout
type IdentifierType string

const (
	IdentifierTypeIP       IdentifierType = "ip"
	IdentifierTypeUserID   IdentifierType = "user_id"
	IdentifierTypeGitHubID IdentifierType = "github_id"
)

// LockoutService handles account lockout after failed login attempts
type LockoutService struct {
	pool              *pgxpool.Pool
	maxAttempts       int
	lockoutDuration   time.Duration
	windowDuration    time.Duration
}

// LockoutConfig holds configuration for the lockout service
type LockoutConfig struct {
	MaxAttempts       int           // Maximum failed attempts before lockout
	LockoutDuration   time.Duration // How long to lock out
	WindowDuration    time.Duration // Time window for counting attempts
}

// DefaultLockoutConfig returns sensible default lockout settings
func DefaultLockoutConfig() LockoutConfig {
	return LockoutConfig{
		MaxAttempts:     5,
		LockoutDuration: 15 * time.Minute,
		WindowDuration:  15 * time.Minute,
	}
}

// NewLockoutService creates a new lockout service
func NewLockoutService(pool *pgxpool.Pool, cfg LockoutConfig) *LockoutService {
	return &LockoutService{
		pool:            pool,
		maxAttempts:     cfg.MaxAttempts,
		lockoutDuration: cfg.LockoutDuration,
		windowDuration:  cfg.WindowDuration,
	}
}

// RecordAttempt records a login attempt (success or failure)
func (s *LockoutService) RecordAttempt(ctx context.Context, identifier string, identifierType IdentifierType, success bool, ip, userAgent string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO login_attempts (identifier, identifier_type, success, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5)
	`, identifier, string(identifierType), success, ip, userAgent)

	if err != nil {
		log.Error().Err(err).
			Str("identifier", identifier).
			Str("type", string(identifierType)).
			Bool("success", success).
			Msg("failed to record login attempt")
		return err
	}

	// If successful login, clear previous failed attempts for this identifier
	if success {
		go s.clearFailedAttempts(context.Background(), identifier, identifierType)
	}

	return nil
}

// IsLocked checks if an identifier is currently locked out
// Returns: locked status, remaining lockout duration, error
func (s *LockoutService) IsLocked(ctx context.Context, identifier string, identifierType IdentifierType) (bool, time.Duration, error) {
	// Count recent failed attempts within the window
	var failedCount int
	var lastAttempt time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(MAX(created_at), NOW())
		FROM login_attempts
		WHERE identifier = $1
		  AND identifier_type = $2
		  AND success = false
		  AND created_at > $3
	`, identifier, string(identifierType), time.Now().Add(-s.windowDuration)).Scan(&failedCount, &lastAttempt)

	if err != nil {
		log.Error().Err(err).
			Str("identifier", identifier).
			Msg("failed to check lockout status")
		return false, 0, err
	}

	// Not enough failures to trigger lockout
	if failedCount < s.maxAttempts {
		return false, 0, nil
	}

	// Calculate remaining lockout time
	lockoutEnds := lastAttempt.Add(s.lockoutDuration)
	remaining := time.Until(lockoutEnds)

	if remaining <= 0 {
		// Lockout has expired
		return false, 0, nil
	}

	log.Warn().
		Str("identifier", identifier).
		Str("type", string(identifierType)).
		Int("failed_attempts", failedCount).
		Dur("remaining", remaining).
		Msg("account is locked out")

	return true, remaining, nil
}

// GetFailedAttempts returns the number of recent failed attempts
func (s *LockoutService) GetFailedAttempts(ctx context.Context, identifier string, identifierType IdentifierType) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM login_attempts
		WHERE identifier = $1
		  AND identifier_type = $2
		  AND success = false
		  AND created_at > $3
	`, identifier, string(identifierType), time.Now().Add(-s.windowDuration)).Scan(&count)

	return count, err
}

// clearFailedAttempts clears failed attempts after a successful login
func (s *LockoutService) clearFailedAttempts(ctx context.Context, identifier string, identifierType IdentifierType) {
	// We don't actually delete the records (for audit purposes)
	// Instead, we could mark them as "cleared" or just let them age out
	// For now, we'll let them naturally expire based on the window duration
	log.Debug().
		Str("identifier", identifier).
		Str("type", string(identifierType)).
		Msg("successful login - failed attempts will age out")
}

// CleanupOldAttempts removes login attempts older than retention period
// This should be called periodically (e.g., via a cron job or background task)
func (s *LockoutService) CleanupOldAttempts(ctx context.Context, retentionHours int) (int64, error) {
	result, err := s.pool.Exec(ctx, `
		SELECT cleanup_login_attempts($1)
	`, retentionHours)

	if err != nil {
		// Try direct delete if function doesn't exist
		result, err = s.pool.Exec(ctx, `
			DELETE FROM login_attempts
			WHERE created_at < NOW() - ($1 || ' hours')::INTERVAL
		`, retentionHours)
		if err != nil {
			return 0, err
		}
	}

	return result.RowsAffected(), nil
}

// LockoutStatus represents the current lockout status for an identifier
type LockoutStatus struct {
	IsLocked         bool          `json:"is_locked"`
	FailedAttempts   int           `json:"failed_attempts"`
	MaxAttempts      int           `json:"max_attempts"`
	RemainingLockout time.Duration `json:"remaining_lockout,omitempty"`
	AttemptsRemaining int          `json:"attempts_remaining,omitempty"`
}

// GetStatus returns the full lockout status for an identifier
func (s *LockoutService) GetStatus(ctx context.Context, identifier string, identifierType IdentifierType) (*LockoutStatus, error) {
	locked, remaining, err := s.IsLocked(ctx, identifier, identifierType)
	if err != nil {
		return nil, err
	}

	failedAttempts, err := s.GetFailedAttempts(ctx, identifier, identifierType)
	if err != nil {
		return nil, err
	}

	return &LockoutStatus{
		IsLocked:          locked,
		FailedAttempts:    failedAttempts,
		MaxAttempts:       s.maxAttempts,
		RemainingLockout:  remaining,
		AttemptsRemaining: s.maxAttempts - failedAttempts,
	}, nil
}
