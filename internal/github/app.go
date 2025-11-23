package github

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// AppConfig contains GitHub App configuration
type AppConfig struct {
	AppID         int64  `json:"app_id"`
	PrivateKeyPEM string `json:"private_key"` // PEM-encoded RSA private key
	WebhookSecret string `json:"webhook_secret,omitempty"`
}

// App represents a GitHub App client
type App struct {
	appID      int64
	privateKey *rsa.PrivateKey
	client     *http.Client
	baseURL    string

	// Token cache
	tokenMu    sync.RWMutex
	tokenCache map[int64]*InstallationToken
}

// Installation represents a GitHub App installation
type Installation struct {
	ID                  int64     `json:"id"`
	Account             Account   `json:"account"`
	RepositorySelection string    `json:"repository_selection"` // "all" or "selected"
	AccessTokensURL     string    `json:"access_tokens_url"`
	RepositoriesURL     string    `json:"repositories_url"`
	HTMLURL             string    `json:"html_url"`
	AppID               int64     `json:"app_id"`
	TargetID            int64     `json:"target_id"`
	TargetType          string    `json:"target_type"` // "User" or "Organization"
	Permissions         Perms     `json:"permissions"`
	Events              []string  `json:"events"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Account represents a GitHub account (user or organization)
type Account struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Type      string `json:"type"` // "User" or "Organization"
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

// Perms represents GitHub App permissions
type Perms map[string]string

// InstallationToken represents an installation access token
type InstallationToken struct {
	Token        string            `json:"token"`
	ExpiresAt    time.Time         `json:"expires_at"`
	Permissions  Perms             `json:"permissions"`
	Repositories []Repository      `json:"repositories,omitempty"`
	Selection    string            `json:"repository_selection,omitempty"`
}

// Repository represents a GitHub repository
type Repository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	HTMLURL  string `json:"html_url"`
	CloneURL string `json:"clone_url"`
	SSHURL   string `json:"ssh_url"`
}

// NewApp creates a new GitHub App client
func NewApp(cfg *AppConfig) (*App, error) {
	if cfg.AppID == 0 {
		return nil, fmt.Errorf("app_id is required")
	}
	if cfg.PrivateKeyPEM == "" {
		return nil, fmt.Errorf("private_key is required")
	}

	// Parse the private key
	block, _ := pem.Decode([]byte(cfg.PrivateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block from private key")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		key, ok = parsedKey.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}

	return &App{
		appID:      cfg.AppID,
		privateKey: key,
		client:     &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://api.github.com",
		tokenCache: make(map[int64]*InstallationToken),
	}, nil
}

// generateJWT creates a JWT for authenticating as the GitHub App
func (a *App) generateJWT() (string, error) {
	now := time.Now()

	// Create JWT header
	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}

	// Create JWT payload
	// IAT can be at most 60 seconds in the past
	// EXP can be at most 10 minutes in the future
	payload := map[string]interface{}{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": a.appID,
	}

	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	headerB64 := base64URLEncode(headerJSON)
	payloadB64 := base64URLEncode(payloadJSON)

	signingInput := headerB64 + "." + payloadB64

	// Sign with RSA-SHA256
	signature, err := signRS256([]byte(signingInput), a.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	signatureB64 := base64URLEncode(signature)

	return signingInput + "." + signatureB64, nil
}

// ListInstallations returns all installations of this GitHub App
func (a *App) ListInstallations(ctx context.Context) ([]Installation, error) {
	jwt, err := a.generateJWT()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/app/installations", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list installations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list installations: %s - %s", resp.Status, string(body))
	}

	var installations []Installation
	if err := json.NewDecoder(resp.Body).Decode(&installations); err != nil {
		return nil, fmt.Errorf("failed to decode installations: %w", err)
	}

	return installations, nil
}

// GetInstallation returns a specific installation by ID
func (a *App) GetInstallation(ctx context.Context, installationID int64) (*Installation, error) {
	jwt, err := a.generateJWT()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/app/installations/%d", a.baseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get installation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get installation: %s - %s", resp.Status, string(body))
	}

	var installation Installation
	if err := json.NewDecoder(resp.Body).Decode(&installation); err != nil {
		return nil, fmt.Errorf("failed to decode installation: %w", err)
	}

	return &installation, nil
}

// GetInstallationToken gets an access token for an installation
// Tokens are cached and reused until they expire
func (a *App) GetInstallationToken(ctx context.Context, installationID int64) (*InstallationToken, error) {
	// Check cache first
	a.tokenMu.RLock()
	if token, ok := a.tokenCache[installationID]; ok {
		// Check if token is still valid (with 5 minute buffer)
		if token.ExpiresAt.After(time.Now().Add(5 * time.Minute)) {
			a.tokenMu.RUnlock()
			return token, nil
		}
	}
	a.tokenMu.RUnlock()

	// Generate new token
	jwt, err := a.generateJWT()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", a.baseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get installation token: %s - %s", resp.Status, string(body))
	}

	var token InstallationToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	// Cache the token
	a.tokenMu.Lock()
	a.tokenCache[installationID] = &token
	a.tokenMu.Unlock()

	return &token, nil
}

// GetInstallationForRepo finds the installation for a specific repository
func (a *App) GetInstallationForRepo(ctx context.Context, owner, repo string) (*Installation, error) {
	jwt, err := a.generateJWT()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/installation", a.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo installation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("app is not installed on repository %s/%s", owner, repo)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get repo installation: %s - %s", resp.Status, string(body))
	}

	var installation Installation
	if err := json.NewDecoder(resp.Body).Decode(&installation); err != nil {
		return nil, fmt.Errorf("failed to decode installation: %w", err)
	}

	return &installation, nil
}

// ListInstallationRepos lists repositories accessible to an installation
func (a *App) ListInstallationRepos(ctx context.Context, installationID int64) ([]Repository, error) {
	token, err := a.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/installation/repositories", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list repos: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list repos: %s - %s", resp.Status, string(body))
	}

	var result struct {
		TotalCount   int          `json:"total_count"`
		Repositories []Repository `json:"repositories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode repos: %w", err)
	}

	return result.Repositories, nil
}

// CreateRepoServiceForInstallation creates a RepoService using an installation token
func (a *App) CreateRepoServiceForInstallation(ctx context.Context, installationID int64, baseDir string) (*RepoService, error) {
	token, err := a.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	return NewRepoService(baseDir, token.Token), nil
}

// CreatePRServiceForInstallation creates a PRService using an installation token
func (a *App) CreatePRServiceForInstallation(ctx context.Context, installationID int64) (*PRService, error) {
	token, err := a.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	return NewPRService(token.Token), nil
}

// InvalidateTokenCache clears the token cache for an installation
func (a *App) InvalidateTokenCache(installationID int64) {
	a.tokenMu.Lock()
	delete(a.tokenCache, installationID)
	a.tokenMu.Unlock()
}

// GetAppInfo returns information about the GitHub App
func (a *App) GetAppInfo(ctx context.Context) (*AppInfo, error) {
	jwt, err := a.generateJWT()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/app", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get app info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get app info: %s - %s", resp.Status, string(body))
	}

	var info AppInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode app info: %w", err)
	}

	return &info, nil
}

// AppInfo contains information about a GitHub App
type AppInfo struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ExternalURL string    `json:"external_url"`
	HTMLURL     string    `json:"html_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Permissions Perms     `json:"permissions"`
	Events      []string  `json:"events"`
}
