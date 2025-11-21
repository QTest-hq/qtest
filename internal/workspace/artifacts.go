package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ArtifactManager handles artifact generation and storage
type ArtifactManager struct {
	ws          *Workspace
	artifactDir string
}

// NewArtifactManager creates an artifact manager for a workspace
func NewArtifactManager(ws *Workspace) *ArtifactManager {
	return &ArtifactManager{
		ws:          ws,
		artifactDir: filepath.Join(ws.Path(), "artifacts"),
	}
}

// Init creates the artifacts directory
func (a *ArtifactManager) Init() error {
	return os.MkdirAll(a.artifactDir, 0755)
}

// TestPlan represents the test plan artifact
type TestPlan struct {
	Version   string          `json:"version"`
	Repo      string          `json:"repository"`
	CommitSHA string          `json:"commit_sha"`
	CreatedAt time.Time       `json:"created_at"`
	Summary   TestPlanSummary `json:"summary"`
	Targets   []PlanTarget    `json:"targets"`
}

type TestPlanSummary struct {
	TotalTargets         int            `json:"total_targets"`
	ByType               map[string]int `json:"by_type"`
	ByFile               map[string]int `json:"by_file"`
	EstimatedTimeMinutes int            `json:"estimated_time_minutes"`
}

type PlanTarget struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	File               string   `json:"file"`
	Line               int      `json:"line"`
	Type               string   `json:"type"`
	Complexity         string   `json:"complexity"`
	Priority           int      `json:"priority"`
	Dependencies       []string `json:"dependencies,omitempty"`
	SuggestedTestTypes []string `json:"suggested_test_types"`
}

// GenerateTestPlan creates the test plan artifact
func (a *ArtifactManager) GenerateTestPlan() (*TestPlan, error) {
	plan := &TestPlan{
		Version:   "1.0",
		Repo:      a.ws.RepoURL,
		CommitSHA: a.ws.CommitSHA,
		CreatedAt: time.Now(),
		Summary: TestPlanSummary{
			TotalTargets: a.ws.State.TotalTargets,
			ByType:       make(map[string]int),
			ByFile:       make(map[string]int),
		},
		Targets: make([]PlanTarget, 0),
	}

	// Build plan from workspace targets
	for _, target := range a.ws.State.Targets {
		// Count by type
		plan.Summary.ByType[target.Type]++

		// Count by file
		plan.Summary.ByFile[target.File]++

		// Add target to plan
		plan.Targets = append(plan.Targets, PlanTarget{
			ID:                 target.ID,
			Name:               target.Name,
			File:               target.File,
			Line:               target.Line,
			Type:               target.Type,
			Complexity:         estimateComplexity(target),
			Priority:           1,
			SuggestedTestTypes: []string{"unit"},
		})
	}

	// Estimate time (rough: 20 seconds per target on Tier 2)
	plan.Summary.EstimatedTimeMinutes = (plan.Summary.TotalTargets * 20) / 60

	// Save artifact
	if err := a.saveArtifact("test-plan.json", plan); err != nil {
		return nil, err
	}

	return plan, nil
}

// ExecutionReport represents test execution results
type ExecutionReport struct {
	Version         string           `json:"version"`
	ExecutedAt      time.Time        `json:"executed_at"`
	DurationSeconds int              `json:"duration_seconds"`
	Summary         ExecutionSummary `json:"summary"`
	Tests           []TestResult     `json:"tests"`
}

type ExecutionSummary struct {
	Total    int     `json:"total"`
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	Skipped  int     `json:"skipped"`
	PassRate float64 `json:"pass_rate"`
}

