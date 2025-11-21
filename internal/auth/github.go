// Package auth provides authentication and authorization
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// GitHubAuthURL is the OAuth authorization endpoint
	GitHubAuthURL = "https://github.com/login/oauth/authorize"
	// GitHubTokenURL is the OAuth token endpoint
	GitHubTokenURL = "https://github.com/login/oauth/access_token"
	// GitHubUserURL is the API endpoint for user info
	GitHubUserURL = "https://api.github.com/user"
)

// GitHubProvider handles GitHub OAuth authentication
type GitHubProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	scopes       []string
	httpClient   *http.Client
	states       *stateStore // CSRF protection
}

// GitHubConfig configures the GitHub OAuth provider
type GitHubConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// GitHubUser represents a GitHub user
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubTokenResponse represents the OAuth token response
type GitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

// stateStore manages OAuth state for CSRF protection
type stateStore struct {
	mu     sync.RWMutex
	states map[string]time.Time
	ttl    time.Duration
}

func newStateStore(ttl time.Duration) *stateStore {
	s := &stateStore{
		states: make(map[string]time.Time),
		ttl:    ttl,
	}
	// Start cleanup goroutine
	go s.cleanup()
	return s
}

func (s *stateStore) Generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.URLEncoding.EncodeToString(b)

	s.mu.Lock()
	s.states[state] = time.Now().Add(s.ttl)
	s.mu.Unlock()

	return state, nil
}

func (s *stateStore) Validate(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	expiry, exists := s.states[state]
	if !exists {
		return false
	}

	// Remove after validation (one-time use)
	delete(s.states, state)

	return time.Now().Before(expiry)
}

func (s *stateStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for state, expiry := range s.states {
			if now.After(expiry) {
				delete(s.states, state)
			}
		}
		s.mu.Unlock()
	}
}

// NewGitHubProvider creates a new GitHub OAuth provider
func NewGitHubProvider(cfg GitHubConfig) *GitHubProvider {
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		// Default scopes for QTest
		scopes = []string{"read:user", "user:email", "repo"}
	}

	return &GitHubProvider{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURL:  cfg.RedirectURL,
		scopes:       scopes,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		states:       newStateStore(10 * time.Minute),
	}
}

// AuthURL returns the URL to redirect users to for authentication
func (p *GitHubProvider) AuthURL() (string, string, error) {
	state, err := p.states.Generate()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate state: %w", err)
	}

	url := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&scope=%s&state=%s",
		GitHubAuthURL,
		p.clientID,
		p.redirectURL,
		strings.Join(p.scopes, "%20"),
		state,
	)

	return url, state, nil
}

// Exchange exchanges an authorization code for an access token
func (p *GitHubProvider) Exchange(ctx context.Context, code, state string) (*GitHubTokenResponse, error) {
	// Validate state for CSRF protection
	if !p.states.Validate(state) {
		return nil, fmt.Errorf("invalid or expired state parameter")
	}

	// Build token request
	reqBody := fmt.Sprintf("client_id=%s&client_secret=%s&code=%s&redirect_uri=%s",
		p.clientID, p.clientSecret, code, p.redirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", GitHubTokenURL, strings.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var tokenResp GitHubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("OAuth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	return &tokenResp, nil
}

// GetUser fetches the authenticated user's information
func (p *GitHubProvider) GetUser(ctx context.Context, accessToken string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", GitHubUserURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user with status: %d", resp.StatusCode)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &user, nil
}

// ValidateToken checks if a token is still valid
func (p *GitHubProvider) ValidateToken(ctx context.Context, accessToken string) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", GitHubUserURL, nil)
	if err != nil {
		log.Debug().Err(err).Msg("failed to create validation request")
		return false
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("token validation request failed")
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// ListUserRepos lists repositories the user has access to
func (p *GitHubProvider) ListUserRepos(ctx context.Context, accessToken string) ([]GitHubRepo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/repos?per_page=100", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repos: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list repos with status: %d", resp.StatusCode)
	}

	var repos []GitHubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("failed to decode repos: %w", err)
	}

	return repos, nil
}

// GitHubRepo represents a GitHub repository
type GitHubRepo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Description   string `json:"description"`
}
