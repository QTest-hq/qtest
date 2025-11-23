package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/rs/zerolog/log"
)

// GitManager handles git operations for a workspace
type GitManager struct {
	ws    *Workspace
	repo  *git.Repository
	token string
}

// NewGitManager creates a git manager for a workspace
func NewGitManager(ws *Workspace, token string) *GitManager {
	return &GitManager{
		ws:    ws,
		token: token,
	}
}

// Clone clones the repository into the workspace (uses default config)
func (g *GitManager) Clone(ctx context.Context) error {
	return g.CloneWithConfig(ctx, DefaultCloneConfig())
}

// CloneWithConfig clones the repository with custom configuration
// Includes timeout handling, size validation, and graceful cleanup
func (g *GitManager) CloneWithConfig(ctx context.Context, cfg *CloneConfig) error {
	if cfg == nil {
		cfg = DefaultCloneConfig()
	}

	g.ws.SetPhase(PhaseCloning)

	// Create timeout context
	cloneCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	// Parse GitHub URL for size validation
	owner, repo, err := ParseGitHubURL(g.ws.RepoURL)
	if err == nil {
		// Validate repository size (GitHub only)
		repoInfo, err := ValidateRepoSize(cloneCtx, owner, repo, cfg.MaxRepoSizeMB, g.token)
		if err != nil {
			g.ws.SetPhase(PhaseFailed)
			return err
		}
		if repoInfo != nil {
			log.Info().
				Str("repo", repoInfo.FullName).
				Int64("size_kb", repoInfo.Size).
				Str("default_branch", repoInfo.DefaultBranch).
				Msg("repository validated")
		}
	}

	// Check disk space
	if err := CheckDiskSpace(filepath.Dir(g.ws.RepoPath), cfg.MinDiskSpaceMB); err != nil {
		g.ws.SetPhase(PhaseFailed)
		return &CloneError{
			Op:  "disk_check",
			URL: g.ws.RepoURL,
			Err: err,
		}
	}

	// Build clone options
	cloneOpts := &git.CloneOptions{
		URL: g.ws.RepoURL,
	}

	if cfg.ProgressWriter != nil {
		cloneOpts.Progress = cfg.ProgressWriter
	}

	if cfg.ShallowClone {
		cloneOpts.Depth = 1
	}

	if cfg.SingleBranch && g.ws.Branch != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(g.ws.Branch)
		cloneOpts.SingleBranch = true
	}

	if g.token != "" {
		cloneOpts.Auth = &http.BasicAuth{
			Username: "git",
			Password: g.token,
		}
	}

	log.Info().
		Str("url", g.ws.RepoURL).
		Str("path", g.ws.RepoPath).
		Dur("timeout", cfg.Timeout).
		Bool("shallow", cfg.ShallowClone).
		Msg("cloning repository")

	// Clone with retry logic
	var cloneErr error
	for attempt := 0; attempt <= cfg.RetryCount; attempt++ {
		if attempt > 0 {
			log.Warn().
				Int("attempt", attempt+1).
				Int("max_attempts", cfg.RetryCount+1).
				Msg("retrying clone")
			time.Sleep(cfg.RetryDelay)
		}

		g.repo, cloneErr = git.PlainCloneContext(cloneCtx, g.ws.RepoPath, false, cloneOpts)
		if cloneErr == nil {
			break
		}

		// Check if context was cancelled (timeout or manual cancel)
		if cloneCtx.Err() != nil {
			// Cleanup partial clone
			CleanupPartialClone(g.ws.RepoPath)
			g.ws.SetPhase(PhaseFailed)
			return &CloneError{
				Op:      "clone",
				URL:     g.ws.RepoURL,
				Err:     cloneCtx.Err(),
				Timeout: cloneCtx.Err() == context.DeadlineExceeded,
			}
		}

		// Check if error is transient and worth retrying
		if !IsTransientError(cloneErr) || attempt == cfg.RetryCount {
			break
		}

		// Cleanup before retry
		CleanupPartialClone(g.ws.RepoPath)
	}

	if cloneErr != nil {
		// Cleanup on failure
		CleanupPartialClone(g.ws.RepoPath)
		g.ws.SetPhase(PhaseFailed)
		return &CloneError{
			Op:  "clone",
			URL: g.ws.RepoURL,
			Err: cloneErr,
		}
	}

	// Get HEAD info
	head, err := g.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	g.ws.CommitSHA = head.Hash().String()
	g.ws.BaseBranch = head.Name().Short()

	log.Info().
		Str("commit", g.ws.CommitSHA[:8]).
		Str("branch", g.ws.BaseBranch).
		Msg("repository cloned")

	return g.ws.Save()
}

