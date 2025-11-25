package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

var (
	// githubRepoPattern matches valid GitHub repository URLs
	githubRepoPattern = regexp.MustCompile(`^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+(?:\.git)?$`)

	// alphanumericPattern matches alphanumeric strings with underscores and hyphens
	alphanumericPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// SanitizeUUID parses and validates a UUID string.
// Returns the parsed UUID or an error if invalid.
func SanitizeUUID(s string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, fmt.Errorf("UUID cannot be empty")
	}

	// Trim whitespace
	s = strings.TrimSpace(s)

	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid UUID format")
	}

	return id, nil
}

// SanitizeString removes control characters and trims to maxLen.
// Returns the sanitized string.
func SanitizeString(s string, maxLen int) string {
	// Remove control characters (except newlines and tabs)
	var result strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) {
			result.WriteRune(r)
		}
	}

	sanitized := result.String()

	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)

	// Truncate if too long
	if maxLen > 0 && len(sanitized) > maxLen {
		sanitized = sanitized[:maxLen]
	}

	return sanitized
}

// SanitizeRepoURL validates and sanitizes a repository URL.
// Only allows GitHub HTTPS URLs.
func SanitizeRepoURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("repository URL cannot be empty")
	}

	// Trim whitespace
	rawURL = strings.TrimSpace(rawURL)

	// Parse the URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format")
	}

	// Must be HTTPS
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("repository URL must use HTTPS")
	}

	// Must be GitHub
	if parsed.Host != "github.com" {
		return "", fmt.Errorf("only GitHub repositories are supported")
	}

	// Validate the path structure
	if !githubRepoPattern.MatchString(rawURL) {
		return "", fmt.Errorf("invalid GitHub repository URL format")
	}

	// Remove trailing .git if present for consistency
	cleanURL := strings.TrimSuffix(rawURL, ".git")

	return cleanURL, nil
}

// SanitizeName validates a name (username, org name, etc.)
// Names must be alphanumeric with underscores/hyphens, 1-100 chars.
func SanitizeName(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("name cannot be empty")
	}

	s = strings.TrimSpace(s)

	if len(s) > 100 {
		return "", fmt.Errorf("name too long (max 100 characters)")
	}

	if !alphanumericPattern.MatchString(s) {
		return "", fmt.Errorf("name must contain only alphanumeric characters, underscores, and hyphens")
	}

	return s, nil
}

// SanitizeEmail validates and normalizes an email address.
func SanitizeEmail(email string) (string, error) {
	if email == "" {
		return "", fmt.Errorf("email cannot be empty")
	}

	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	// Basic email validation
	if !strings.Contains(email, "@") {
		return "", fmt.Errorf("invalid email format")
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid email format")
	}

	if !strings.Contains(parts[1], ".") {
		return "", fmt.Errorf("invalid email domain")
	}

	if len(email) > 254 {
		return "", fmt.Errorf("email too long")
	}

	return email, nil
}
