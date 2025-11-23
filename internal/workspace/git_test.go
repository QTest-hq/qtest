package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestNewGitManager(t *testing.T) {
	ws := &Workspace{
		ID:       "test-id",
		RepoURL:  "https://github.com/test/repo",
		RepoPath: "/tmp/repo",
	}

	gm := NewGitManager(ws, "test-token")

	if gm == nil {
		t.Fatal("NewGitManager returned nil")
	}

	if gm.ws != ws {
		t.Error("workspace not set correctly")
	}

	if gm.token != "test-token" {
		t.Errorf("token = %s, want test-token", gm.token)
	}
}

func TestNewGitManager_EmptyToken(t *testing.T) {
	ws := &Workspace{
		ID:       "test-id",
		RepoURL:  "https://github.com/test/repo",
		RepoPath: "/tmp/repo",
	}

	gm := NewGitManager(ws, "")

	if gm == nil {
		t.Fatal("NewGitManager returned nil")
	}

	if gm.token != "" {
		t.Errorf("token = %s, want empty", gm.token)
	}
}

// Helper to create a test git repository
func createTestRepo(t *testing.T) (string, *git.Repository) {
	t.Helper()

	tmpDir := t.TempDir()

	// Initialize git repo
	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	// Create initial file
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Add and commit
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	_, err = wt.Add("README.md")
	if err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	_, err = wt.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	return tmpDir, repo
}

func TestGitManager_CreateTestBranch(t *testing.T) {
	tmpDir, repo := createTestRepo(t)

	// Create workspace with the test repo
	wsPath := filepath.Join(tmpDir, ".qtest-ws")
	os.MkdirAll(wsPath, 0755)

	ws := &Workspace{
		ID:       "test-id",
		RepoPath: tmpDir,
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
		path: wsPath,
	}

	gm := &GitManager{
		ws:    ws,
		repo:  repo,
		token: "",
	}

	// Create test branch
	err := gm.CreateTestBranch("qtest/generated-tests")
	if err != nil {
		t.Fatalf("CreateTestBranch error: %v", err)
	}

	// Verify branch was created
	if ws.Branch != "qtest/generated-tests" {
		t.Errorf("Branch = %s, want qtest/generated-tests", ws.Branch)
	}

	// Verify we're on the new branch
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}

	if head.Name().Short() != "qtest/generated-tests" {
		t.Errorf("HEAD branch = %s, want qtest/generated-tests", head.Name().Short())
	}
}

func TestGitManager_CreateTestBranch_NilRepo(t *testing.T) {
	tmpDir, _ := createTestRepo(t)

	wsPath := filepath.Join(tmpDir, ".qtest-ws")
	os.MkdirAll(wsPath, 0755)

	ws := &Workspace{
		ID:       "test-id",
		RepoPath: tmpDir,
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
		path: wsPath,
	}

	// GitManager with nil repo - should open repo automatically
	gm := &GitManager{
		ws:    ws,
		repo:  nil,
		token: "",
	}

	err := gm.CreateTestBranch("new-branch")
	if err != nil {
		t.Fatalf("CreateTestBranch error: %v", err)
	}

	if ws.Branch != "new-branch" {
		t.Errorf("Branch = %s, want new-branch", ws.Branch)
	}
}

