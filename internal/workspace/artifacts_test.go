package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewArtifactManager(t *testing.T) {
	ws := &Workspace{
		path: "/tmp/test-workspace",
	}

	am := NewArtifactManager(ws)

	if am == nil {
		t.Fatal("NewArtifactManager() returned nil")
	}
	if am.ws != ws {
		t.Error("ws reference mismatch")
	}
	if am.artifactDir != "/tmp/test-workspace/artifacts" {
		t.Errorf("artifactDir = %s, want /tmp/test-workspace/artifacts", am.artifactDir)
	}
}

func TestArtifactManager_Init(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{
		path: tmpDir,
	}

	am := NewArtifactManager(ws)

	if err := am.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify directory was created
	artifactDir := filepath.Join(tmpDir, "artifacts")
	if _, err := os.Stat(artifactDir); os.IsNotExist(err) {
		t.Error("artifacts directory was not created")
	}
}

func TestTestPlan_Fields(t *testing.T) {
	plan := &TestPlan{
		Version:   "1.0",
		Repo:      "https://github.com/test/repo",
		CommitSHA: "abc123",
		CreatedAt: time.Now(),
		Summary: TestPlanSummary{
			TotalTargets:         100,
			ByType:               map[string]int{"function": 80, "method": 20},
			ByFile:               map[string]int{"main.go": 50, "util.go": 50},
			EstimatedTimeMinutes: 30,
		},
		Targets: []PlanTarget{
			{ID: "1", Name: "TestFunc", File: "main.go"},
		},
	}

	if plan.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", plan.Version)
	}
	if plan.Summary.TotalTargets != 100 {
		t.Errorf("TotalTargets = %d, want 100", plan.Summary.TotalTargets)
	}
	if len(plan.Targets) != 1 {
		t.Errorf("len(Targets) = %d, want 1", len(plan.Targets))
	}
}

func TestPlanTarget_Fields(t *testing.T) {
	target := PlanTarget{
		ID:                 "main.go:10:TestFunc",
		Name:               "TestFunc",
		File:               "main.go",
		Line:               10,
		Type:               "function",
		Complexity:         "medium",
		Priority:           5,
		Dependencies:       []string{"dep1", "dep2"},
		SuggestedTestTypes: []string{"unit", "integration"},
	}

	if target.ID != "main.go:10:TestFunc" {
		t.Errorf("ID mismatch")
	}
	if target.Complexity != "medium" {
		t.Errorf("Complexity = %s, want medium", target.Complexity)
	}
	if target.Priority != 5 {
		t.Errorf("Priority = %d, want 5", target.Priority)
	}
	if len(target.Dependencies) != 2 {
		t.Errorf("len(Dependencies) = %d, want 2", len(target.Dependencies))
	}
}

func TestExecutionReport_Fields(t *testing.T) {
	report := &ExecutionReport{
		Version:         "1.0",
		ExecutedAt:      time.Now(),
		DurationSeconds: 120,
		Summary: ExecutionSummary{
			Total:    100,
			Passed:   90,
			Failed:   8,
			Skipped:  2,
			PassRate: 90.0,
		},
		Tests: []TestResult{
			{ID: "1", Name: "Test1", Status: "passed"},
		},
	}

	if report.DurationSeconds != 120 {
		t.Errorf("DurationSeconds = %d, want 120", report.DurationSeconds)
	}
	if report.Summary.Total != 100 {
		t.Errorf("Total = %d, want 100", report.Summary.Total)
	}
	if report.Summary.PassRate != 90.0 {
		t.Errorf("PassRate = %f, want 90.0", report.Summary.PassRate)
	}
}

func TestTestResult_Fields(t *testing.T) {
	result := TestResult{
		ID:         "test-1",
		Name:       "TestSomething",
		File:       "test.go",
		Target:     "Something",
		Status:     "failed",
		DurationMs: 150,
		Error:      "assertion failed",
		StackTrace: "at line 10...",
	}

	if result.ID != "test-1" {
		t.Errorf("ID = %s, want test-1", result.ID)
	}
	if result.Status != "failed" {
		t.Errorf("Status = %s, want failed", result.Status)
	}
	if result.DurationMs != 150 {
		t.Errorf("DurationMs = %d, want 150", result.DurationMs)
	}
	if result.Error != "assertion failed" {
		t.Errorf("Error mismatch")
	}
}

func TestCoverageReport_Fields(t *testing.T) {
	report := &CoverageReport{
		Version:     "1.0",
		GeneratedAt: time.Now(),
		Tool:        "go cover",
		Summary: CoverageSummary{
			TotalLines:      1000,
			CoveredLines:    800,
			CoveragePercent: 80.0,
			ByPackage:       map[string]float64{"main": 85.0, "util": 75.0},
		},
		Files: []FileCoverage{
			{Path: "main.go", TotalLines: 500, CoveredLines: 400, CoveragePercent: 80.0},
		},
	}

	if report.Tool != "go cover" {
		t.Errorf("Tool = %s, want go cover", report.Tool)
	}
	if report.Summary.CoveragePercent != 80.0 {
		t.Errorf("CoveragePercent = %f, want 80.0", report.Summary.CoveragePercent)
	}
	if len(report.Files) != 1 {
		t.Errorf("len(Files) = %d, want 1", len(report.Files))
	}
}

