package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

// CloneConfig holds configuration for repository cloning
type CloneConfig struct {
	// Timeout for the entire clone operation
	Timeout time.Duration

	// MaxRepoSizeMB is the maximum repository size in megabytes (0 = no limit)
	MaxRepoSizeMB int64

	// MinDiskSpaceMB is the minimum required free disk space in megabytes
	MinDiskSpaceMB int64

	// ShallowClone enables depth=1 cloning for faster downloads
	ShallowClone bool

	// SingleBranch only clones the specified branch
	SingleBranch bool

	// ProgressWriter receives clone progress updates (nil = discard)
	ProgressWriter *os.File

	// RetryCount is the number of retries on transient failures
	RetryCount int

	// RetryDelay is the delay between retries
	RetryDelay time.Duration
}

// DefaultCloneConfig returns sensible defaults for repository cloning
func DefaultCloneConfig() *CloneConfig {
	return &CloneConfig{
		Timeout:        10 * time.Minute,
		MaxRepoSizeMB:  500,           // 500MB default limit
		MinDiskSpaceMB: 1024,          // 1GB minimum free space
		ShallowClone:   true,          // Faster cloning
		SingleBranch:   true,          // Only clone needed branch
		ProgressWriter: nil,           // No progress output by default
		RetryCount:     3,             // Retry transient failures
		RetryDelay:     5 * time.Second,
	}
}

// CloneError represents a clone operation error with additional context
type CloneError struct {
	Op      string // Operation that failed
	URL     string // Repository URL
	Err     error  // Underlying error
	Timeout bool   // Whether this was a timeout
	SizeErr bool   // Whether this was a size validation error
}

func (e *CloneError) Error() string {
	if e.Timeout {
		return fmt.Sprintf("clone timeout: %s after %s", e.URL, e.Op)
	}
	if e.SizeErr {
		return fmt.Sprintf("clone size error: %s - %s", e.URL, e.Err)
	}
	return fmt.Sprintf("clone failed: %s during %s: %v", e.URL, e.Op, e.Err)
}

func (e *CloneError) Unwrap() error {
	return e.Err
}

// GitHubRepoInfo contains repository metadata from GitHub API
type GitHubRepoInfo struct {
	Size          int64  `json:"size"`           // Size in KB
	DefaultBranch string `json:"default_branch"` // Default branch name
	Private       bool   `json:"private"`        // Is private repo
	Archived      bool   `json:"archived"`       // Is archived
	Disabled      bool   `json:"disabled"`       // Is disabled
	FullName      string `json:"full_name"`      // owner/repo
}

// ValidateRepoSize checks if a GitHub repository is within size limits
// Returns the repo info if valid, or an error if not
func ValidateRepoSize(ctx context.Context, owner, repo string, maxSizeMB int64, token string) (*GitHubRepoInfo, error) {
	if maxSizeMB <= 0 {
		return nil, nil // No limit
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "QTest/1.0")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Non-fatal: can't validate size, proceed with clone
		log.Warn().Err(err).Str("repo", fmt.Sprintf("%s/%s", owner, repo)).
			Msg("failed to fetch repo info, skipping size validation")
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("repository not found: %s/%s", owner, repo)
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		// Private repo without token, skip validation
		log.Debug().Str("repo", fmt.Sprintf("%s/%s", owner, repo)).
			Msg("cannot access repo info (may be private), skipping size validation")
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var info GitHubRepoInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// GitHub reports size in KB, convert to MB
	sizeInMB := info.Size / 1024

	log.Debug().
		Str("repo", info.FullName).
		Int64("size_kb", info.Size).
		Int64("size_mb", sizeInMB).
		Int64("max_mb", maxSizeMB).
		Msg("repository size check")

	if sizeInMB > maxSizeMB {
		return &info, &CloneError{
			Op:      "size_validation",
			URL:     fmt.Sprintf("https://github.com/%s/%s", owner, repo),
			Err:     fmt.Errorf("repository size %dMB exceeds limit of %dMB", sizeInMB, maxSizeMB),
			SizeErr: true,
		}
	}

	if info.Archived {
		log.Warn().Str("repo", info.FullName).Msg("repository is archived")
	}

	if info.Disabled {
		return &info, fmt.Errorf("repository is disabled: %s", info.FullName)
	}

	return &info, nil
}

// CheckDiskSpace verifies there's enough free disk space at the given path
func CheckDiskSpace(path string, requiredMB int64) error {
	if requiredMB <= 0 {
		return nil
	}

	// Get filesystem stats
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		// Non-fatal: can't check disk space, proceed
		log.Warn().Err(err).Str("path", path).Msg("failed to check disk space")
		return nil
	}

	// Calculate available space in MB
	availableMB := int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024)

	log.Debug().
		Int64("available_mb", availableMB).
		Int64("required_mb", requiredMB).
		Str("path", path).
		Msg("disk space check")

	if availableMB < requiredMB {
		return fmt.Errorf("insufficient disk space: %dMB available, %dMB required", availableMB, requiredMB)
	}

	return nil
}

// CleanupPartialClone removes a partially cloned repository
func CleanupPartialClone(path string) error {
	if path == "" {
		return nil
	}

	// Safety check: don't delete non-QTest directories
	if !strings.Contains(path, ".qtest") && !strings.Contains(path, "workspace") {
		log.Warn().Str("path", path).Msg("refusing to cleanup non-workspace directory")
		return nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Nothing to clean up
	}

	log.Info().Str("path", path).Msg("cleaning up partial clone")

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to cleanup partial clone: %w", err)
	}

	return nil
}

// ParseGitHubURL extracts owner and repo from a GitHub URL
func ParseGitHubURL(repoURL string) (owner, repo string, err error) {
	// Handle git@github.com:owner/repo.git format
	if strings.HasPrefix(repoURL, "git@github.com:") {
		parts := strings.TrimPrefix(repoURL, "git@github.com:")
		parts = strings.TrimSuffix(parts, ".git")
		segments := strings.Split(parts, "/")
		if len(segments) != 2 {
			return "", "", fmt.Errorf("invalid GitHub SSH URL: %s", repoURL)
		}
		return segments[0], segments[1], nil
	}

	// Handle https://github.com/owner/repo format
	if strings.Contains(repoURL, "github.com") {
		repoURL = strings.TrimSuffix(repoURL, ".git")
		parts := strings.Split(repoURL, "github.com/")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid GitHub URL: %s", repoURL)
		}
		segments := strings.Split(parts[1], "/")
		if len(segments) < 2 {
			return "", "", fmt.Errorf("invalid GitHub URL path: %s", repoURL)
		}
		return segments[0], segments[1], nil
	}

	return "", "", fmt.Errorf("not a GitHub URL: %s", repoURL)
}

// IsTransientError checks if an error is likely transient and worth retrying
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network-related transient errors
	transientPatterns := []string{
		"connection reset",
		"connection refused",
		"timeout",
		"timed out",
		"temporary failure",
		"no such host",
		"network is unreachable",
		"TLS handshake",
		"EOF",
		"broken pipe",
		"503",
		"502",
		"500",
	}

	for _, pattern := range transientPatterns {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
