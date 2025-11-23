// Package auth provides GitHub App authentication
package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

const (
	// GitHubAppAPIURL is the base URL for GitHub App API
	GitHubAppAPIURL = "https://api.github.com/app"
	// GitHubInstallationsURL is the URL for listing installations
	GitHubInstallationsURL = "https://api.github.com/app/installations"
)

// GitHubAppClient handles GitHub App authentication
type GitHubAppClient struct {
	appID      int64
	privateKey *rsa.PrivateKey
	httpClient *http.Client

	// Cache for installation tokens
	tokenCache   map[int64]*InstallationToken
	tokenCacheMu sync.RWMutex
}

// GitHubAppClientConfig configures the GitHub App client
type GitHubAppClientConfig struct {
	AppID          int64
	PrivateKeyPath string
	PrivateKeyPEM  string
}

// Installation represents a GitHub App installation
type Installation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Type  string `json:"type"` // "User" or "Organization"
	} `json:"account"`
	RepositorySelection string   `json:"repository_selection"` // "all" or "selected"
	Permissions         struct {
		Contents     string `json:"contents,omitempty"`
		PullRequests string `json:"pull_requests,omitempty"`
		Issues       string `json:"issues,omitempty"`
		Metadata     string `json:"metadata,omitempty"`
	} `json:"permissions"`
	Events          []string  `json:"events"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	SingleFileName  string    `json:"single_file_name,omitempty"`
	HasMultipleFiles bool     `json:"has_multiple_single_files"`
}

// InstallationToken represents an installation access token
type InstallationToken struct {
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expires_at"`
	Permissions struct {
		Contents     string `json:"contents,omitempty"`
		PullRequests string `json:"pull_requests,omitempty"`
		Issues       string `json:"issues,omitempty"`
		Metadata     string `json:"metadata,omitempty"`
	} `json:"permissions"`
	RepositorySelection string `json:"repository_selection"`
}

// NewGitHubAppClient creates a new GitHub App client
func NewGitHubAppClient(cfg GitHubAppClientConfig) (*GitHubAppClient, error) {
	var privateKey *rsa.PrivateKey
	var err error

	// Load private key from PEM or file
	if cfg.PrivateKeyPEM != "" {
		privateKey, err = parsePrivateKey([]byte(cfg.PrivateKeyPEM))
	} else if cfg.PrivateKeyPath != "" {
		pemData, readErr := os.ReadFile(cfg.PrivateKeyPath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", readErr)
		}
		privateKey, err = parsePrivateKey(pemData)
	} else {
		return nil, fmt.Errorf("either PrivateKeyPEM or PrivateKeyPath must be provided")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &GitHubAppClient{
		appID:      cfg.AppID,
		privateKey: privateKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		tokenCache: make(map[int64]*InstallationToken),
	}, nil
}

// parsePrivateKey parses a PEM-encoded RSA private key
func parsePrivateKey(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try PKCS#1 first, then PKCS#8
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS#8
		pkcs8Key, pkcs8Err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if pkcs8Err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w (PKCS#8: %v)", err, pkcs8Err)
		}
		var ok bool
		key, ok = pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key is not RSA")
		}
	}

	return key, nil
}

// GenerateJWT generates a JWT for GitHub App authentication
func (c *GitHubAppClient) GenerateJWT() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(10 * time.Minute).Unix(), // GitHub JWTs expire in 10 min
		"iss": c.appID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return signedToken, nil
}

// GetApp returns information about the GitHub App
func (c *GitHubAppClient) GetApp(ctx context.Context) (*AppInfo, error) {
	jwt, err := c.GenerateJWT()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", GitHubAppAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch app info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get app with status: %d", resp.StatusCode)
	}

	var app AppInfo
	if err := json.NewDecoder(resp.Body).Decode(&app); err != nil {
		return nil, fmt.Errorf("failed to decode app info: %w", err)
	}

	return &app, nil
}

// AppInfo represents GitHub App information
type AppInfo struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	Owner       struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"owner"`
	Permissions struct {
		Contents     string `json:"contents,omitempty"`
		PullRequests string `json:"pull_requests,omitempty"`
		Issues       string `json:"issues,omitempty"`
		Metadata     string `json:"metadata,omitempty"`
	} `json:"permissions"`
	Events []string `json:"events"`
}

// ListInstallations returns all installations of the GitHub App
func (c *GitHubAppClient) ListInstallations(ctx context.Context) ([]Installation, error) {
	jwt, err := c.GenerateJWT()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", GitHubInstallationsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch installations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list installations with status: %d", resp.StatusCode)
	}

	var installations []Installation
	if err := json.NewDecoder(resp.Body).Decode(&installations); err != nil {
		return nil, fmt.Errorf("failed to decode installations: %w", err)
	}

	return installations, nil
}

// GetInstallationToken returns an access token for a specific installation
// The token is cached until 5 minutes before expiry
func (c *GitHubAppClient) GetInstallationToken(ctx context.Context, installationID int64) (*InstallationToken, error) {
	// Check cache first
	c.tokenCacheMu.RLock()
	if token, ok := c.tokenCache[installationID]; ok {
		// Token valid for at least 5 more minutes
		if time.Now().Add(5 * time.Minute).Before(token.ExpiresAt) {
			c.tokenCacheMu.RUnlock()
			return token, nil
		}
	}
	c.tokenCacheMu.RUnlock()

	// Generate new token
	jwt, err := c.GenerateJWT()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create installation token with status: %d", resp.StatusCode)
	}

	var token InstallationToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	// Cache the token
	c.tokenCacheMu.Lock()
	c.tokenCache[installationID] = &token
	c.tokenCacheMu.Unlock()

	log.Debug().
		Int64("installation_id", installationID).
		Time("expires_at", token.ExpiresAt).
		Msg("created new installation token")

	return &token, nil
}

// GetInstallationForAccount finds the installation for a specific account (user or org)
func (c *GitHubAppClient) GetInstallationForAccount(ctx context.Context, accountLogin string) (*Installation, error) {
	installations, err := c.ListInstallations(ctx)
	if err != nil {
		return nil, err
	}

	for _, inst := range installations {
		if inst.Account.Login == accountLogin {
			return &inst, nil
		}
	}

	return nil, fmt.Errorf("no installation found for account: %s", accountLogin)
}

// GetRepositoryInstallation gets the installation for a specific repository
func (c *GitHubAppClient) GetRepositoryInstallation(ctx context.Context, owner, repo string) (*Installation, error) {
	jwt, err := c.GenerateJWT()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/installation", owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository installation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("app not installed on repository %s/%s", owner, repo)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get repository installation with status: %d", resp.StatusCode)
	}

	var installation Installation
	if err := json.NewDecoder(resp.Body).Decode(&installation); err != nil {
		return nil, fmt.Errorf("failed to decode installation: %w", err)
	}

	return &installation, nil
}

// GetTokenForRepository gets an access token for a specific repository
// This is a convenience method that combines GetRepositoryInstallation and GetInstallationToken
func (c *GitHubAppClient) GetTokenForRepository(ctx context.Context, owner, repo string) (string, error) {
	installation, err := c.GetRepositoryInstallation(ctx, owner, repo)
	if err != nil {
		return "", err
	}

	token, err := c.GetInstallationToken(ctx, installation.ID)
	if err != nil {
		return "", err
	}

	return token.Token, nil
}
