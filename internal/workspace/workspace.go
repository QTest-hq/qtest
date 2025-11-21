package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/google/uuid"
)

// Workspace represents a local working environment for test generation
type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RepoURL   string    `json:"repo_url"`
	RepoPath  string    `json:"repo_path"`   // Where the repo is cloned
	Branch    string    `json:"branch"`       // Working branch for tests
	BaseBranch string   `json:"base_branch"`  // Original branch
	CommitSHA string    `json:"commit_sha"`
	Language  string    `json:"language"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// State tracking
	State *WorkspaceState `json:"state"`

	// Internal
	path string // Workspace root directory
	mu   sync.RWMutex
}

// WorkspaceState tracks the generation progress
type WorkspaceState struct {
	Phase        Phase                      `json:"phase"`
	TotalTargets int                        `json:"total_targets"`
	Completed    int                        `json:"completed"`
	Failed       int                        `json:"failed"`
	Skipped      int                        `json:"skipped"`
	Targets      map[string]*TargetState    `json:"targets"` // Function ID -> state
	CurrentIndex int                        `json:"current_index"`
	StartedAt    *time.Time                 `json:"started_at,omitempty"`
	PausedAt     *time.Time                 `json:"paused_at,omitempty"`
}

// Phase represents the current phase of generation
type Phase string

const (
	PhaseInit       Phase = "init"
	PhaseCloning    Phase = "cloning"
	PhaseParsing    Phase = "parsing"
	PhasePlanning   Phase = "planning"
	PhaseGenerating Phase = "generating"
	PhasePaused     Phase = "paused"
	PhaseCompleted  Phase = "completed"
	PhaseFailed     Phase = "failed"
)

// TargetState tracks state for a single function/method
type TargetState struct {
	ID           string         `json:"id"`           // Unique ID: file:line:name
	File         string         `json:"file"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`         // function, method, class
	Line         int            `json:"line"`
	Status       TargetStatus   `json:"status"`
	TestFile     string         `json:"test_file,omitempty"`
	CommitSHA    string         `json:"commit_sha,omitempty"`
	Error        string         `json:"error,omitempty"`
	GeneratedAt  *time.Time     `json:"generated_at,omitempty"`
	DSL          json.RawMessage `json:"dsl,omitempty"`
}

// TargetStatus represents the status of a target
type TargetStatus string

const (
	StatusPending   TargetStatus = "pending"
	StatusRunning   TargetStatus = "running"
	StatusCompleted TargetStatus = "completed"
	StatusFailed    TargetStatus = "failed"
	StatusSkipped   TargetStatus = "skipped"
)

// WorkspaceConfig holds configuration for workspace operations
type WorkspaceConfig struct {
	BaseDir     string // Base directory for all workspaces
	GitHubToken string
}

// DefaultConfig returns default workspace configuration
func DefaultConfig() *WorkspaceConfig {
	homeDir, _ := os.UserHomeDir()
	return &WorkspaceConfig{
		BaseDir: filepath.Join(homeDir, ".qtest", "workspaces"),
	}
}

// New creates a new workspace
func New(name, repoURL string, cfg *WorkspaceConfig) (*Workspace, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	id := uuid.New().String()[:8]
	wsPath := filepath.Join(cfg.BaseDir, id)

	// Create workspace directory
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	ws := &Workspace{
		ID:        id,
		Name:      name,
		RepoURL:   repoURL,
		RepoPath:  filepath.Join(wsPath, "repo"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		State: &WorkspaceState{
			Phase:   PhaseInit,
			Targets: make(map[string]*TargetState),
		},
		path: wsPath,
	}

	// Save initial state
	if err := ws.Save(); err != nil {
		return nil, err
	}

	return ws, nil
}

// Load loads an existing workspace
func Load(wsPath string) (*Workspace, error) {
	statePath := filepath.Join(wsPath, "workspace.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workspace state: %w", err)
	}

	var ws Workspace
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("failed to parse workspace state: %w", err)
	}

	ws.path = wsPath
	return &ws, nil
}

// LoadByID loads a workspace by ID
func LoadByID(id string, cfg *WorkspaceConfig) (*Workspace, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return Load(filepath.Join(cfg.BaseDir, id))
}

// Save persists workspace state to disk
func (ws *Workspace) Save() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workspace: %w", err)
	}

	statePath := filepath.Join(ws.path, "workspace.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write workspace state: %w", err)
	}

	return nil
}

// Path returns the workspace root directory
func (ws *Workspace) Path() string {
	return ws.path
}

// SetPhase updates the workspace phase
func (ws *Workspace) SetPhase(phase Phase) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.State.Phase = phase
}

// AddTargets adds functions/methods to be processed
func (ws *Workspace) AddTargets(functions []parser.Function, filePath string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	for _, fn := range functions {
		if !fn.Exported {
			continue // Skip private functions
		}

		id := fn.ID
		if id == "" {
			id = fmt.Sprintf("%s:%d:%s", filePath, fn.StartLine, fn.Name)
		}

		ws.State.Targets[id] = &TargetState{
			ID:     id,
			File:   filePath,
			Name:   fn.Name,
			Type:   "function",
			Line:   fn.StartLine,
			Status: StatusPending,
		}
		ws.State.TotalTargets++
	}
}

// GetNextTarget returns the next pending target
func (ws *Workspace) GetNextTarget() *TargetState {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	for _, target := range ws.State.Targets {
		if target.Status == StatusPending {
			return target
		}
	}
	return nil
}

// UpdateTarget updates a target's state
func (ws *Workspace) UpdateTarget(id string, status TargetStatus, testFile string, err error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	target, ok := ws.State.Targets[id]
	if !ok {
		return
	}

	target.Status = status
	target.TestFile = testFile
	now := time.Now()
	target.GeneratedAt = &now

	if err != nil {
		target.Error = err.Error()
	}

	switch status {
	case StatusCompleted:
		ws.State.Completed++
	case StatusFailed:
		ws.State.Failed++
	case StatusSkipped:
		ws.State.Skipped++
	}
}

// Progress returns current progress as a percentage
func (ws *Workspace) Progress() float64 {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	if ws.State.TotalTargets == 0 {
		return 0
	}

	done := ws.State.Completed + ws.State.Failed + ws.State.Skipped
	return float64(done) / float64(ws.State.TotalTargets) * 100
}

// Summary returns a summary of the workspace state
func (ws *Workspace) Summary() map[string]interface{} {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	return map[string]interface{}{
		"id":        ws.ID,
		"name":      ws.Name,
		"phase":     ws.State.Phase,
		"total":     ws.State.TotalTargets,
		"completed": ws.State.Completed,
		"failed":    ws.State.Failed,
		"skipped":   ws.State.Skipped,
		"pending":   ws.State.TotalTargets - ws.State.Completed - ws.State.Failed - ws.State.Skipped,
		"progress":  fmt.Sprintf("%.1f%%", ws.Progress()),
	}
}

// ListWorkspaces returns all workspaces
func ListWorkspaces(cfg *WorkspaceConfig) ([]*Workspace, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	entries, err := os.ReadDir(cfg.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Workspace{}, nil
		}
		return nil, err
	}

	workspaces := make([]*Workspace, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			ws, err := Load(filepath.Join(cfg.BaseDir, entry.Name()))
			if err == nil {
				workspaces = append(workspaces, ws)
			}
		}
	}

	return workspaces, nil
}
