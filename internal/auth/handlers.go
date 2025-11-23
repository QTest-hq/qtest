package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/db"
)

// Handlers provides HTTP handlers for authentication
type Handlers struct {
	github   *GitHubProvider
	sessions *SessionStore
	store    *db.Store
	ttl      time.Duration
}

// NewHandlers creates new auth handlers
func NewHandlers(github *GitHubProvider, sessions *SessionStore) *Handlers {
	return &Handlers{
		github:   github,
		sessions: sessions,
		ttl:      24 * time.Hour,
	}
}

// NewHandlersWithStore creates new auth handlers with database store
func NewHandlersWithStore(github *GitHubProvider, sessions *SessionStore, store *db.Store) *Handlers {
	return &Handlers{
		github:   github,
		sessions: sessions,
		store:    store,
		ttl:      24 * time.Hour,
	}
}

// LoginResponse is returned after successful login
type LoginResponse struct {
	SessionID string      `json:"session_id"`
	User      *GitHubUser `json:"user"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// HandleLogin initiates the OAuth login flow
func (h *Handlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	authURL, state, err := h.github.AuthURL()
	if err != nil {
		log.Error().Err(err).Msg("failed to generate auth URL")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Store state in cookie for validation
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	log.Debug().Str("state", state[:8]+"...").Msg("initiating OAuth login")
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleCallback handles the OAuth callback
func (h *Handlers) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// Get code and state from query params
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errorDesc := r.URL.Query().Get("error_description")
		if errorDesc == "" {
			errorDesc = r.URL.Query().Get("error")
		}
		log.Warn().Str("error", errorDesc).Msg("OAuth callback without code")
		http.Error(w, "authorization denied: "+errorDesc, http.StatusBadRequest)
		return
	}

	if state == "" {
		http.Error(w, "missing state parameter", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := h.github.Exchange(r.Context(), code, state)
	if err != nil {
		log.Error().Err(err).Msg("failed to exchange code")
		http.Error(w, "failed to complete authentication", http.StatusInternalServerError)
		return
	}

	// Get user info
	githubUser, err := h.github.GetUser(r.Context(), token.AccessToken)
	if err != nil {
		log.Error().Err(err).Msg("failed to get user info")
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}

	// Persist or update user in database
	var userID uuid.UUID
	if h.store != nil {
		dbUser, err := h.store.UpsertUserFromGitHub(
			r.Context(),
			githubUser.ID,
			githubUser.Login,
			githubUser.Email,
			githubUser.Name,
			githubUser.AvatarURL,
		)
		if err != nil {
			log.Error().Err(err).Msg("failed to persist user")
			http.Error(w, "failed to save user", http.StatusInternalServerError)
			return
		}
		userID = dbUser.ID
		log.Debug().
			Str("user", githubUser.Login).
			Str("user_id", userID.String()).
			Msg("user persisted to database")
	} else {
		// Fallback for backwards compatibility (no database)
		userID = uuid.New()
	}

	// Create session
	session, err := h.sessions.Create(userID, githubUser, token.AccessToken, "")
	if err != nil {
		log.Error().Err(err).Msg("failed to create session")
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	// Clear OAuth state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "qtest_session",
		Value:    session.ID,
		Path:     "/",
		MaxAge:   int(h.sessions.ttl.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	log.Info().
		Str("user", githubUser.Login).
		Str("session", session.ID[:8]+"...").
		Msg("user logged in")

	// Check for redirect URL in state or session
	redirectURL := r.URL.Query().Get("redirect")
	if redirectURL == "" {
		redirectURL = "/"
	}

	// For API clients, return JSON
	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{
			SessionID: session.ID,
			User:      githubUser,
			ExpiresAt: session.ExpiresAt,
		})
		return
	}

	// For browser clients, redirect
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// HandleLogout handles user logout
func (h *Handlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Get session from cookie or header
	sessionID := ""

	// Try cookie first
	if cookie, err := r.Cookie("qtest_session"); err == nil {
		sessionID = cookie.Value
	}

	// Try Authorization header
	if sessionID == "" {
		authHeader := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(authHeader) > len(prefix) && authHeader[:len(prefix)] == prefix {
			sessionID = authHeader[len(prefix):]
		}
	}

	if sessionID != "" {
		h.sessions.Delete(sessionID)
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "qtest_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	log.Debug().Msg("user logged out")

	// Return success
	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "logged_out"})
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// HandleMe returns the current user's information
func (h *Handlers) HandleMe(w http.ResponseWriter, r *http.Request) {
	session, ok := GetSessionFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":       session.GitHubUser,
		"session_id": session.ID[:8] + "...",
		"expires_at": session.ExpiresAt,
	})
}

// HandleRefresh extends the current session
func (h *Handlers) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	session, ok := GetSessionFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	if err := h.sessions.Extend(session.ID); err != nil {
		log.Error().Err(err).Msg("failed to extend session")
		http.Error(w, "failed to extend session", http.StatusInternalServerError)
		return
	}

	// Get updated session
	session, _ = h.sessions.Get(session.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id": session.ID[:8] + "...",
		"expires_at": session.ExpiresAt,
	})
}

// HandleListRepos returns the user's accessible repositories
func (h *Handlers) HandleListRepos(w http.ResponseWriter, r *http.Request) {
	session, ok := GetSessionFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	repos, err := h.github.ListUserRepos(r.Context(), session.AccessToken)
	if err != nil {
		log.Error().Err(err).Msg("failed to list repos")
		http.Error(w, "failed to list repositories", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repos)
}
