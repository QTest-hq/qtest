package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	// ErrSessionNotFound indicates the session does not exist
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired indicates the session has expired
	ErrSessionExpired = errors.New("session expired")
	// ErrInvalidToken indicates the token is invalid
	ErrInvalidToken = errors.New("invalid token")
)

// Session represents a user session
type Session struct {
	ID           string       `json:"id"`
	UserID       uuid.UUID    `json:"user_id"`
	GitHubUser   *GitHubUser  `json:"github_user,omitempty"`
	AccessToken  string       `json:"-"` // Not serialized for security
	RefreshToken string       `json:"-"`
	CreatedAt    time.Time    `json:"created_at"`
	ExpiresAt    time.Time    `json:"expires_at"`
	LastAccess   time.Time    `json:"last_access"`
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// SessionStore manages user sessions
type SessionStore struct {
	mu        sync.RWMutex
	sessions  map[string]*Session
	byUserID  map[uuid.UUID]string // user_id -> session_id
	ttl       time.Duration
	maxPerUser int
}

// SessionStoreConfig configures the session store
type SessionStoreConfig struct {
	TTL        time.Duration // Session lifetime
	MaxPerUser int           // Max concurrent sessions per user
}

// NewSessionStore creates a new session store
func NewSessionStore(cfg SessionStoreConfig) *SessionStore {
	if cfg.TTL == 0 {
		cfg.TTL = 24 * time.Hour // Default 24 hours
	}
	if cfg.MaxPerUser == 0 {
		cfg.MaxPerUser = 5
	}

	s := &SessionStore{
		sessions:   make(map[string]*Session),
		byUserID:   make(map[uuid.UUID]string),
		ttl:        cfg.TTL,
		maxPerUser: cfg.MaxPerUser,
	}

	// Start cleanup goroutine
	go s.cleanup()

	return s
}

// Create creates a new session for a user
func (s *SessionStore) Create(userID uuid.UUID, githubUser *GitHubUser, accessToken, refreshToken string) (*Session, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	session := &Session{
		ID:           sessionID,
		UserID:       userID,
		GitHubUser:   githubUser,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		CreatedAt:    now,
		ExpiresAt:    now.Add(s.ttl),
		LastAccess:   now,
	}

	s.mu.Lock()
	// Remove existing session for this user if exists
	if existingID, exists := s.byUserID[userID]; exists {
		delete(s.sessions, existingID)
	}
	s.sessions[sessionID] = session
	s.byUserID[userID] = sessionID
	s.mu.Unlock()

	log.Debug().
		Str("session_id", sessionID[:8]+"...").
		Str("user", githubUser.Login).
		Msg("session created")

	return session, nil
}

// Get retrieves a session by ID
func (s *SessionStore) Get(sessionID string) (*Session, error) {
	s.mu.RLock()
	session, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrSessionNotFound
	}

	if session.IsExpired() {
		s.Delete(sessionID)
		return nil, ErrSessionExpired
	}

	// Update last access
	s.mu.Lock()
	session.LastAccess = time.Now()
	s.mu.Unlock()

	return session, nil
}

// Delete removes a session
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		delete(s.byUserID, session.UserID)
		delete(s.sessions, sessionID)
		log.Debug().Str("session_id", sessionID[:8]+"...").Msg("session deleted")
	}
}

// GetByUserID retrieves a session by user ID
func (s *SessionStore) GetByUserID(userID uuid.UUID) (*Session, error) {
	s.mu.RLock()
	sessionID, exists := s.byUserID[userID]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrSessionNotFound
	}

	return s.Get(sessionID)
}

// Extend extends a session's expiry
func (s *SessionStore) Extend(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	session.ExpiresAt = time.Now().Add(s.ttl)
	session.LastAccess = time.Now()
	return nil
}

// Count returns the number of active sessions
func (s *SessionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

func (s *SessionStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, session := range s.sessions {
			if now.After(session.ExpiresAt) {
				delete(s.byUserID, session.UserID)
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// contextKey is the type for context keys
type contextKey string

const (
	// SessionKey is the context key for the session
	SessionKey contextKey = "session"
	// UserKey is the context key for the user
	UserKey contextKey = "user"
)

// GetSessionFromContext retrieves the session from context
func GetSessionFromContext(ctx context.Context) (*Session, bool) {
	session, ok := ctx.Value(SessionKey).(*Session)
	return session, ok
}

// GetUserFromContext retrieves the user from context
func GetUserFromContext(ctx context.Context) (*GitHubUser, bool) {
	user, ok := ctx.Value(UserKey).(*GitHubUser)
	return user, ok
}

// Middleware provides authentication middleware
type Middleware struct {
	sessions *SessionStore
	github   *GitHubProvider
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(sessions *SessionStore, github *GitHubProvider) *Middleware {
	return &Middleware{
		sessions: sessions,
		github:   github,
	}
}

// RequireAuth is middleware that requires authentication
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := m.extractSession(r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// Add session and user to context
		ctx := context.WithValue(r.Context(), SessionKey, session)
		if session.GitHubUser != nil {
			ctx = context.WithValue(ctx, UserKey, session.GitHubUser)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth is middleware that adds auth info if present but doesn't require it
func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := m.extractSession(r)
		if err == nil && session != nil {
			ctx := context.WithValue(r.Context(), SessionKey, session)
			if session.GitHubUser != nil {
				ctx = context.WithValue(ctx, UserKey, session.GitHubUser)
			}
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) extractSession(r *http.Request) (*Session, error) {
	// Try Authorization header first (Bearer token)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		const prefix = "Bearer "
		if len(authHeader) > len(prefix) && authHeader[:len(prefix)] == prefix {
			sessionID := authHeader[len(prefix):]
			return m.sessions.Get(sessionID)
		}
	}

	// Try cookie
	cookie, err := r.Cookie("qtest_session")
	if err == nil && cookie.Value != "" {
		return m.sessions.Get(cookie.Value)
	}

	return nil, ErrInvalidToken
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
