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

// ===== Artifact Generation Tests =====

func TestArtifactManager_GenerateTestPlan(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{
		path:      tmpDir,
		RepoURL:   "https://github.com/test/repo",
		CommitSHA: "abc123",
		State: &WorkspaceState{
			TotalTargets: 3,
			Targets: map[string]*TargetState{
				"1": {ID: "1", Name: "Func1", File: "main.go", Line: 10, Type: "function"},
				"2": {ID: "2", Name: "Func2", File: "main.go", Line: 20, Type: "function"},
				"3": {ID: "3", Name: "Method1", File: "util.go", Line: 5, Type: "method"},
			},
		},
	}
	am := NewArtifactManager(ws)

	plan, err := am.GenerateTestPlan()
	if err != nil {
		t.Fatalf("GenerateTestPlan() error: %v", err)
	}

	if plan == nil {
		t.Fatal("GenerateTestPlan() returned nil")
	}
	if plan.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", plan.Version)
	}
	if plan.Repo != "https://github.com/test/repo" {
		t.Errorf("Repo = %s, want https://github.com/test/repo", plan.Repo)
	}
	if plan.CommitSHA != "abc123" {
		t.Errorf("CommitSHA = %s, want abc123", plan.CommitSHA)
	}
	if plan.Summary.TotalTargets != 3 {
		t.Errorf("TotalTargets = %d, want 3", plan.Summary.TotalTargets)
	}
	if len(plan.Targets) != 3 {
		t.Errorf("len(Targets) = %d, want 3", len(plan.Targets))
	}

	// Check by type counts
	if plan.Summary.ByType["function"] != 2 {
		t.Errorf("ByType[function] = %d, want 2", plan.Summary.ByType["function"])
	}
	if plan.Summary.ByType["method"] != 1 {
		t.Errorf("ByType[method] = %d, want 1", plan.Summary.ByType["method"])
	}

	// Check by file counts
	if plan.Summary.ByFile["main.go"] != 2 {
		t.Errorf("ByFile[main.go] = %d, want 2", plan.Summary.ByFile["main.go"])
	}
	if plan.Summary.ByFile["util.go"] != 1 {
		t.Errorf("ByFile[util.go] = %d, want 1", plan.Summary.ByFile["util.go"])
	}

	// Verify file was created
	artifactPath := filepath.Join(tmpDir, "artifacts", "test-plan.json")
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Error("test-plan.json was not created")
	}
}

func TestArtifactManager_GenerateTestPlan_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{
		path: tmpDir,
		State: &WorkspaceState{
			TotalTargets: 0,
			Targets:      map[string]*TargetState{},
		},
	}
	am := NewArtifactManager(ws)

	plan, err := am.GenerateTestPlan()
	if err != nil {
		t.Fatalf("GenerateTestPlan() error: %v", err)
	}

	if len(plan.Targets) != 0 {
		t.Errorf("len(Targets) = %d, want 0", len(plan.Targets))
	}
	if plan.Summary.EstimatedTimeMinutes != 0 {
		t.Errorf("EstimatedTimeMinutes = %d, want 0", plan.Summary.EstimatedTimeMinutes)
	}
}

func TestArtifactManager_GenerateExecutionReport(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{path: tmpDir}
	am := NewArtifactManager(ws)

	results := []TestResult{
		{ID: "1", Name: "Test1", Status: "passed", DurationMs: 100},
		{ID: "2", Name: "Test2", Status: "passed", DurationMs: 150},
		{ID: "3", Name: "Test3", Status: "failed", DurationMs: 200, Error: "assertion failed"},
		{ID: "4", Name: "Test4", Status: "skipped", DurationMs: 0},
	}
	duration := time.Minute * 5

	report, err := am.GenerateExecutionReport(results, duration)
	if err != nil {
		t.Fatalf("GenerateExecutionReport() error: %v", err)
	}

	if report == nil {
		t.Fatal("GenerateExecutionReport() returned nil")
	}
	if report.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", report.Version)
	}
	if report.DurationSeconds != 300 { // 5 minutes
		t.Errorf("DurationSeconds = %d, want 300", report.DurationSeconds)
	}
	if report.Summary.Total != 4 {
		t.Errorf("Total = %d, want 4", report.Summary.Total)
	}
	if report.Summary.Passed != 2 {
		t.Errorf("Passed = %d, want 2", report.Summary.Passed)
	}
	if report.Summary.Failed != 1 {
		t.Errorf("Failed = %d, want 1", report.Summary.Failed)
	}
	if report.Summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", report.Summary.Skipped)
	}
	if report.Summary.PassRate != 50.0 { // 2/4 * 100
		t.Errorf("PassRate = %f, want 50.0", report.Summary.PassRate)
	}

	// Verify file was created
	artifactPath := filepath.Join(tmpDir, "artifacts", "execution.json")
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Error("execution.json was not created")
	}
}

