package workspace

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestDefaultCloneConfig(t *testing.T) {
	cfg := DefaultCloneConfig()

	if cfg.Timeout != 10*time.Minute {
		t.Errorf("expected timeout to be 10 minutes, got %v", cfg.Timeout)
	}

	if cfg.MaxRepoSizeMB != 500 {
		t.Errorf("expected max repo size to be 500MB, got %d", cfg.MaxRepoSizeMB)
	}

	if cfg.MinDiskSpaceMB != 1024 {
		t.Errorf("expected min disk space to be 1024MB, got %d", cfg.MinDiskSpaceMB)
	}

	if !cfg.ShallowClone {
		t.Error("expected shallow clone to be enabled by default")
	}

	if !cfg.SingleBranch {
		t.Error("expected single branch to be enabled by default")
	}

	if cfg.RetryCount != 3 {
		t.Errorf("expected retry count to be 3, got %d", cfg.RetryCount)
	}
}

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		url      string
		owner    string
		repo     string
		hasError bool
	}{
		{
			url:      "https://github.com/owner/repo",
			owner:    "owner",
			repo:     "repo",
			hasError: false,
		},
		{
			url:      "https://github.com/owner/repo.git",
			owner:    "owner",
			repo:     "repo",
			hasError: false,
		},
		{
			url:      "git@github.com:owner/repo.git",
			owner:    "owner",
			repo:     "repo",
			hasError: false,
		},
		{
			url:      "git@github.com:owner/repo",
			owner:    "owner",
			repo:     "repo",
			hasError: false,
		},
		{
			url:      "https://gitlab.com/owner/repo",
			owner:    "",
			repo:     "",
			hasError: true,
		},
		{
			url:      "invalid-url",
			owner:    "",
			repo:     "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			owner, repo, err := ParseGitHubURL(tt.url)

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error for URL %s", tt.url)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for URL %s: %v", tt.url, err)
				return
			}

			if owner != tt.owner {
				t.Errorf("expected owner %s, got %s", tt.owner, owner)
			}

			if repo != tt.repo {
				t.Errorf("expected repo %s, got %s", tt.repo, repo)
			}
		})
	}
}

func TestCloneError(t *testing.T) {
	t.Run("timeout error", func(t *testing.T) {
		err := &CloneError{
			Op:      "clone",
			URL:     "https://github.com/owner/repo",
			Err:     context.DeadlineExceeded,
			Timeout: true,
		}

		if err.Error() != "clone timeout: https://github.com/owner/repo after clone" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})

	t.Run("size error", func(t *testing.T) {
		err := &CloneError{
			Op:      "size_validation",
			URL:     "https://github.com/owner/repo",
			Err:     errors.New("repository size 600MB exceeds limit of 500MB"),
			SizeErr: true,
		}

		if err.Error() != "clone size error: https://github.com/owner/repo - repository size 600MB exceeds limit of 500MB" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})

	t.Run("generic error", func(t *testing.T) {
		underlying := errors.New("network error")
		err := &CloneError{
			Op:  "clone",
			URL: "https://github.com/owner/repo",
			Err: underlying,
		}

		if err.Unwrap() != underlying {
			t.Error("Unwrap() should return underlying error")
		}
	})
}

func TestIsTransientError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if IsTransientError(nil) {
			t.Error("expected nil error to not be transient")
		}
	})

	transientTests := []struct {
		name      string
		err       error
		transient bool
	}{
		{"connection reset", errors.New("connection reset by peer"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"timed out", errors.New("operation timed out"), true},
		{"temp failure", errors.New("temporary failure in name resolution"), true},
		{"no such host", errors.New("no such host"), true},
		{"network unreachable", errors.New("network is unreachable"), true},
		{"TLS handshake", errors.New("TLS handshake timeout"), true},
		{"EOF", errors.New("unexpected EOF"), true},
		{"broken pipe", errors.New("broken pipe"), true},
		{"503", errors.New("503 Service Unavailable"), true},
		{"502", errors.New("502 Bad Gateway"), true},
		{"auth required", errors.New("authentication required"), false},
		{"not found", errors.New("repository not found"), false},
		{"permission denied", errors.New("permission denied"), false},
	}

	for _, tt := range transientTests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTransientError(tt.err)
			if result != tt.transient {
				t.Errorf("expected IsTransientError(%q) = %v, got %v", tt.err, tt.transient, result)
			}
		})
	}
}