func TestFileCoverage_Fields(t *testing.T) {
	fc := FileCoverage{
		Path:            "main.go",
		TotalLines:      100,
		CoveredLines:    85,
		CoveragePercent: 85.0,
		UncoveredLines:  []int{10, 20, 30},
	}

	if fc.Path != "main.go" {
		t.Errorf("Path = %s, want main.go", fc.Path)
	}
	if fc.CoveragePercent != 85.0 {
		t.Errorf("CoveragePercent = %f, want 85.0", fc.CoveragePercent)
	}
	if len(fc.UncoveredLines) != 3 {
		t.Errorf("len(UncoveredLines) = %d, want 3", len(fc.UncoveredLines))
	}
}

func TestMutationReport_Fields(t *testing.T) {
	report := &MutationReport{
		Version:         "1.0",
		ExecutedAt:      time.Now(),
		DurationSeconds: 600,
		Summary: MutationSummary{
			TotalMutants:  100,
			Killed:        85,
			Survived:      10,
			Timeout:       5,
			MutationScore: 85.0,
		},
		ByTest: []TestMutations{
			{TestID: "test1", MutantsTested: 50, Killed: 45, Score: 90.0},
		},
		Survivors: []SurvivedMutant{
			{ID: "mut1", Operator: "MATH", Original: "+", Mutated: "-"},
		},
	}

	if report.Summary.MutationScore != 85.0 {
		t.Errorf("MutationScore = %f, want 85.0", report.Summary.MutationScore)
	}
	if len(report.ByTest) != 1 {
		t.Errorf("len(ByTest) = %d, want 1", len(report.ByTest))
	}
	if len(report.Survivors) != 1 {
		t.Errorf("len(Survivors) = %d, want 1", len(report.Survivors))
	}
}

func TestGenerationSummary_Fields(t *testing.T) {
	summary := &GenerationSummary{
		Version:     "1.0",
		WorkspaceID: "ws-123",
		Repository:  "https://github.com/test/repo",
		Branch:      "main",
		CommitSHA:   "abc123",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Duration:    "5m30s",
		Results: GenerationResults{
			TotalTargets: 100,
			Completed:    90,
			Failed:       5,
			Skipped:      5,
			TestsWritten: 85,
			Commits:      10,
		},
		Artifacts: []string{"test-plan.json", "coverage.json"},
	}

	if summary.WorkspaceID != "ws-123" {
		t.Errorf("WorkspaceID = %s, want ws-123", summary.WorkspaceID)
	}
	if summary.Results.TestsWritten != 85 {
		t.Errorf("TestsWritten = %d, want 85", summary.Results.TestsWritten)
	}
	if len(summary.Artifacts) != 2 {
		t.Errorf("len(Artifacts) = %d, want 2", len(summary.Artifacts))
	}
}

func TestEstimateComplexity(t *testing.T) {
	tests := []struct {
		name       string
		targetType string
		want       string
	}{
		{"method", "method", "medium"},
		{"function", "function", "low"},
		{"class", "class", "low"},
		{"empty", "", "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &TargetState{Type: tt.targetType}
			got := estimateComplexity(target)
			if got != tt.want {
				t.Errorf("estimateComplexity() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDetectCoverageTool(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{"go", "go cover"},
		{"python", "coverage.py"},
		{"javascript", "jest --coverage"},
		{"typescript", "jest --coverage"},
		{"rust", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			got := detectCoverageTool(tt.language)
			if got != tt.want {
				t.Errorf("detectCoverageTool(%s) = %s, want %s", tt.language, got, tt.want)
			}
		})
	}
}

func TestArtifactManager_ListArtifacts_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{path: tmpDir}
	am := NewArtifactManager(ws)

	artifacts := am.ListArtifacts()

	if len(artifacts) != 0 {
		t.Errorf("len(artifacts) = %d, want 0", len(artifacts))
	}
}

func TestArtifactManager_ListArtifacts_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	artifactDir := filepath.Join(tmpDir, "artifacts")
	os.MkdirAll(artifactDir, 0755)

	// Create some artifact files
	os.WriteFile(filepath.Join(artifactDir, "test-plan.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(artifactDir, "coverage.json"), []byte("{}"), 0644)

	ws := &Workspace{path: tmpDir}
	am := NewArtifactManager(ws)

	artifacts := am.ListArtifacts()

	if len(artifacts) != 2 {
		t.Errorf("len(artifacts) = %d, want 2", len(artifacts))
	}
}