func TestArtifactManager_GenerateExecutionReport_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{path: tmpDir}
	am := NewArtifactManager(ws)

	report, err := am.GenerateExecutionReport([]TestResult{}, time.Second)
	if err != nil {
		t.Fatalf("GenerateExecutionReport() error: %v", err)
	}

	if report.Summary.Total != 0 {
		t.Errorf("Total = %d, want 0", report.Summary.Total)
	}
	if report.Summary.PassRate != 0 {
		t.Errorf("PassRate = %f, want 0 for empty results", report.Summary.PassRate)
	}
}

func TestArtifactManager_GenerateCoverageReport(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{
		path:     tmpDir,
		Language: "go",
	}
	am := NewArtifactManager(ws)

	files := []FileCoverage{
		{Path: "main.go", TotalLines: 100, CoveredLines: 80, CoveragePercent: 80.0, UncoveredLines: []int{10, 20}},
		{Path: "util.go", TotalLines: 50, CoveredLines: 40, CoveragePercent: 80.0, UncoveredLines: []int{5}},
	}

	report, err := am.GenerateCoverageReport(files)
	if err != nil {
		t.Fatalf("GenerateCoverageReport() error: %v", err)
	}

	if report == nil {
		t.Fatal("GenerateCoverageReport() returned nil")
	}
	if report.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", report.Version)
	}
	if report.Tool != "go cover" {
		t.Errorf("Tool = %s, want go cover", report.Tool)
	}
	if report.Summary.TotalLines != 150 { // 100 + 50
		t.Errorf("TotalLines = %d, want 150", report.Summary.TotalLines)
	}
	if report.Summary.CoveredLines != 120 { // 80 + 40
		t.Errorf("CoveredLines = %d, want 120", report.Summary.CoveredLines)
	}
	if report.Summary.CoveragePercent != 80.0 { // 120/150 * 100
		t.Errorf("CoveragePercent = %f, want 80.0", report.Summary.CoveragePercent)
	}
	if len(report.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(report.Files))
	}

	// Verify file was created
	artifactPath := filepath.Join(tmpDir, "artifacts", "coverage.json")
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Error("coverage.json was not created")
	}
}

func TestArtifactManager_GenerateCoverageReport_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{path: tmpDir, Language: "python"}
	am := NewArtifactManager(ws)

	report, err := am.GenerateCoverageReport([]FileCoverage{})
	if err != nil {
		t.Fatalf("GenerateCoverageReport() error: %v", err)
	}

	if report.Summary.TotalLines != 0 {
		t.Errorf("TotalLines = %d, want 0", report.Summary.TotalLines)
	}
	if report.Summary.CoveragePercent != 0 {
		t.Errorf("CoveragePercent = %f, want 0 for empty files", report.Summary.CoveragePercent)
	}
	if report.Tool != "coverage.py" {
		t.Errorf("Tool = %s, want coverage.py for python", report.Tool)
	}
}

func TestArtifactManager_GenerateMutationReport(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{path: tmpDir}
	am := NewArtifactManager(ws)

	summary := MutationSummary{
		TotalMutants:  100,
		Killed:        85,
		Survived:      10,
		Timeout:       5,
		MutationScore: 85.0,
	}
	byTest := []TestMutations{
		{TestID: "test1", MutantsTested: 50, Killed: 45, Score: 90.0},
		{TestID: "test2", MutantsTested: 50, Killed: 40, Score: 80.0},
	}
	survivors := []SurvivedMutant{
		{ID: "mut1", Operator: "MATH", Location: "main.go:10", Original: "+", Mutated: "-"},
	}
	duration := time.Minute * 10

	report, err := am.GenerateMutationReport(summary, byTest, survivors, duration)
	if err != nil {
		t.Fatalf("GenerateMutationReport() error: %v", err)
	}

	if report == nil {
		t.Fatal("GenerateMutationReport() returned nil")
	}
	if report.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", report.Version)
	}
	if report.DurationSeconds != 600 { // 10 minutes
		t.Errorf("DurationSeconds = %d, want 600", report.DurationSeconds)
	}
	if report.Summary.MutationScore != 85.0 {
		t.Errorf("MutationScore = %f, want 85.0", report.Summary.MutationScore)
	}
	if len(report.ByTest) != 2 {
		t.Errorf("len(ByTest) = %d, want 2", len(report.ByTest))
	}
	if len(report.Survivors) != 1 {
		t.Errorf("len(Survivors) = %d, want 1", len(report.Survivors))
	}

	// Verify file was created
	artifactPath := filepath.Join(tmpDir, "artifacts", "mutation.json")
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Error("mutation.json was not created")
	}
}

