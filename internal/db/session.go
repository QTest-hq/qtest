package db

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Session represents a database-backed user session
type Session struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	AccessToken  string    `json:"-"` // Not serialized for security
	RefreshToken *string   `json:"-"`
	ExpiresAt    time.Time `json:"expires_at"`
	LastAccess   time.Time `json:"last_access"`
	IPAddress    *string   `json:"ip_address,omitempty"`
	UserAgent    *string   `json:"user_agent,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// SessionWithUser includes user details
type SessionWithUser struct {
	Session
	User *User `json:"user"`
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// CreateSession creates a new session in the database
func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, accessToken, refreshToken, ipAddress, userAgent string, ttl time.Duration) (*Session, error) {
	session := &Session{
		ID:          uuid.New(),
		UserID:      userID,
		AccessToken: accessToken,
		ExpiresAt:   time.Now().Add(ttl),
		LastAccess:  time.Now(),
		CreatedAt:   time.Now(),
	}

	if refreshToken != "" {
		session.RefreshToken = &refreshToken
	}
	if ipAddress != "" {
		session.IPAddress = &ipAddress
	}
	if userAgent != "" {
		session.UserAgent = &userAgent
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, access_token, refresh_token, expires_at, last_access, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, session.ID, session.UserID, session.AccessToken, session.RefreshToken, session.ExpiresAt, session.LastAccess, session.IPAddress, session.UserAgent, session.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// GetSessionByID retrieves a session by ID
func (s *Store) GetSessionByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	session := &Session{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, access_token, refresh_token, expires_at, last_access, ip_address, user_agent, created_at
		FROM sessions
		WHERE id = $1
	`, id).Scan(&session.ID, &session.UserID, &session.AccessToken, &session.RefreshToken, &session.ExpiresAt, &session.LastAccess, &session.IPAddress, &session.UserAgent, &session.CreatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// GetSessionWithUser retrieves a session with user details
func (s *Store) GetSessionWithUser(ctx context.Context, id uuid.UUID) (*SessionWithUser, error) {
	result := &SessionWithUser{
		User: &User{},
	}

	err := s.pool.QueryRow(ctx, `
		SELECT s.id, s.user_id, s.access_token, s.refresh_token, s.expires_at, s.last_access, s.ip_address, s.user_agent, s.created_at,
		       u.id, u.github_id, u.github_login, u.email, u.name, u.avatar_url, u.is_active, u.created_at, u.updated_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.id = $1
	`, id).Scan(
		&result.Session.ID, &result.Session.UserID, &result.Session.AccessToken, &result.Session.RefreshToken,
		&result.Session.ExpiresAt, &result.Session.LastAccess, &result.Session.IPAddress, &result.Session.UserAgent, &result.Session.CreatedAt,
		&result.User.ID, &result.User.GitHubID, &result.User.GitHubLogin, &result.User.Email, &result.User.Name,
		&result.User.AvatarURL, &result.User.IsActive, &result.User.CreatedAt, &result.User.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session with user: %w", err)
	}

	return result, nil
}

// UpdateSessionLastAccess updates the last access time
func (s *Store) UpdateSessionLastAccess(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions SET last_access = $2 WHERE id = $1
	`, id, time.Now())

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// ExtendSession extends the session expiry
func (s *Store) ExtendSession(ctx context.Context, id uuid.UUID, ttl time.Duration) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions SET expires_at = $2, last_access = $3 WHERE id = $1
	`, id, time.Now().Add(ttl), time.Now())

	if err != nil {
		return fmt.Errorf("failed to extend session: %w", err)
	}

	return nil
}

// DeleteSession deletes a session
func (s *Store) DeleteSession(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// DeleteUserSessions deletes all sessions for a user
func (s *Store) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}
	return nil
}

// CleanupExpiredSessions removes expired sessions
func (s *Store) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	result, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < $1`, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup sessions: %w", err)
	}
	return result.RowsAffected(), nil
}

// ListUserSessions lists active sessions for a user
func (s *Store) ListUserSessions(ctx context.Context, userID uuid.UUID) ([]Session, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, access_token, refresh_token, expires_at, last_access, ip_address, user_agent, created_at
		FROM sessions
		WHERE user_id = $1 AND expires_at > $2
		ORDER BY last_access DESC
	`, userID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		err := rows.Scan(&s.ID, &s.UserID, &s.AccessToken, &s.RefreshToken, &s.ExpiresAt, &s.LastAccess, &s.IPAddress, &s.UserAgent, &s.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// GenerateSessionToken generates a secure random token
func GenerateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