// CreateTestBranch creates a new branch for generated tests
func (g *GitManager) CreateTestBranch(name string) error {
	if g.repo == nil {
		var err error
		g.repo, err = git.PlainOpen(g.ws.RepoPath)
		if err != nil {
			return fmt.Errorf("failed to open repo: %w", err)
		}
	}

	worktree, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Create and checkout new branch
	branchRef := plumbing.NewBranchReferenceName(name)
	head, err := g.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash:   head.Hash(),
		Branch: branchRef,
		Create: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	g.ws.Branch = name
	log.Info().Str("branch", name).Msg("created test branch")

	return g.ws.Save()
}

// CommitTest commits a generated test file
func (g *GitManager) CommitTest(testFile, targetName string) (string, error) {
	if g.repo == nil {
		var err error
		g.repo, err = git.PlainOpen(g.ws.RepoPath)
		if err != nil {
			return "", fmt.Errorf("failed to open repo: %w", err)
		}
	}

	worktree, err := g.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get relative path
	relPath, err := filepath.Rel(g.ws.RepoPath, testFile)
	if err != nil {
		relPath = testFile
	}

	// Stage the file
	_, err = worktree.Add(relPath)
	if err != nil {
		return "", fmt.Errorf("failed to stage file: %w", err)
	}

	// Commit
	commitMsg := fmt.Sprintf("test: add generated test for %s\n\nGenerated by QTest", targetName)
	commit, err := worktree.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "QTest",
			Email: "qtest@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}

	commitHash := commit.String()
	log.Debug().
		Str("file", relPath).
		Str("commit", commitHash[:8]).
		Msg("committed test")

	return commitHash, nil
}

// CommitAll commits all staged changes
func (g *GitManager) CommitAll(message string) (string, error) {
	if g.repo == nil {
		var err error
		g.repo, err = git.PlainOpen(g.ws.RepoPath)
		if err != nil {
			return "", fmt.Errorf("failed to open repo: %w", err)
		}
	}

	worktree, err := g.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Stage all changes
	err = worktree.AddGlob(".")
	if err != nil {
		return "", fmt.Errorf("failed to stage changes: %w", err)
	}

	// Check if there are changes to commit
	status, err := worktree.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	if status.IsClean() {
		return "", nil // Nothing to commit
	}

	// Commit
	commit, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "QTest",
			Email: "qtest@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}

	return commit.String(), nil
}

// Push pushes commits to remote
func (g *GitManager) Push(ctx context.Context) error {
	if g.repo == nil {
		var err error
		g.repo, err = git.PlainOpen(g.ws.RepoPath)
		if err != nil {
			return fmt.Errorf("failed to open repo: %w", err)
		}
	}

	pushOpts := &git.PushOptions{}
	if g.token != "" {
		pushOpts.Auth = &http.BasicAuth{
			Username: "git",
			Password: g.token,
		}
	}

	err := g.repo.PushContext(ctx, pushOpts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to push: %w", err)
	}

	log.Info().Str("branch", g.ws.Branch).Msg("pushed to remote")
	return nil
}

// GetStatus returns the git status
func (g *GitManager) GetStatus() (map[string]string, error) {
	if g.repo == nil {
		var err error
		g.repo, err = git.PlainOpen(g.ws.RepoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open repo: %w", err)
		}
	}

	worktree, err := g.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	result := make(map[string]string)
	for file, s := range status {
		result[file] = string(s.Staging) + string(s.Worktree)
	}

	return result, nil
}

// GetCommitCount returns the number of commits ahead of base branch
func (g *GitManager) GetCommitCount() (int, error) {
	if g.repo == nil {
		var err error
		g.repo, err = git.PlainOpen(g.ws.RepoPath)
		if err != nil {
			return 0, fmt.Errorf("failed to open repo: %w", err)
		}
	}

	// Get current HEAD
	head, err := g.repo.Head()
	if err != nil {
		return 0, err
	}

	// Count commits
	iter, err := g.repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return 0, err
	}

	count := 0
	iter.ForEach(func(c *object.Commit) error {
		if c.Hash.String() == g.ws.CommitSHA {
			return fmt.Errorf("stop") // Stop at base commit
		}
		count++
		return nil
	})

	return count, nil
}
