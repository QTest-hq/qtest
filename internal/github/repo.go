package github

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/rs/zerolog/log"
)

// RepoService handles repository operations
type RepoService struct {
	baseDir string
	token   string
}

// NewRepoService creates a new repository service
func NewRepoService(baseDir, token string) *RepoService {
	return &RepoService{
		baseDir: baseDir,
		token:   token,
	}
}

// RepoInfo contains parsed repository information
type RepoInfo struct {
	Owner    string
	Name     string
	URL      string
	CloneURL string
	Branch   string
}

// CloneResult contains the result of a clone operation
type CloneResult struct {
	Path      string
	CommitSHA string
	Branch    string
}

// ParseRepoURL parses a GitHub URL and returns repo info
func ParseRepoURL(rawURL string) (*RepoInfo, error) {
	// Handle git@github.com:owner/repo.git format
	if strings.HasPrefix(rawURL, "git@") {
		parts := strings.Split(rawURL, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid SSH URL format: %s", rawURL)
		}
		pathParts := strings.Split(strings.TrimSuffix(parts[1], ".git"), "/")
		if len(pathParts) != 2 {
			return nil, fmt.Errorf("invalid repo path: %s", parts[1])
		}
		return &RepoInfo{
			Owner:    pathParts[0],
			Name:     pathParts[1],
			URL:      rawURL,
			CloneURL: fmt.Sprintf("https://github.com/%s/%s.git", pathParts[0], pathParts[1]),
			Branch:   "main",
		}, nil
	}

	// Parse HTTPS URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	if parsed.Host != "github.com" {
		return nil, fmt.Errorf("only github.com URLs are supported, got: %s", parsed.Host)
	}

	pathParts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(pathParts) < 2 {
		return nil, fmt.Errorf("invalid repo path: %s", parsed.Path)
	}

	owner := pathParts[0]
	name := strings.TrimSuffix(pathParts[1], ".git")

	return &RepoInfo{
		Owner:    owner,
		Name:     name,
		URL:      rawURL,
		CloneURL: fmt.Sprintf("https://github.com/%s/%s.git", owner, name),
		Branch:   "main",
	}, nil
}

// Clone clones a repository to local storage
func (s *RepoService) Clone(ctx context.Context, info *RepoInfo) (*CloneResult, error) {
	// Create directory for this repo
	repoDir := filepath.Join(s.baseDir, info.Owner, info.Name)

	// Remove existing directory if it exists
	if _, err := os.Stat(repoDir); err == nil {
		log.Debug().Str("path", repoDir).Msg("removing existing repo directory")
		if err := os.RemoveAll(repoDir); err != nil {
			return nil, fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(repoDir), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	log.Info().
		Str("url", info.CloneURL).
		Str("path", repoDir).
		Msg("cloning repository")

	// Setup clone options
	cloneOpts := &git.CloneOptions{
		URL:      info.CloneURL,
		Progress: nil, // Could add progress writer here
		Depth:    1,   // Shallow clone for faster download
	}

	// Add authentication if token is available
	if s.token != "" {
		cloneOpts.Auth = &http.BasicAuth{
			Username: "git", // Can be anything for token auth
			Password: s.token,
		}
	}

	// Try specific branch first, fall back to default
	if info.Branch != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(info.Branch)
		cloneOpts.SingleBranch = true
	}

	// Clone the repository
	repo, err := git.PlainCloneContext(ctx, repoDir, false, cloneOpts)
	if err != nil {
		// If branch doesn't exist, try without specifying branch
		if strings.Contains(err.Error(), "reference not found") && info.Branch != "" {
			log.Debug().Str("branch", info.Branch).Msg("branch not found, trying default")
			cloneOpts.ReferenceName = ""
			cloneOpts.SingleBranch = false
			repo, err = git.PlainCloneContext(ctx, repoDir, false, cloneOpts)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to clone: %w", err)
		}
	}

	// Get HEAD commit
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	result := &CloneResult{
		Path:      repoDir,
		CommitSHA: head.Hash().String(),
		Branch:    head.Name().Short(),
	}

	log.Info().
		Str("commit", result.CommitSHA[:8]).
		Str("branch", result.Branch).
		Msg("clone complete")

	return result, nil
}

// Pull updates an existing repository
func (s *RepoService) Pull(ctx context.Context, repoPath string) (*CloneResult, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repo: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	pullOpts := &git.PullOptions{
		Progress: nil,
	}

	if s.token != "" {
		pullOpts.Auth = &http.BasicAuth{
			Username: "git",
			Password: s.token,
		}
	}

	err = worktree.PullContext(ctx, pullOpts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, fmt.Errorf("failed to pull: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	return &CloneResult{
		Path:      repoPath,
		CommitSHA: head.Hash().String(),
		Branch:    head.Name().Short(),
	}, nil
}

// GetFiles returns all files in the repository matching patterns
func GetFiles(repoPath string, patterns []string) ([]string, error) {
	var files []string

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and common ignore patterns
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check file against patterns
		relPath, _ := filepath.Rel(repoPath, path)
		for _, pattern := range patterns {
			matched, _ := filepath.Match(pattern, filepath.Base(path))
			if matched {
				files = append(files, relPath)
				break
			}
		}

		return nil
	})

	return files, err
}

// DetectLanguages analyzes a repository and returns detected languages
func DetectLanguages(repoPath string) (map[string]int, error) {
	languages := make(map[string]int)

	extensionMap := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".jsx":  "javascript",
		".tsx":  "typescript",
		".java": "java",
		".rb":   "ruby",
		".rs":   "rust",
		".c":    "c",
		".cpp":  "cpp",
		".cs":   "csharp",
	}

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		// Skip test files and common directories
		if strings.Contains(path, "_test.") || strings.Contains(path, ".test.") ||
			strings.Contains(path, "/test/") || strings.Contains(path, "/tests/") {
			return nil
		}

		ext := filepath.Ext(path)
		if lang, ok := extensionMap[ext]; ok {
			languages[lang]++
		}

		return nil
	})

	return languages, err
}
