package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// User represents a user account
type User struct {
	ID          uuid.UUID  `json:"id"`
	GitHubID    int64      `json:"github_id"`
	GitHubLogin string     `json:"github_login"`
	Email       *string    `json:"email,omitempty"`
	Name        *string    `json:"name,omitempty"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateUser creates a new user (triggers auto-create personal org)
func (s *Store) CreateUser(ctx context.Context, user *User) error {
	user.ID = uuid.New()
	user.IsActive = true
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, github_id, github_login, email, name, avatar_url, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, user.ID, user.GitHubID, user.GitHubLogin, user.Email, user.Name, user.AvatarURL, user.IsActive, user.CreatedAt, user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByID retrieves a user by ID
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	user := &User{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, github_id, github_login, email, name, avatar_url, is_active, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id).Scan(&user.ID, &user.GitHubID, &user.GitHubLogin, &user.Email, &user.Name, &user.AvatarURL, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserByGitHubID retrieves a user by GitHub ID
func (s *Store) GetUserByGitHubID(ctx context.Context, githubID int64) (*User, error) {
	user := &User{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, github_id, github_login, email, name, avatar_url, is_active, created_at, updated_at
		FROM users
		WHERE github_id = $1
	`, githubID).Scan(&user.ID, &user.GitHubID, &user.GitHubLogin, &user.Email, &user.Name, &user.AvatarURL, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by github_id: %w", err)
	}

	return user, nil
}

// GetUserByGitHubLogin retrieves a user by GitHub login
func (s *Store) GetUserByGitHubLogin(ctx context.Context, login string) (*User, error) {
	user := &User{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, github_id, github_login, email, name, avatar_url, is_active, created_at, updated_at
		FROM users
		WHERE github_login = $1
	`, login).Scan(&user.ID, &user.GitHubID, &user.GitHubLogin, &user.Email, &user.Name, &user.AvatarURL, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by login: %w", err)
	}

	return user, nil
}

// UpsertUserFromGitHub creates or updates a user from GitHub OAuth
func (s *Store) UpsertUserFromGitHub(ctx context.Context, githubID int64, login, email, name, avatarURL string) (*User, error) {
	var emailPtr, namePtr, avatarPtr *string
	if email != "" {
		emailPtr = &email
	}
	if name != "" {
		namePtr = &name
	}
	if avatarURL != "" {
		avatarPtr = &avatarURL
	}

	// Try to find existing user
	existing, err := s.GetUserByGitHubID(ctx, githubID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		// Update existing user
		_, err := s.pool.Exec(ctx, `
			UPDATE users
			SET github_login = $2, email = $3, name = $4, avatar_url = $5, updated_at = $6
			WHERE id = $1
		`, existing.ID, login, emailPtr, namePtr, avatarPtr, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}

		// Refresh user data
		return s.GetUserByID(ctx, existing.ID)
	}

	// Create new user
	user := &User{
		GitHubID:    githubID,
		GitHubLogin: login,
		Email:       emailPtr,
		Name:        namePtr,
		AvatarURL:   avatarPtr,
	}

	if err := s.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// UpdateUser updates a user
func (s *Store) UpdateUser(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		UPDATE users
		SET github_login = $2, email = $3, name = $4, avatar_url = $5, is_active = $6, updated_at = $7
		WHERE id = $1
	`, user.ID, user.GitHubLogin, user.Email, user.Name, user.AvatarURL, user.IsActive, user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// DeactivateUser soft-deletes a user
func (s *Store) DeactivateUser(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET is_active = false, updated_at = $2 WHERE id = $1
	`, id, time.Now())

	if err != nil {
		return fmt.Errorf("failed to deactivate user: %w", err)
	}

	return nil
}