func TestArtifactManager_GenerateSummary(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{
		ID:        "ws-123",
		Name:      "test-repo",
		path:      tmpDir,
		RepoURL:   "https://github.com/test/repo",
		Branch:    "main",
		CommitSHA: "abc123",
		State: &WorkspaceState{
			TotalTargets: 10,
			Completed:    8,
			Failed:       1,
			Skipped:      1,
			Targets: map[string]*TargetState{
				"1": {ID: "1", TestFile: "test1.go", CommitSHA: "commit1"},
				"2": {ID: "2", TestFile: "test2.go", CommitSHA: "commit2"},
				"3": {ID: "3", TestFile: "", CommitSHA: ""}, // No test written
			},
		},
	}

	// Create an artifact file so ListArtifacts returns something
	os.MkdirAll(filepath.Join(tmpDir, "artifacts"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "artifacts", "test-plan.json"), []byte("{}"), 0644)

	am := NewArtifactManager(ws)
	startTime := time.Now().Add(-time.Minute * 5) // Started 5 minutes ago

	summary, err := am.GenerateSummary(startTime)
	if err != nil {
		t.Fatalf("GenerateSummary() error: %v", err)
	}

	if summary == nil {
		t.Fatal("GenerateSummary() returned nil")
	}
	if summary.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", summary.Version)
	}
	if summary.WorkspaceID != "ws-123" {
		t.Errorf("WorkspaceID = %s, want ws-123", summary.WorkspaceID)
	}
	if summary.Repository != "https://github.com/test/repo" {
		t.Errorf("Repository mismatch")
	}
	if summary.Branch != "main" {
		t.Errorf("Branch = %s, want main", summary.Branch)
	}
	if summary.Results.TotalTargets != 10 {
		t.Errorf("TotalTargets = %d, want 10", summary.Results.TotalTargets)
	}
	if summary.Results.Completed != 8 {
		t.Errorf("Completed = %d, want 8", summary.Results.Completed)
	}
	if summary.Results.TestsWritten != 2 {
		t.Errorf("TestsWritten = %d, want 2", summary.Results.TestsWritten)
	}
	if summary.Results.Commits != 2 {
		t.Errorf("Commits = %d, want 2", summary.Results.Commits)
	}
	if len(summary.Artifacts) < 1 {
		t.Error("Should list at least 1 artifact")
	}

	// Verify file was created
	artifactPath := filepath.Join(tmpDir, "artifacts", "summary.json")
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Error("summary.json was not created")
	}
}

func TestArtifactManager_LoadArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	artifactDir := filepath.Join(tmpDir, "artifacts")
	os.MkdirAll(artifactDir, 0755)

	// Create a test artifact
	testData := `{"version": "1.0", "tool": "test"}`
	os.WriteFile(filepath.Join(artifactDir, "test.json"), []byte(testData), 0644)

	ws := &Workspace{path: tmpDir}
	am := NewArtifactManager(ws)

	var loaded struct {
		Version string `json:"version"`
		Tool    string `json:"tool"`
	}

	err := am.LoadArtifact("test.json", &loaded)
	if err != nil {
		t.Fatalf("LoadArtifact() error: %v", err)
	}

	if loaded.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", loaded.Version)
	}
	if loaded.Tool != "test" {
		t.Errorf("Tool = %s, want test", loaded.Tool)
	}
}

func TestArtifactManager_LoadArtifact_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{path: tmpDir}
	am := NewArtifactManager(ws)

	var loaded interface{}
	err := am.LoadArtifact("nonexistent.json", &loaded)
	if err == nil {
		t.Error("Expected error for nonexistent artifact")
	}
}

func TestArtifactManager_LoadArtifact_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	artifactDir := filepath.Join(tmpDir, "artifacts")
	os.MkdirAll(artifactDir, 0755)
	os.WriteFile(filepath.Join(artifactDir, "invalid.json"), []byte("not json"), 0644)

	ws := &Workspace{path: tmpDir}
	am := NewArtifactManager(ws)

	var loaded map[string]interface{}
	err := am.LoadArtifact("invalid.json", &loaded)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestArtifactManager_SaveArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{path: tmpDir}
	am := NewArtifactManager(ws)

	testData := map[string]interface{}{
		"version": "1.0",
		"data":    []int{1, 2, 3},
	}

	err := am.saveArtifact("custom.json", testData)
	if err != nil {
		t.Fatalf("saveArtifact() error: %v", err)
	}

	// Verify file was created
	artifactPath := filepath.Join(tmpDir, "artifacts", "custom.json")
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Error("custom.json was not created")
	}

	// Load and verify
	var loaded map[string]interface{}
	err = am.LoadArtifact("custom.json", &loaded)
	if err != nil {
		t.Fatalf("LoadArtifact() error: %v", err)
	}

	if loaded["version"] != "1.0" {
		t.Errorf("version = %v, want 1.0", loaded["version"])
	}
}

func TestTestPlanSummary_EstimatedTime(t *testing.T) {
	tmpDir := t.TempDir()
	targets := make(map[string]*TargetState)
	for i := 0; i < 30; i++ {
		id := string(rune('a' + i))
		targets[id] = &TargetState{
			ID:   id,
			Name: "Func",
			Type: "function",
		}
	}

	ws := &Workspace{
		path: tmpDir,
		State: &WorkspaceState{
			TotalTargets: 30,
			Targets:      targets,
		},
	}
	am := NewArtifactManager(ws)

	plan, err := am.GenerateTestPlan()
	if err != nil {
		t.Fatalf("GenerateTestPlan() error: %v", err)
	}

	// 30 targets * 20 seconds / 60 = 10 minutes
	expectedTime := 10
	if plan.Summary.EstimatedTimeMinutes != expectedTime {
		t.Errorf("EstimatedTimeMinutes = %d, want %d", plan.Summary.EstimatedTimeMinutes, expectedTime)
	}
}
