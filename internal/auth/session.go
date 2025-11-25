package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
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
	// Note: ErrInvalidToken is defined in jwt.go
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
	// APIKeyKey is the context key for API key info
	APIKeyKey contextKey = "api_key"
	// JWTClaimsKey is the context key for JWT claims
	JWTClaimsKey contextKey = "jwt_claims"
)

// APIKeyInfo contains information about an authenticated API key
type APIKeyInfo struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Scopes         []string
}

// HasScope checks if the API key has a specific scope (or admin scope)
func (info *APIKeyInfo) HasScope(scope string) bool {
	for _, s := range info.Scopes {
		if s == scope || s == "admin" {
			return true
		}
	}
	return false
}

// HasAnyScope checks if the API key has any of the specified scopes
func (info *APIKeyInfo) HasAnyScope(scopes ...string) bool {
	for _, scope := range scopes {
		if info.HasScope(scope) {
			return true
		}
	}
	return false
}

// APIKeyValidator is an interface for validating API keys
// Returns nil, nil if key not found, nil, error on failure
type APIKeyValidator interface {
	ValidateAPIKeyForAuth(ctx context.Context, apiKey string) (*APIKeyInfo, error)
}

// GetAPIKeyFromContext retrieves the API key info from context
func GetAPIKeyFromContext(ctx context.Context) (*APIKeyInfo, bool) {
	info, ok := ctx.Value(APIKeyKey).(*APIKeyInfo)
	return info, ok
}

// GetJWTClaimsFromContext retrieves the JWT claims from context
func GetJWTClaimsFromContext(ctx context.Context) (*QTestClaims, bool) {
	claims, ok := ctx.Value(JWTClaimsKey).(*QTestClaims)
	return claims, ok
}

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
	sessions        *SessionStore
	github          *GitHubProvider
	apiKeyValidator APIKeyValidator
	jwtService      *JWTService
	refreshService  *RefreshService
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(sessions *SessionStore, github *GitHubProvider) *Middleware {
	return &Middleware{
		sessions: sessions,
		github:   github,
	}
}

// SetAPIKeyValidator sets the API key validator for the middleware
func (m *Middleware) SetAPIKeyValidator(v APIKeyValidator) {
	m.apiKeyValidator = v
}

// SetJWTService sets the JWT service for the middleware
func (m *Middleware) SetJWTService(jwtSvc *JWTService) {
	m.jwtService = jwtSvc
}

// SetRefreshService sets the refresh token service for the middleware
func (m *Middleware) SetRefreshService(refreshSvc *RefreshService) {
	m.refreshService = refreshSvc
}

