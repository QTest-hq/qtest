package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/parser"
)

func TestPhase_Constants(t *testing.T) {
	tests := []struct {
		phase Phase
		want  string
	}{
		{PhaseInit, "init"},
		{PhaseCloning, "cloning"},
		{PhaseParsing, "parsing"},
		{PhasePlanning, "planning"},
		{PhaseGenerating, "generating"},
		{PhasePaused, "paused"},
		{PhaseCompleted, "completed"},
		{PhaseFailed, "failed"},
	}

	for _, tt := range tests {
		if string(tt.phase) != tt.want {
			t.Errorf("Phase %v = %s, want %s", tt.phase, string(tt.phase), tt.want)
		}
	}
}

func TestTargetStatus_Constants(t *testing.T) {
	tests := []struct {
		status TargetStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusSkipped, "skipped"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("TargetStatus %v = %s, want %s", tt.status, string(tt.status), tt.want)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if cfg.BaseDir == "" {
		t.Error("BaseDir should not be empty")
	}

	// Should contain .qtest/workspaces
	if !contains(cfg.BaseDir, ".qtest") {
		t.Errorf("BaseDir = %s, should contain .qtest", cfg.BaseDir)
	}
}

func TestWorkspaceConfig_Fields(t *testing.T) {
	cfg := &WorkspaceConfig{
		BaseDir:     "/tmp/workspaces",
		GitHubToken: "test-token",
	}

	if cfg.BaseDir != "/tmp/workspaces" {
		t.Errorf("BaseDir = %s, want /tmp/workspaces", cfg.BaseDir)
	}
	if cfg.GitHubToken != "test-token" {
		t.Errorf("GitHubToken = %s, want test-token", cfg.GitHubToken)
	}
}

func TestWorkspace_Fields(t *testing.T) {
	now := time.Now()
	ws := &Workspace{
		ID:         "abc123",
		Name:       "test-workspace",
		RepoURL:    "https://github.com/test/repo",
		RepoPath:   "/tmp/repo",
		Branch:     "feature",
		BaseBranch: "main",
		CommitSHA:  "abcdef123456",
		Language:   "go",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if ws.ID != "abc123" {
		t.Errorf("ID = %s, want abc123", ws.ID)
	}
	if ws.Name != "test-workspace" {
		t.Errorf("Name = %s, want test-workspace", ws.Name)
	}
	if ws.RepoURL != "https://github.com/test/repo" {
		t.Errorf("RepoURL mismatch")
	}
	if ws.Branch != "feature" {
		t.Errorf("Branch = %s, want feature", ws.Branch)
	}
	if ws.BaseBranch != "main" {
		t.Errorf("BaseBranch = %s, want main", ws.BaseBranch)
	}
	if ws.Language != "go" {
		t.Errorf("Language = %s, want go", ws.Language)
	}
}

func TestWorkspaceState_Fields(t *testing.T) {
	now := time.Now()
	state := &WorkspaceState{
		Phase:        PhaseGenerating,
		TotalTargets: 100,
		Completed:    50,
		Failed:       5,
		Skipped:      10,
		Targets:      make(map[string]*TargetState),
		CurrentIndex: 65,
		StartedAt:    &now,
	}

	if state.Phase != PhaseGenerating {
		t.Errorf("Phase = %s, want generating", state.Phase)
	}
	if state.TotalTargets != 100 {
		t.Errorf("TotalTargets = %d, want 100", state.TotalTargets)
	}
	if state.Completed != 50 {
		t.Errorf("Completed = %d, want 50", state.Completed)
	}
	if state.Failed != 5 {
		t.Errorf("Failed = %d, want 5", state.Failed)
	}
	if state.Skipped != 10 {
		t.Errorf("Skipped = %d, want 10", state.Skipped)
	}
}

func TestTargetState_Fields(t *testing.T) {
	now := time.Now()
	target := &TargetState{
		ID:          "file.go:10:TestFunc",
		File:        "file.go",
		Name:        "TestFunc",
		Type:        "function",
		Line:        10,
		Status:      StatusCompleted,
		Covered:     true,
		SpecID:      "spec-123",
		TestFile:    "file_test.go",
		CommitSHA:   "abc123",
		GeneratedAt: &now,
	}

	if target.ID != "file.go:10:TestFunc" {
		t.Errorf("ID = %s, want file.go:10:TestFunc", target.ID)
	}
	if target.File != "file.go" {
		t.Errorf("File = %s, want file.go", target.File)
	}
	if target.Name != "TestFunc" {
		t.Errorf("Name = %s, want TestFunc", target.Name)
	}
	if target.Status != StatusCompleted {
		t.Errorf("Status = %s, want completed", target.Status)
	}
	if !target.Covered {
		t.Error("Covered should be true")
	}
}

func TestWorkspace_Path(t *testing.T) {
	ws := &Workspace{
		path: "/tmp/workspace/abc123",
	}

	if ws.Path() != "/tmp/workspace/abc123" {
		t.Errorf("Path() = %s, want /tmp/workspace/abc123", ws.Path())
	}
}

func TestWorkspace_SetPhase(t *testing.T) {
	ws := &Workspace{
		State: &WorkspaceState{
			Phase: PhaseInit,
		},
	}

	ws.SetPhase(PhaseGenerating)

	if ws.State.Phase != PhaseGenerating {
		t.Errorf("Phase = %s, want generating", ws.State.Phase)
	}
}

func TestWorkspace_Progress(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		completed int
		failed    int
		skipped   int
		want      float64
	}{
		{"empty", 0, 0, 0, 0, 0},
		{"all completed", 10, 10, 0, 0, 100},
		{"half done", 100, 50, 0, 0, 50},
		{"with failures", 100, 40, 10, 0, 50},
		{"with skipped", 100, 30, 10, 10, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &Workspace{
				State: &WorkspaceState{
					TotalTargets: tt.total,
					Completed:    tt.completed,
					Failed:       tt.failed,
					Skipped:      tt.skipped,
				},
			}

			got := ws.Progress()
			if got != tt.want {
				t.Errorf("Progress() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestWorkspace_Summary(t *testing.T) {
	ws := &Workspace{
		ID:   "test-id",
		Name: "test-name",
		State: &WorkspaceState{
			Phase:        PhaseCompleted,
			TotalTargets: 100,
			Completed:    80,
			Failed:       10,
			Skipped:      5,
		},
	}

	summary := ws.Summary()

	if summary["id"] != "test-id" {
		t.Errorf("id = %v, want test-id", summary["id"])
	}
	if summary["name"] != "test-name" {
		t.Errorf("name = %v, want test-name", summary["name"])
	}
	if summary["phase"] != PhaseCompleted {
		t.Errorf("phase = %v, want completed", summary["phase"])
	}
	if summary["total"] != 100 {
		t.Errorf("total = %v, want 100", summary["total"])
	}
	if summary["completed"] != 80 {
		t.Errorf("completed = %v, want 80", summary["completed"])
	}
	if summary["failed"] != 10 {
		t.Errorf("failed = %v, want 10", summary["failed"])
	}
	if summary["pending"] != 5 {
		t.Errorf("pending = %v, want 5", summary["pending"])
	}
}

func TestWorkspace_AddTargets(t *testing.T) {
	ws := &Workspace{
		State: &WorkspaceState{
			TotalTargets: 0,
			Targets:      make(map[string]*TargetState),
		},
	}

	functions := []parser.Function{
		{Name: "PublicFunc", Exported: true, StartLine: 10},
		{Name: "privateFunc", Exported: false, StartLine: 20},
		{Name: "AnotherPublic", Exported: true, StartLine: 30},
	}

	ws.AddTargets(functions, "test.go")

	// Should only add exported functions
	if ws.State.TotalTargets != 2 {
		t.Errorf("TotalTargets = %d, want 2", ws.State.TotalTargets)
	}
	if len(ws.State.Targets) != 2 {
		t.Errorf("len(Targets) = %d, want 2", len(ws.State.Targets))
	}
}

func TestWorkspace_GetNextTarget(t *testing.T) {
	ws := &Workspace{
		State: &WorkspaceState{
			Targets: map[string]*TargetState{
				"1": {ID: "1", Status: StatusCompleted},
				"2": {ID: "2", Status: StatusPending},
				"3": {ID: "3", Status: StatusFailed},
			},
		},
	}

	next := ws.GetNextTarget()

	if next == nil {
		t.Fatal("GetNextTarget() returned nil")
	}
	if next.ID != "2" {
		t.Errorf("ID = %s, want 2", next.ID)
	}
}

func TestWorkspace_GetNextTarget_NoPending(t *testing.T) {
	ws := &Workspace{
		State: &WorkspaceState{
			Targets: map[string]*TargetState{
				"1": {ID: "1", Status: StatusCompleted},
				"2": {ID: "2", Status: StatusFailed},
			},
		},
	}

	next := ws.GetNextTarget()

	if next != nil {
		t.Error("GetNextTarget() should return nil when no pending targets")
	}
}

func TestWorkspace_UpdateTarget(t *testing.T) {
	ws := &Workspace{
		State: &WorkspaceState{
			Completed: 0,
			Failed:    0,
			Skipped:   0,
			Targets: map[string]*TargetState{
				"test": {ID: "test", Status: StatusPending},
			},
		},
	}

	ws.UpdateTarget("test", StatusCompleted, "test_file.go", nil)

	target := ws.State.Targets["test"]
	if target.Status != StatusCompleted {
		t.Errorf("Status = %s, want completed", target.Status)
	}
	if target.TestFile != "test_file.go" {
		t.Errorf("TestFile = %s, want test_file.go", target.TestFile)
	}
	if ws.State.Completed != 1 {
		t.Errorf("Completed = %d, want 1", ws.State.Completed)
	}
}

func TestWorkspace_UpdateTarget_WithError(t *testing.T) {
	ws := &Workspace{
		State: &WorkspaceState{
			Failed: 0,
			Targets: map[string]*TargetState{
				"test": {ID: "test", Status: StatusPending},
			},
		},
	}

	testErr := &testError{msg: "test error"}
	ws.UpdateTarget("test", StatusFailed, "", testErr)

	target := ws.State.Targets["test"]
	if target.Status != StatusFailed {
		t.Errorf("Status = %s, want failed", target.Status)
	}
	if target.Error != "test error" {
		t.Errorf("Error = %s, want 'test error'", target.Error)
	}
	if ws.State.Failed != 1 {
		t.Errorf("Failed = %d, want 1", ws.State.Failed)
	}
}

func TestWorkspace_UpdateTargetCovered(t *testing.T) {
	ws := &Workspace{
		State: &WorkspaceState{
			Targets: map[string]*TargetState{
				"test": {ID: "test", Status: StatusPending, Covered: false},
			},
		},
	}

	ws.UpdateTargetCovered("test", "spec-123")

	target := ws.State.Targets["test"]
	if target.Status != StatusCompleted {
		t.Errorf("Status = %s, want completed", target.Status)
	}
	if !target.Covered {
		t.Error("Covered should be true")
	}
	if target.SpecID != "spec-123" {
		t.Errorf("SpecID = %s, want spec-123", target.SpecID)
	}
}

func TestNew_CreatesWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &WorkspaceConfig{
		BaseDir: tmpDir,
	}

	ws, err := New("test-ws", "https://github.com/test/repo", cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if ws == nil {
		t.Fatal("New() returned nil")
	}
	if ws.Name != "test-ws" {
		t.Errorf("Name = %s, want test-ws", ws.Name)
	}
	if ws.RepoURL != "https://github.com/test/repo" {
		t.Errorf("RepoURL mismatch")
	}
	if ws.State == nil {
		t.Error("State should not be nil")
	}
	if ws.State.Phase != PhaseInit {
		t.Errorf("Phase = %s, want init", ws.State.Phase)
	}

	// Clean up
	os.RemoveAll(ws.Path())
}

func TestLoad_LoadsWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &WorkspaceConfig{
		BaseDir: tmpDir,
	}

	// Create a workspace first
	ws, err := New("test-ws", "https://github.com/test/repo", cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Load it back
	loaded, err := Load(ws.Path())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Name != ws.Name {
		t.Errorf("Name = %s, want %s", loaded.Name, ws.Name)
	}
	if loaded.RepoURL != ws.RepoURL {
		t.Errorf("RepoURL mismatch")
	}
}

func TestLoadByID(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &WorkspaceConfig{
		BaseDir: tmpDir,
	}

	// Create a workspace first
	ws, err := New("test-ws", "https://github.com/test/repo", cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Load by ID
	loaded, err := LoadByID(ws.ID, cfg)
	if err != nil {
		t.Fatalf("LoadByID() error = %v", err)
	}

	if loaded.ID != ws.ID {
		t.Errorf("ID = %s, want %s", loaded.ID, ws.ID)
	}
}

func TestListWorkspaces_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &WorkspaceConfig{
		BaseDir: tmpDir,
	}

	workspaces, err := ListWorkspaces(cfg)
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}

	if len(workspaces) != 0 {
		t.Errorf("len(workspaces) = %d, want 0", len(workspaces))
	}
}

func TestListWorkspaces_NonExistentDir(t *testing.T) {
	cfg := &WorkspaceConfig{
		BaseDir: "/nonexistent/path/that/does/not/exist",
	}

	workspaces, err := ListWorkspaces(cfg)
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}

	// Should return empty list, not error
	if len(workspaces) != 0 {
		t.Errorf("len(workspaces) = %d, want 0", len(workspaces))
	}
}

func TestListWorkspaces_WithWorkspaces(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &WorkspaceConfig{
		BaseDir: tmpDir,
	}

	// Create a few workspaces
	for i := 0; i < 3; i++ {
		_, err := New("test-ws", "https://github.com/test/repo", cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
	}

	workspaces, err := ListWorkspaces(cfg)
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}

	if len(workspaces) != 3 {
		t.Errorf("len(workspaces) = %d, want 3", len(workspaces))
	}
}

func TestWorkspace_Save(t *testing.T) {
	tmpDir := t.TempDir()
	wsPath := filepath.Join(tmpDir, "test-ws")
	os.MkdirAll(wsPath, 0755)

	ws := &Workspace{
		ID:      "test-id",
		Name:    "test-ws",
		RepoURL: "https://github.com/test/repo",
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
		path: wsPath,
	}

	if err := ws.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file was created
	statePath := filepath.Join(wsPath, "workspace.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("workspace.json was not created")
	}
}

// Helper types and functions

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