type TestResult struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	File       string `json:"file"`
	Target     string `json:"target"`
	Status     string `json:"status"` // passed, failed, skipped
	DurationMs int    `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	StackTrace string `json:"stack_trace,omitempty"`
}

// GenerateExecutionReport creates the execution report artifact
func (a *ArtifactManager) GenerateExecutionReport(results []TestResult, duration time.Duration) (*ExecutionReport, error) {
	report := &ExecutionReport{
		Version:         "1.0",
		ExecutedAt:      time.Now(),
		DurationSeconds: int(duration.Seconds()),
		Summary: ExecutionSummary{
			Total: len(results),
		},
		Tests: results,
	}

	// Calculate summary
	for _, r := range results {
		switch r.Status {
		case "passed":
			report.Summary.Passed++
		case "failed":
			report.Summary.Failed++
		case "skipped":
			report.Summary.Skipped++
		}
	}

	if report.Summary.Total > 0 {
		report.Summary.PassRate = float64(report.Summary.Passed) / float64(report.Summary.Total) * 100
	}

	// Save artifact
	if err := a.saveArtifact("execution.json", report); err != nil {
		return nil, err
	}

	return report, nil
}

// CoverageReport represents code coverage results
type CoverageReport struct {
	Version     string          `json:"version"`
	GeneratedAt time.Time       `json:"generated_at"`
	Tool        string          `json:"tool"`
	Summary     CoverageSummary `json:"summary"`
	Files       []FileCoverage  `json:"files"`
}

type CoverageSummary struct {
	TotalLines      int                `json:"total_lines"`
	CoveredLines    int                `json:"covered_lines"`
	CoveragePercent float64            `json:"coverage_percent"`
	ByPackage       map[string]float64 `json:"by_package"`
}

type FileCoverage struct {
	Path            string  `json:"path"`
	TotalLines      int     `json:"total_lines"`
	CoveredLines    int     `json:"covered_lines"`
	CoveragePercent float64 `json:"coverage_percent"`
	UncoveredLines  []int   `json:"uncovered_lines"`
}

// GenerateCoverageReport creates the coverage report artifact
func (a *ArtifactManager) GenerateCoverageReport(files []FileCoverage) (*CoverageReport, error) {
	report := &CoverageReport{
		Version:     "1.0",
		GeneratedAt: time.Now(),
		Tool:        detectCoverageTool(a.ws.Language),
		Summary: CoverageSummary{
			ByPackage: make(map[string]float64),
		},
		Files: files,
	}

	// Calculate summary
	for _, f := range files {
		report.Summary.TotalLines += f.TotalLines
		report.Summary.CoveredLines += f.CoveredLines
	}

	if report.Summary.TotalLines > 0 {
		report.Summary.CoveragePercent = float64(report.Summary.CoveredLines) / float64(report.Summary.TotalLines) * 100
	}

	// Save artifact
	if err := a.saveArtifact("coverage.json", report); err != nil {
		return nil, err
	}

	return report, nil
}

// MutationReport represents mutation testing results
type MutationReport struct {
	Version         string           `json:"version"`
	ExecutedAt      time.Time        `json:"executed_at"`
	DurationSeconds int              `json:"duration_seconds"`
	Summary         MutationSummary  `json:"summary"`
	ByTest          []TestMutations  `json:"by_test"`
	Survivors       []SurvivedMutant `json:"survivors"`
}

type MutationSummary struct {
	TotalMutants  int     `json:"total_mutants"`
	Killed        int     `json:"killed"`
	Survived      int     `json:"survived"`
	Timeout       int     `json:"timeout"`
	MutationScore float64 `json:"mutation_score"`
}

type TestMutations struct {
	TestID        string  `json:"test_id"`
	MutantsTested int     `json:"mutants_tested"`
	Killed        int     `json:"killed"`
	Score         float64 `json:"score"`
}

type SurvivedMutant struct {
	ID                  string `json:"id"`
	Operator            string `json:"operator"`
	Location            string `json:"location"`
	Original            string `json:"original"`
	Mutated             string `json:"mutated"`
	TestThatShouldCatch string `json:"test_that_should_catch"`
}

// GenerateMutationReport creates the mutation report artifact
func (a *ArtifactManager) GenerateMutationReport(summary MutationSummary, byTest []TestMutations, survivors []SurvivedMutant, duration time.Duration) (*MutationReport, error) {
	report := &MutationReport{
		Version:         "1.0",
		ExecutedAt:      time.Now(),
		DurationSeconds: int(duration.Seconds()),
		Summary:         summary,
		ByTest:          byTest,
		Survivors:       survivors,
	}

	// Save artifact
	if err := a.saveArtifact("mutation.json", report); err != nil {
		return nil, err
	}

	return report, nil
}

// GenerationSummary creates a summary of the generation run
type GenerationSummary struct {
	Version     string            `json:"version"`
	WorkspaceID string            `json:"workspace_id"`
	Repository  string            `json:"repository"`
	Branch      string            `json:"branch"`
	CommitSHA   string            `json:"commit_sha"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
	Duration    string            `json:"duration"`
	Results     GenerationResults `json:"results"`
	Artifacts   []string          `json:"artifacts"`
}

type GenerationResults struct {
	TotalTargets int `json:"total_targets"`
	Completed    int `json:"completed"`
	Failed       int `json:"failed"`
	Skipped      int `json:"skipped"`
	TestsWritten int `json:"tests_written"`
	Commits      int `json:"commits"`
}

// GenerateSummary creates the final summary artifact
func (a *ArtifactManager) GenerateSummary(startTime time.Time) (*GenerationSummary, error) {
	summary := &GenerationSummary{
		Version:     "1.0",
		WorkspaceID: a.ws.ID,
		Repository:  a.ws.RepoURL,
		Branch:      a.ws.Branch,
		CommitSHA:   a.ws.CommitSHA,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Duration:    time.Since(startTime).Round(time.Second).String(),
		Results: GenerationResults{
			TotalTargets: a.ws.State.TotalTargets,
			Completed:    a.ws.State.Completed,
			Failed:       a.ws.State.Failed,
			Skipped:      a.ws.State.Skipped,
		},
	}

	// Count tests written and commits
	for _, target := range a.ws.State.Targets {
		if target.TestFile != "" {
			summary.Results.TestsWritten++
		}
		if target.CommitSHA != "" {
			summary.Results.Commits++
		}
	}

	// List artifacts
	summary.Artifacts = a.ListArtifacts()

	// Save artifact
	if err := a.saveArtifact("summary.json", summary); err != nil {
		return nil, err
	}

	return summary, nil
}

// ListArtifacts returns all artifact files
func (a *ArtifactManager) ListArtifacts() []string {
	artifacts := []string{}

	entries, err := os.ReadDir(a.artifactDir)
	if err != nil {
		return artifacts
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			artifacts = append(artifacts, entry.Name())
		}
	}

	return artifacts
}

// LoadArtifact loads an artifact by name
func (a *ArtifactManager) LoadArtifact(name string, v interface{}) error {
	path := filepath.Join(a.artifactDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// saveArtifact saves an artifact to disk
func (a *ArtifactManager) saveArtifact(name string, v interface{}) error {
	if err := a.Init(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal artifact: %w", err)
	}

	path := filepath.Join(a.artifactDir, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write artifact: %w", err)
	}

	return nil
}

// Helper functions

func estimateComplexity(target *TargetState) string {
	// Simple heuristic based on target type
	// Could be enhanced with actual code analysis
	switch target.Type {
	case "method":
		return "medium"
	default:
		return "low"
	}
}

func detectCoverageTool(language string) string {
	switch language {
	case "go":
		return "go cover"
	case "python":
		return "coverage.py"
	case "javascript", "typescript":
		return "jest --coverage"
	default:
		return "unknown"
	}
}