func TestGitManager_CommitTest(t *testing.T) {
	tmpDir, repo := createTestRepo(t)

	wsPath := filepath.Join(tmpDir, ".qtest-ws")
	os.MkdirAll(wsPath, 0755)

	ws := &Workspace{
		ID:       "test-id",
		RepoPath: tmpDir,
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
		path: wsPath,
	}

	gm := &GitManager{
		ws:    ws,
		repo:  repo,
		token: "",
	}

	// Create a test file to commit
	testFile := filepath.Join(tmpDir, "test_example.go")
	if err := os.WriteFile(testFile, []byte("package test\n\nfunc TestExample(t *testing.T) {}"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Commit the test
	commitHash, err := gm.CommitTest(testFile, "ExampleFunc")
	if err != nil {
		t.Fatalf("CommitTest error: %v", err)
	}

	if commitHash == "" {
		t.Error("expected non-empty commit hash")
	}

	if len(commitHash) != 40 {
		t.Errorf("commit hash length = %d, want 40", len(commitHash))
	}
}

func TestGitManager_CommitAll(t *testing.T) {
	tmpDir, repo := createTestRepo(t)

	wsPath := filepath.Join(tmpDir, ".qtest-ws")
	os.MkdirAll(wsPath, 0755)

	ws := &Workspace{
		ID:       "test-id",
		RepoPath: tmpDir,
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
		path: wsPath,
	}

	gm := &GitManager{
		ws:    ws,
		repo:  repo,
		token: "",
	}

	// Create multiple test files
	for i := 1; i <= 3; i++ {
		testFile := filepath.Join(tmpDir, "test_file_"+string(rune('0'+i))+".go")
		content := "package test\n\nfunc TestFunc" + string(rune('0'+i)) + "(t *testing.T) {}"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
	}

	// Commit all changes
	commitHash, err := gm.CommitAll("test: add generated tests")
	if err != nil {
		t.Fatalf("CommitAll error: %v", err)
	}

	if commitHash == "" {
		t.Error("expected non-empty commit hash")
	}
}

func TestGitManager_CommitAll_NoChanges(t *testing.T) {
	tmpDir, repo := createTestRepo(t)

	wsPath := filepath.Join(tmpDir, ".qtest-ws")
	os.MkdirAll(wsPath, 0755)

	ws := &Workspace{
		ID:       "test-id",
		RepoPath: tmpDir,
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
		path: wsPath,
	}

	gm := &GitManager{
		ws:    ws,
		repo:  repo,
		token: "",
	}

	// Commit with no changes
	commitHash, err := gm.CommitAll("empty commit")
	if err != nil {
		t.Fatalf("CommitAll error: %v", err)
	}

	// Should return empty hash when nothing to commit
	if commitHash != "" {
		t.Errorf("expected empty commit hash for no changes, got %s", commitHash)
	}
}

func TestGitManager_GetStatus(t *testing.T) {
	tmpDir, repo := createTestRepo(t)

	ws := &Workspace{
		ID:       "test-id",
		RepoPath: tmpDir,
	}

	gm := &GitManager{
		ws:    ws,
		repo:  repo,
		token: "",
	}

	// Initially clean
	status, err := gm.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus error: %v", err)
	}

	if len(status) != 0 {
		t.Errorf("expected empty status for clean repo, got %d entries", len(status))
	}

	// Create an untracked file
	untrackedFile := filepath.Join(tmpDir, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("untracked"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Modify existing file
	readmeFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Modified"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	// Get status again
	status, err = gm.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus error: %v", err)
	}

	if len(status) < 2 {
		t.Errorf("expected at least 2 status entries, got %d", len(status))
	}
}

func TestGitManager_GetStatus_NilRepo(t *testing.T) {
	tmpDir, _ := createTestRepo(t)

	ws := &Workspace{
		ID:       "test-id",
		RepoPath: tmpDir,
	}

	// GitManager with nil repo
	gm := &GitManager{
		ws:    ws,
		repo:  nil,
		token: "",
	}

	// Should open repo automatically
	status, err := gm.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus error: %v", err)
	}

	// Should be empty for clean repo
	if len(status) != 0 {
		t.Errorf("expected empty status, got %d entries", len(status))
	}
}

func TestGitManager_GetCommitCount(t *testing.T) {
	tmpDir, repo := createTestRepo(t)

	wsPath := filepath.Join(tmpDir, ".qtest-ws")
	os.MkdirAll(wsPath, 0755)

	// Get initial commit SHA
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}

	ws := &Workspace{
		ID:        "test-id",
		RepoPath:  tmpDir,
		CommitSHA: head.Hash().String(),
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
		path: wsPath,
	}

	gm := &GitManager{
		ws:    ws,
		repo:  repo,
		token: "",
	}

	// Initially 0 commits ahead
	count, err := gm.GetCommitCount()
	if err != nil {
		t.Fatalf("GetCommitCount error: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 commits ahead, got %d", count)
	}

	// Add some commits
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(tmpDir, "file_"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		wt, err := repo.Worktree()
		if err != nil {
			t.Fatalf("failed to get worktree: %v", err)
		}

		_, err = wt.Add("file_" + string(rune('a'+i)) + ".txt")
		if err != nil {
			t.Fatalf("failed to add file: %v", err)
		}

		_, err = wt.Commit("Add file "+string(rune('a'+i)), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}
	}

	// Now should be 3 commits ahead
	count, err = gm.GetCommitCount()
	if err != nil {
		t.Fatalf("GetCommitCount error: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 commits ahead, got %d", count)
	}
}

func TestGitManager_Clone_DefaultConfig(t *testing.T) {
	// Test that Clone calls CloneWithConfig with default config
	// This is more of a unit test to verify the method signature works
	ws := &Workspace{
		ID:       "test-id",
		RepoURL:  "https://github.com/invalid/repo",
		RepoPath: "/tmp/nonexistent",
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
	}

	gm := NewGitManager(ws, "")

	// This will fail because the repo doesn't exist, but we're testing the path
	ctx := context.Background()
	err := gm.Clone(ctx)

	// Expected to fail with clone error
	if err == nil {
		t.Error("expected error for invalid repo")
	}

	// Should have set phase to failed
	if ws.State.Phase != PhaseFailed {
		t.Errorf("Phase = %s, want failed", ws.State.Phase)
	}
}

