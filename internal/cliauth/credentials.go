// Package cliauth provides CLI authentication and credential management.
package cliauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrNoCredentials indicates no credentials are stored
	ErrNoCredentials = errors.New("no credentials found")
	// ErrInvalidAPIKey indicates the API key format is invalid
	ErrInvalidAPIKey = errors.New("invalid API key format")
	// ErrAuthFailed indicates authentication failed
	ErrAuthFailed = errors.New("authentication failed")
)

// Credentials stores authentication credentials for the CLI
type Credentials struct {
	APIKey         string    `json:"api_key,omitempty"`
	APIServer      string    `json:"api_server,omitempty"`
	OrganizationID string    `json:"organization_id,omitempty"`
	UserID         string    `json:"user_id,omitempty"`
	Username       string    `json:"username,omitempty"`
	Email          string    `json:"email,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at,omitempty"`
}

// IsExpired checks if the credentials have expired
func (c *Credentials) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false // API keys don't expire
	}
	return time.Now().After(c.ExpiresAt)
}

// CredentialsManager manages CLI credentials
type CredentialsManager struct {
	configDir string
}

// NewCredentialsManager creates a new credentials manager
func NewCredentialsManager() (*CredentialsManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".qtest")
	return &CredentialsManager{configDir: configDir}, nil
}

// NewCredentialsManagerWithDir creates a credentials manager with a custom directory
func NewCredentialsManagerWithDir(dir string) *CredentialsManager {
	return &CredentialsManager{configDir: dir}
}

// credentialsPath returns the path to the credentials file
func (m *CredentialsManager) credentialsPath() string {
	return filepath.Join(m.configDir, "credentials.json")
}

// Save saves credentials to disk
func (m *CredentialsManager) Save(creds *Credentials) error {
	// Ensure config directory exists with restricted permissions
	if err := os.MkdirAll(m.configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Write with restricted permissions (owner read/write only)
	if err := os.WriteFile(m.credentialsPath(), data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}

	return nil
}

// Load loads credentials from disk
func (m *CredentialsManager) Load() (*Credentials, error) {
	data, err := os.ReadFile(m.credentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoCredentials
		}
		return nil, fmt.Errorf("failed to read credentials: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return &creds, nil
}

// Delete removes stored credentials
func (m *CredentialsManager) Delete() error {
	err := os.Remove(m.credentialsPath())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}
	return nil
}

// Exists checks if credentials exist
func (m *CredentialsManager) Exists() bool {
	_, err := os.Stat(m.credentialsPath())
	return err == nil
}

// ValidateAPIKey validates the format of an API key
func ValidateAPIKey(apiKey string) error {
	if apiKey == "" {
		return ErrInvalidAPIKey
	}
	if !strings.HasPrefix(apiKey, "qtest_") {
		return fmt.Errorf("%w: must start with 'qtest_'", ErrInvalidAPIKey)
	}
	if len(apiKey) < 20 {
		return fmt.Errorf("%w: key too short", ErrInvalidAPIKey)
	}
	return nil
}

// APIClient provides methods to interact with the QTest API
type APIClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL, apiKey string) *APIClient {
	return &APIClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UserInfo contains information about the authenticated user
type UserInfo struct {
	ID             string   `json:"id"`
	Username       string   `json:"username"`
	Email          string   `json:"email"`
	OrganizationID string   `json:"organization_id"`
	Scopes         []string `json:"scopes"`
}

// ValidateCredentials validates the API key against the server
func (c *APIClient) ValidateCredentials() (*UserInfo, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/auth/me", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrAuthFailed
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	var info UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &info, nil
}

// CheckHealth checks if the API server is reachable
func (c *APIClient) CheckHealth() error {
	req, err := http.NewRequest("GET", c.baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server unhealthy (status %d)", resp.StatusCode)
	}

	return nil
}

// DefaultAPIServer returns the default API server URL
func DefaultAPIServer() string {
	if env := os.Getenv("QTEST_API_URL"); env != "" {
		return env
	}
	return "http://localhost:8080"
}

// GetAPIKey returns the API key from credentials or environment
func GetAPIKey() (string, error) {
	// Check environment variable first
	if apiKey := os.Getenv("QTEST_API_KEY"); apiKey != "" {
		return apiKey, nil
	}

	// Try loading from credentials file
	mgr, err := NewCredentialsManager()
	if err != nil {
		return "", err
	}

	creds, err := mgr.Load()
	if err != nil {
		return "", err
	}

	if creds.IsExpired() {
		return "", errors.New("credentials have expired, please login again")
	}

	return creds.APIKey, nil
}

// GetCredentials returns the full credentials from file or environment
func GetCredentials() (*Credentials, error) {
	// Check environment variables first
	apiKey := os.Getenv("QTEST_API_KEY")
	if apiKey != "" {
		return &Credentials{
			APIKey:    apiKey,
			APIServer: DefaultAPIServer(),
			CreatedAt: time.Now(),
		}, nil
	}

	// Try loading from credentials file
	mgr, err := NewCredentialsManager()
	if err != nil {
		return nil, err
	}

	creds, err := mgr.Load()
	if err != nil {
		return nil, err
	}

	if creds.IsExpired() {
		return nil, errors.New("credentials have expired, please login again")
	}

	return creds, nil
}