// RequireAuth is middleware that requires authentication
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Try API key authentication first
		if apiKeyInfo, ok := m.tryAPIKeyAuth(r); ok {
			ctx = context.WithValue(ctx, APIKeyKey, apiKeyInfo)
			// Create a synthetic session for API key auth to work with existing handlers
			ctx = context.WithValue(ctx, SessionKey, &Session{
				UserID:    apiKeyInfo.UserID,
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(time.Hour),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Try JWT authentication second (Bearer token with dots = JWT)
		if claims, ok := m.tryJWTAuth(r); ok {
			ctx = context.WithValue(ctx, JWTClaimsKey, claims)
			// Create a synthetic session for JWT auth to work with existing handlers
			userID, _ := uuid.Parse(claims.UserID)
			var orgID *uuid.UUID
			if claims.OrganizationID != "" {
				parsed, _ := uuid.Parse(claims.OrganizationID)
				orgID = &parsed
			}
			ctx = context.WithValue(ctx, SessionKey, &Session{
				ID:        claims.SessionID,
				UserID:    userID,
				CreatedAt: claims.IssuedAt.Time,
				ExpiresAt: claims.ExpiresAt.Time,
				GitHubUser: &GitHubUser{
					Login: claims.GitHubLogin,
				},
			})
			// Also set API key info for scope checking
			ctx = context.WithValue(ctx, APIKeyKey, &APIKeyInfo{
				UserID:         userID,
				OrganizationID: func() uuid.UUID { if orgID != nil { return *orgID }; return uuid.Nil }(),
				Scopes:         claims.Scopes,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Try session-based authentication as fallback
		session, err := m.extractSession(r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// Add session and user to context
		ctx = context.WithValue(ctx, SessionKey, session)
		if session.GitHubUser != nil {
			ctx = context.WithValue(ctx, UserKey, session.GitHubUser)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth is middleware that adds auth info if present but doesn't require it
func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Try API key authentication first
		if apiKeyInfo, ok := m.tryAPIKeyAuth(r); ok {
			ctx = context.WithValue(ctx, APIKeyKey, apiKeyInfo)
			// Create a synthetic session for API key auth
			ctx = context.WithValue(ctx, SessionKey, &Session{
				UserID:    apiKeyInfo.UserID,
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(time.Hour),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Try JWT authentication second (Bearer token with dots = JWT)
		if claims, ok := m.tryJWTAuth(r); ok {
			ctx = context.WithValue(ctx, JWTClaimsKey, claims)
			userID, _ := uuid.Parse(claims.UserID)
			var orgID *uuid.UUID
			if claims.OrganizationID != "" {
				parsed, _ := uuid.Parse(claims.OrganizationID)
				orgID = &parsed
			}
			ctx = context.WithValue(ctx, SessionKey, &Session{
				ID:        claims.SessionID,
				UserID:    userID,
				CreatedAt: claims.IssuedAt.Time,
				ExpiresAt: claims.ExpiresAt.Time,
				GitHubUser: &GitHubUser{
					Login: claims.GitHubLogin,
				},
			})
			ctx = context.WithValue(ctx, APIKeyKey, &APIKeyInfo{
				UserID:         userID,
				OrganizationID: func() uuid.UUID { if orgID != nil { return *orgID }; return uuid.Nil }(),
				Scopes:         claims.Scopes,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Try session-based authentication as fallback
		session, err := m.extractSession(r)
		if err == nil && session != nil {
			ctx = context.WithValue(ctx, SessionKey, session)
			if session.GitHubUser != nil {
				ctx = context.WithValue(ctx, UserKey, session.GitHubUser)
			}
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) extractSession(r *http.Request) (*Session, error) {
	// Try Authorization header first (Bearer token, but not API keys)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		const prefix = "Bearer "
		if len(authHeader) > len(prefix) && authHeader[:len(prefix)] == prefix {
			token := authHeader[len(prefix):]
			// Skip if it looks like an API key
			if !strings.HasPrefix(token, "qtest_") {
				return m.sessions.Get(token)
			}
		}
	}

	// Try cookie
	cookie, err := r.Cookie("qtest_session")
	if err == nil && cookie.Value != "" {
		return m.sessions.Get(cookie.Value)
	}

	return nil, ErrInvalidToken
}

// tryAPIKeyAuth attempts to authenticate using an API key
func (m *Middleware) tryAPIKeyAuth(r *http.Request) (*APIKeyInfo, bool) {
	if m.apiKeyValidator == nil {
		return nil, false
	}

	// Try X-API-Key header first
	apiKey := r.Header.Get("X-API-Key")

	// Try Authorization: Bearer header
	if apiKey == "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			const prefix = "Bearer "
			if len(authHeader) > len(prefix) && authHeader[:len(prefix)] == prefix {
				token := authHeader[len(prefix):]
				if strings.HasPrefix(token, "qtest_") {
					apiKey = token
				}
			}
		}
	}

	if apiKey == "" {
		return nil, false
	}

	// Validate the API key
	info, err := m.apiKeyValidator.ValidateAPIKeyForAuth(r.Context(), apiKey)
	if err != nil || info == nil {
		log.Debug().Err(err).Msg("API key validation failed")
		return nil, false
	}

	log.Debug().
		Str("key_id", info.ID.String()[:8]+"...").
		Str("org_id", info.OrganizationID.String()[:8]+"...").
		Msg("API key authenticated")

	return info, true
}

// tryJWTAuth attempts to authenticate using a JWT access token
// JWTs are identified by having dots (.) in them - format: header.payload.signature
func (m *Middleware) tryJWTAuth(r *http.Request) (*QTestClaims, bool) {
	if m.jwtService == nil {
		return nil, false
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, false
	}

	const prefix = "Bearer "
	if len(authHeader) <= len(prefix) || authHeader[:len(prefix)] != prefix {
		return nil, false
	}

	token := authHeader[len(prefix):]

	// Skip API keys and session tokens - JWTs have dots
	if strings.HasPrefix(token, "qtest_") {
		return nil, false
	}
	if !strings.Contains(token, ".") {
		return nil, false
	}

	// Validate the JWT
	claims, err := m.jwtService.ValidateAccessToken(token)
	if err != nil {
		log.Debug().Err(err).Msg("JWT validation failed")
		return nil, false
	}

	// Check if the token has been revoked (blacklisted)
	// The JTI is extracted from the validated claims, not from ParseUnverified
	if m.refreshService != nil && claims.ID != "" {
		blacklisted, err := m.refreshService.IsBlacklisted(r.Context(), claims.ID)
		if err != nil {
			log.Warn().Err(err).Msg("failed to check token blacklist")
			// Fail open for availability, but log the error
			// In high-security environments, consider failing closed instead
		} else if blacklisted {
			log.Info().
				Str("jti", claims.ID[:8]+"...").
				Str("user_id", claims.UserID[:8]+"...").
				Msg("rejected blacklisted token")
			return nil, false
		}
	}

	log.Debug().
		Str("user_id", claims.UserID[:8]+"...").
		Str("session_id", claims.SessionID[:8]+"...").
		Msg("JWT authenticated")

	return claims, true
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