func TestGitManager_CloneWithConfig_NilConfig(t *testing.T) {
	ws := &Workspace{
		ID:       "test-id",
		RepoURL:  "https://github.com/invalid/repo",
		RepoPath: "/tmp/nonexistent",
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
	}

	gm := NewGitManager(ws, "")

	// CloneWithConfig with nil config should use defaults
	ctx := context.Background()
	err := gm.CloneWithConfig(ctx, nil)

	// Expected to fail because the repo doesn't exist
	if err == nil {
		t.Error("expected error for invalid repo")
	}
}

func TestGitManager_CommitTest_NilRepo(t *testing.T) {
	tmpDir, _ := createTestRepo(t)

	wsPath := filepath.Join(tmpDir, ".qtest-ws")
	os.MkdirAll(wsPath, 0755)

	ws := &Workspace{
		ID:       "test-id",
		RepoPath: tmpDir,
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
		path: wsPath,
	}

	// GitManager with nil repo
	gm := &GitManager{
		ws:    ws,
		repo:  nil,
		token: "",
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "new_test.go")
	if err := os.WriteFile(testFile, []byte("package test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Should open repo automatically
	commitHash, err := gm.CommitTest(testFile, "TestFunc")
	if err != nil {
		t.Fatalf("CommitTest error: %v", err)
	}

	if commitHash == "" {
		t.Error("expected non-empty commit hash")
	}
}

func TestGitManager_CommitAll_NilRepo(t *testing.T) {
	tmpDir, _ := createTestRepo(t)

	wsPath := filepath.Join(tmpDir, ".qtest-ws")
	os.MkdirAll(wsPath, 0755)

	ws := &Workspace{
		ID:       "test-id",
		RepoPath: tmpDir,
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
		path: wsPath,
	}

	// GitManager with nil repo
	gm := &GitManager{
		ws:    ws,
		repo:  nil,
		token: "",
	}

	// Create a file
	testFile := filepath.Join(tmpDir, "another_test.go")
	if err := os.WriteFile(testFile, []byte("package test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Should open repo automatically
	commitHash, err := gm.CommitAll("test commit")
	if err != nil {
		t.Fatalf("CommitAll error: %v", err)
	}

	if commitHash == "" {
		t.Error("expected non-empty commit hash")
	}
}

func TestGitManager_GetCommitCount_NilRepo(t *testing.T) {
	tmpDir, repo := createTestRepo(t)

	// Get initial commit SHA
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}

	ws := &Workspace{
		ID:        "test-id",
		RepoPath:  tmpDir,
		CommitSHA: head.Hash().String(),
	}

	// GitManager with nil repo
	gm := &GitManager{
		ws:    ws,
		repo:  nil,
		token: "",
	}

	// Should open repo automatically
	count, err := gm.GetCommitCount()
	if err != nil {
		t.Fatalf("GetCommitCount error: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestGitManager_Push_NilRepo(t *testing.T) {
	tmpDir, _ := createTestRepo(t)

	ws := &Workspace{
		ID:       "test-id",
		RepoPath: tmpDir,
		Branch:   "main",
	}

	// GitManager with nil repo
	gm := &GitManager{
		ws:    ws,
		repo:  nil,
		token: "",
	}

	// Push will fail because there's no remote, but it should open the repo first
	ctx := context.Background()
	err := gm.Push(ctx)

	// Expected to fail because there's no remote
	if err == nil {
		t.Error("expected error for repo with no remote")
	}
}

// Test error paths
func TestGitManager_CreateTestBranch_InvalidPath(t *testing.T) {
	ws := &Workspace{
		ID:       "test-id",
		RepoPath: "/nonexistent/path",
	}

	gm := &GitManager{
		ws:    ws,
		repo:  nil,
		token: "",
	}

	err := gm.CreateTestBranch("new-branch")
	if err == nil {
		t.Error("expected error for invalid repo path")
	}
}

func TestGitManager_CommitTest_InvalidPath(t *testing.T) {
	ws := &Workspace{
		ID:       "test-id",
		RepoPath: "/nonexistent/path",
	}

	gm := &GitManager{
		ws:    ws,
		repo:  nil,
		token: "",
	}

	_, err := gm.CommitTest("/nonexistent/file.go", "TestFunc")
	if err == nil {
		t.Error("expected error for invalid repo path")
	}
}

func TestGitManager_GetStatus_InvalidPath(t *testing.T) {
	ws := &Workspace{
		ID:       "test-id",
		RepoPath: "/nonexistent/path",
	}

	gm := &GitManager{
		ws:    ws,
		repo:  nil,
		token: "",
	}

	_, err := gm.GetStatus()
	if err == nil {
		t.Error("expected error for invalid repo path")
	}
}

func TestGitManager_GetCommitCount_InvalidPath(t *testing.T) {
	ws := &Workspace{
		ID:       "test-id",
		RepoPath: "/nonexistent/path",
	}

	gm := &GitManager{
		ws:    ws,
		repo:  nil,
		token: "",
	}

	_, err := gm.GetCommitCount()
	if err == nil {
		t.Error("expected error for invalid repo path")
	}
}