func TestCheckDiskSpace(t *testing.T) {
	t.Run("no limit", func(t *testing.T) {
		err := CheckDiskSpace("/tmp", 0)
		if err != nil {
			t.Errorf("expected no error with 0 limit, got %v", err)
		}
	})

	t.Run("reasonable limit", func(t *testing.T) {
		// Most systems should have at least 100MB free in /tmp
		err := CheckDiskSpace("/tmp", 100)
		if err != nil {
			t.Logf("disk space check failed (may be low disk): %v", err)
		}
	})

	t.Run("unreasonable limit", func(t *testing.T) {
		// 10TB should exceed available space
		err := CheckDiskSpace("/tmp", 10*1024*1024)
		if err == nil {
			t.Error("expected error for 10TB requirement")
		}
	})
}

func TestCleanupPartialClone(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		err := CleanupPartialClone("")
		if err != nil {
			t.Errorf("expected no error for empty path, got %v", err)
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		err := CleanupPartialClone("/tmp/.qtest/nonexistent-test-dir")
		if err != nil {
			t.Errorf("expected no error for non-existent path, got %v", err)
		}
	})

	t.Run("safety check for non-workspace path", func(t *testing.T) {
		// Should refuse to delete non-workspace directories
		err := CleanupPartialClone("/tmp/some-random-dir")
		if err != nil {
			t.Errorf("expected no error (but also no deletion), got %v", err)
		}
	})

	t.Run("cleanup workspace path", func(t *testing.T) {
		// Create a test directory with .qtest in path
		testDir, err := os.MkdirTemp("", ".qtest-test-cleanup")
		if err != nil {
			t.Fatal(err)
		}

		// Create a file in it
		testFile := testDir + "/test.txt"
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		// Cleanup should work
		err = CleanupPartialClone(testDir)
		if err != nil {
			t.Errorf("expected cleanup to succeed, got %v", err)
		}

		// Directory should be gone
		if _, err := os.Stat(testDir); !os.IsNotExist(err) {
			t.Error("expected directory to be deleted")
		}
	})
}

func TestValidateRepoSize(t *testing.T) {
	ctx := context.Background()

	t.Run("no limit", func(t *testing.T) {
		info, err := ValidateRepoSize(ctx, "owner", "repo", 0, "")
		if err != nil {
			t.Errorf("expected no error with 0 limit, got %v", err)
		}
		if info != nil {
			t.Error("expected nil info with no limit")
		}
	})

	t.Run("public repo", func(t *testing.T) {
		// Test with a known small public repo
		info, err := ValidateRepoSize(ctx, "octocat", "Hello-World", 1000, "")
		if err != nil {
			t.Logf("validation failed (expected in CI without token): %v", err)
		}
		if info != nil {
			t.Logf("repo info: size=%dKB, branch=%s", info.Size, info.DefaultBranch)
		}
	})

	t.Run("non-existent repo", func(t *testing.T) {
		_, err := ValidateRepoSize(ctx, "nonexistent-owner-12345", "nonexistent-repo-67890", 500, "")
		if err != nil {
			// Expected for non-existent repos
			t.Logf("got expected error: %v", err)
		}
	})
}

func TestGitHubRepoInfo(t *testing.T) {
	info := GitHubRepoInfo{
		Size:          1024,
		DefaultBranch: "main",
		Private:       false,
		Archived:      false,
		Disabled:      false,
		FullName:      "owner/repo",
	}

	if info.Size != 1024 {
		t.Error("size mismatch")
	}
	if info.DefaultBranch != "main" {
		t.Error("default branch mismatch")
	}
	if info.FullName != "owner/repo" {
		t.Error("full name mismatch")
	}
}
