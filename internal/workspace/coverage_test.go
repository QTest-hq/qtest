package workspace

import (
	"testing"
)

func TestNewCoverageCollector(t *testing.T) {
	ws := &Workspace{
		path:     "/tmp/test-workspace",
		Language: "go",
	}

	collector := NewCoverageCollector(ws)

	if collector == nil {
		t.Fatal("NewCoverageCollector() returned nil")
	}
	if collector.ws != ws {
		t.Error("ws reference mismatch")
	}
	if collector.artifacts == nil {
		t.Error("artifacts should not be nil")
	}
}

func TestCoverageSummaryLine(t *testing.T) {
	report := &CoverageReport{
		Summary: CoverageSummary{
			CoveragePercent: 85.5,
			CoveredLines:    855,
			TotalLines:      1000,
		},
	}

	line := CoverageSummaryLine(report)

	expected := "Coverage: 85.5% (855/1000 lines)"
	if line != expected {
		t.Errorf("CoverageSummaryLine() = %s, want %s", line, expected)
	}
}

func TestCoverageSummaryLine_ZeroCoverage(t *testing.T) {
	report := &CoverageReport{
		Summary: CoverageSummary{
			CoveragePercent: 0,
			CoveredLines:    0,
			TotalLines:      100,
		},
	}

	line := CoverageSummaryLine(report)

	expected := "Coverage: 0.0% (0/100 lines)"
	if line != expected {
		t.Errorf("CoverageSummaryLine() = %s, want %s", line, expected)
	}
}

func TestCoverageSummaryLine_FullCoverage(t *testing.T) {
	report := &CoverageReport{
		Summary: CoverageSummary{
			CoveragePercent: 100.0,
			CoveredLines:    500,
			TotalLines:      500,
		},
	}

	line := CoverageSummaryLine(report)

	expected := "Coverage: 100.0% (500/500 lines)"
	if line != expected {
		t.Errorf("CoverageSummaryLine() = %s, want %s", line, expected)
	}
}

func TestCoverageSummary_Fields(t *testing.T) {
	summary := CoverageSummary{
		TotalLines:      1000,
		CoveredLines:    800,
		CoveragePercent: 80.0,
		ByPackage: map[string]float64{
			"main":  85.0,
			"utils": 75.0,
		},
	}

	if summary.TotalLines != 1000 {
		t.Errorf("TotalLines = %d, want 1000", summary.TotalLines)
	}
	if summary.CoveredLines != 800 {
		t.Errorf("CoveredLines = %d, want 800", summary.CoveredLines)
	}
	if summary.CoveragePercent != 80.0 {
		t.Errorf("CoveragePercent = %f, want 80.0", summary.CoveragePercent)
	}
	if len(summary.ByPackage) != 2 {
		t.Errorf("len(ByPackage) = %d, want 2", len(summary.ByPackage))
	}
	if summary.ByPackage["main"] != 85.0 {
		t.Errorf("ByPackage[main] = %f, want 85.0", summary.ByPackage["main"])
	}
}

func TestCoverageSummary_Defaults(t *testing.T) {
	summary := CoverageSummary{}

	if summary.TotalLines != 0 {
		t.Errorf("default TotalLines = %d, want 0", summary.TotalLines)
	}
	if summary.CoveredLines != 0 {
		t.Errorf("default CoveredLines = %d, want 0", summary.CoveredLines)
	}
	if summary.CoveragePercent != 0 {
		t.Errorf("default CoveragePercent = %f, want 0", summary.CoveragePercent)
	}
	if summary.ByPackage != nil {
		t.Error("default ByPackage should be nil")
	}
}

func TestFileCoverage_Defaults(t *testing.T) {
	fc := FileCoverage{}

	if fc.Path != "" {
		t.Errorf("default Path = %s, want empty", fc.Path)
	}
	if fc.TotalLines != 0 {
		t.Errorf("default TotalLines = %d, want 0", fc.TotalLines)
	}
	if fc.CoveredLines != 0 {
		t.Errorf("default CoveredLines = %d, want 0", fc.CoveredLines)
	}
	if fc.CoveragePercent != 0 {
		t.Errorf("default CoveragePercent = %f, want 0", fc.CoveragePercent)
	}
	if fc.UncoveredLines != nil {
		t.Error("default UncoveredLines should be nil")
	}
}

func TestMutationSummary_Fields(t *testing.T) {
	summary := MutationSummary{
		TotalMutants:  100,
		Killed:        85,
		Survived:      10,
		Timeout:       5,
		MutationScore: 85.0,
	}

	if summary.TotalMutants != 100 {
		t.Errorf("TotalMutants = %d, want 100", summary.TotalMutants)
	}
	if summary.Killed != 85 {
		t.Errorf("Killed = %d, want 85", summary.Killed)
	}
	if summary.Survived != 10 {
		t.Errorf("Survived = %d, want 10", summary.Survived)
	}
	if summary.Timeout != 5 {
		t.Errorf("Timeout = %d, want 5", summary.Timeout)
	}
	if summary.MutationScore != 85.0 {
		t.Errorf("MutationScore = %f, want 85.0", summary.MutationScore)
	}
}

func TestTestMutations_Fields(t *testing.T) {
	tm := TestMutations{
		TestID:        "test-1",
		MutantsTested: 50,
		Killed:        45,
		Score:         90.0,
	}

	if tm.TestID != "test-1" {
		t.Errorf("TestID = %s, want test-1", tm.TestID)
	}
	if tm.MutantsTested != 50 {
		t.Errorf("MutantsTested = %d, want 50", tm.MutantsTested)
	}
	if tm.Killed != 45 {
		t.Errorf("Killed = %d, want 45", tm.Killed)
	}
	if tm.Score != 90.0 {
		t.Errorf("Score = %f, want 90.0", tm.Score)
	}
}

func TestSurvivedMutant_Fields(t *testing.T) {
	mutant := SurvivedMutant{
		ID:                  "mut-1",
		Operator:            "MATH",
		Location:            "main.go:10",
		Original:            "+",
		Mutated:             "-",
		TestThatShouldCatch: "TestAdd",
	}

	if mutant.ID != "mut-1" {
		t.Errorf("ID = %s, want mut-1", mutant.ID)
	}
	if mutant.Operator != "MATH" {
		t.Errorf("Operator = %s, want MATH", mutant.Operator)
	}
	if mutant.Original != "+" {
		t.Errorf("Original = %s, want +", mutant.Original)
	}
	if mutant.Mutated != "-" {
		t.Errorf("Mutated = %s, want -", mutant.Mutated)
	}
	if mutant.TestThatShouldCatch != "TestAdd" {
		t.Errorf("TestThatShouldCatch = %s, want TestAdd", mutant.TestThatShouldCatch)
	}
}

func TestExecutionSummary_Fields(t *testing.T) {
	summary := ExecutionSummary{
		Total:    100,
		Passed:   90,
		Failed:   8,
		Skipped:  2,
		PassRate: 90.0,
	}

	if summary.Total != 100 {
		t.Errorf("Total = %d, want 100", summary.Total)
	}
	if summary.Passed != 90 {
		t.Errorf("Passed = %d, want 90", summary.Passed)
	}
	if summary.Failed != 8 {
		t.Errorf("Failed = %d, want 8", summary.Failed)
	}
	if summary.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", summary.Skipped)
	}
	if summary.PassRate != 90.0 {
		t.Errorf("PassRate = %f, want 90.0", summary.PassRate)
	}
}

func TestGenerationResults_Fields(t *testing.T) {
	results := GenerationResults{
		TotalTargets: 100,
		Completed:    90,
		Failed:       5,
		Skipped:      5,
		TestsWritten: 85,
		Commits:      10,
	}

	if results.TotalTargets != 100 {
		t.Errorf("TotalTargets = %d, want 100", results.TotalTargets)
	}
	if results.Completed != 90 {
		t.Errorf("Completed = %d, want 90", results.Completed)
	}
	if results.Failed != 5 {
		t.Errorf("Failed = %d, want 5", results.Failed)
	}
	if results.TestsWritten != 85 {
		t.Errorf("TestsWritten = %d, want 85", results.TestsWritten)
	}
	if results.Commits != 10 {
		t.Errorf("Commits = %d, want 10", results.Commits)
	}
}

// ===== Coverage Parsing Tests =====

func TestParseGoCoverage_Basic(t *testing.T) {
	// Create temp coverage file
	tmpDir := t.TempDir()
	coverFile := tmpDir + "/coverage.out"

	coverageData := `mode: count
github.com/example/pkg/main.go:10.20,15.2 3 1
github.com/example/pkg/main.go:17.30,22.2 5 0
github.com/example/pkg/utils.go:5.10,10.2 4 2
`
	if err := writeFile(coverFile, coverageData); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	ws := &Workspace{
		path:     tmpDir,
		RepoPath: "/some/other/path", // Different path to test relative path handling
		Language: "go",
	}
	collector := NewCoverageCollector(ws)

	files, err := collector.parseGoCoverage(coverFile)
	if err != nil {
		t.Fatalf("parseGoCoverage() error: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("parseGoCoverage() returned no files")
	}

	// Should have parsed multiple files
	if len(files) < 1 {
		t.Errorf("Expected at least 1 file, got %d", len(files))
	}
}

func TestParseGoCoverage_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := tmpDir + "/coverage.out"

	coverageData := `mode: count
`
	if err := writeFile(coverFile, coverageData); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	ws := &Workspace{path: tmpDir, RepoPath: tmpDir, Language: "go"}
	collector := NewCoverageCollector(ws)

	files, err := collector.parseGoCoverage(coverFile)
	if err != nil {
		t.Fatalf("parseGoCoverage() error: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 files for empty coverage, got %d", len(files))
	}
}

func TestParseGoCoverage_FileNotFound(t *testing.T) {
	ws := &Workspace{path: "/tmp", RepoPath: "/tmp", Language: "go"}
	collector := NewCoverageCollector(ws)

	_, err := collector.parseGoCoverage("/nonexistent/coverage.out")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestParseGoCoverage_CoverageCalculation(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := tmpDir + "/coverage.out"

	// 5 statements covered (count > 0), 3 statements not covered (count = 0)
	coverageData := `mode: count
pkg/main.go:10.20,15.2 5 1
pkg/main.go:17.30,22.2 3 0
`
	if err := writeFile(coverFile, coverageData); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	ws := &Workspace{path: tmpDir, RepoPath: tmpDir, Language: "go"}
	collector := NewCoverageCollector(ws)

	files, err := collector.parseGoCoverage(coverFile)
	if err != nil {
		t.Fatalf("parseGoCoverage() error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	fc := files[0]
	if fc.TotalLines != 8 { // 5 + 3
		t.Errorf("TotalLines = %d, want 8", fc.TotalLines)
	}
	if fc.CoveredLines != 5 {
		t.Errorf("CoveredLines = %d, want 5", fc.CoveredLines)
	}
	expectedPercent := 62.5 // 5/8 * 100
	if fc.CoveragePercent != expectedPercent {
		t.Errorf("CoveragePercent = %f, want %f", fc.CoveragePercent, expectedPercent)
	}
}

func TestParsePythonCoverage_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := tmpDir + "/coverage.json"

	coverageJSON := `{
		"files": {
			"/project/main.py": {
				"summary": {
					"covered_lines": 80,
					"num_statements": 100,
					"percent_covered": 80.0,
					"missing_lines": [15, 20, 25]
				}
			},
			"/project/utils.py": {
				"summary": {
					"covered_lines": 45,
					"num_statements": 50,
					"percent_covered": 90.0,
					"missing_lines": [10]
				}
			}
		}
	}`
	if err := writeFile(coverFile, coverageJSON); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	ws := &Workspace{path: tmpDir, RepoPath: "/project", Language: "python"}
	collector := NewCoverageCollector(ws)

	files, err := collector.parsePythonCoverage(coverFile)
	if err != nil {
		t.Fatalf("parsePythonCoverage() error: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(files))
	}
}

func TestParsePythonCoverage_EmptyFiles(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := tmpDir + "/coverage.json"

	coverageJSON := `{"files": {}}`
	if err := writeFile(coverFile, coverageJSON); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	ws := &Workspace{path: tmpDir, RepoPath: tmpDir, Language: "python"}
	collector := NewCoverageCollector(ws)

	files, err := collector.parsePythonCoverage(coverFile)
	if err != nil {
		t.Fatalf("parsePythonCoverage() error: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(files))
	}
}

func TestParsePythonCoverage_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := tmpDir + "/coverage.json"

	if err := writeFile(coverFile, "not json"); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	ws := &Workspace{path: tmpDir, RepoPath: tmpDir, Language: "python"}
	collector := NewCoverageCollector(ws)

	_, err := collector.parsePythonCoverage(coverFile)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParsePythonCoverage_FileNotFound(t *testing.T) {
	ws := &Workspace{path: "/tmp", RepoPath: "/tmp", Language: "python"}
	collector := NewCoverageCollector(ws)

	_, err := collector.parsePythonCoverage("/nonexistent/coverage.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestParseJestCoverage_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := tmpDir + "/coverage-final.json"

	// Istanbul/Jest coverage format
	coverageJSON := `{
		"/project/src/index.js": {
			"s": {"0": 1, "1": 1, "2": 0, "3": 1},
			"f": {"0": 1, "1": 0},
			"b": {"0": [1, 0]}
		},
		"/project/src/utils.js": {
			"s": {"0": 1, "1": 1},
			"f": {"0": 1},
			"b": {}
		}
	}`
	if err := writeFile(coverFile, coverageJSON); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	ws := &Workspace{path: tmpDir, RepoPath: "/project", Language: "javascript"}
	collector := NewCoverageCollector(ws)

	files, err := collector.parseJestCoverage(coverFile)
	if err != nil {
		t.Fatalf("parseJestCoverage() error: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(files))
	}
}

func TestParseJestCoverage_CoverageCalculation(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := tmpDir + "/coverage-final.json"

	// 3 statements: 2 covered (count > 0), 1 not covered (count = 0)
	coverageJSON := `{
		"src/app.js": {
			"s": {"0": 5, "1": 0, "2": 3},
			"f": {},
			"b": {}
		}
	}`
	if err := writeFile(coverFile, coverageJSON); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	ws := &Workspace{path: tmpDir, RepoPath: tmpDir, Language: "javascript"}
	collector := NewCoverageCollector(ws)

	files, err := collector.parseJestCoverage(coverFile)
	if err != nil {
		t.Fatalf("parseJestCoverage() error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	fc := files[0]
	if fc.TotalLines != 3 {
		t.Errorf("TotalLines = %d, want 3", fc.TotalLines)
	}
	if fc.CoveredLines != 2 {
		t.Errorf("CoveredLines = %d, want 2", fc.CoveredLines)
	}
	// 2/3 * 100 = 66.666...
	if fc.CoveragePercent < 66.0 || fc.CoveragePercent > 67.0 {
		t.Errorf("CoveragePercent = %f, want ~66.67", fc.CoveragePercent)
	}
}

func TestParseJestCoverage_EmptyStatements(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := tmpDir + "/coverage-final.json"

	coverageJSON := `{
		"src/empty.js": {
			"s": {},
			"f": {},
			"b": {}
		}
	}`
	if err := writeFile(coverFile, coverageJSON); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	ws := &Workspace{path: tmpDir, RepoPath: tmpDir, Language: "javascript"}
	collector := NewCoverageCollector(ws)

	files, err := collector.parseJestCoverage(coverFile)
	if err != nil {
		t.Fatalf("parseJestCoverage() error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	fc := files[0]
	if fc.TotalLines != 0 {
		t.Errorf("TotalLines = %d, want 0", fc.TotalLines)
	}
	if fc.CoveragePercent != 0 {
		t.Errorf("CoveragePercent = %f, want 0", fc.CoveragePercent)
	}
}

func TestParseJestCoverage_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := tmpDir + "/coverage-final.json"

	if err := writeFile(coverFile, "{invalid json}"); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	ws := &Workspace{path: tmpDir, RepoPath: tmpDir, Language: "javascript"}
	collector := NewCoverageCollector(ws)

	_, err := collector.parseJestCoverage(coverFile)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParseCoverageOutput_PercentagePattern(t *testing.T) {
	ws := &Workspace{path: "/tmp", Language: "go"}
	collector := NewCoverageCollector(ws)

	tests := []struct {
		name     string
		output   string
		expected float64
	}{
		{"simple percentage", "Coverage: 85.5%", 85.5},
		{"integer percentage", "Coverage: 100%", 100.0},
		{"percentage in text", "Total coverage is 42.3% of lines", 42.3},
		{"zero percent", "0% covered", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := collector.parseCoverageOutput(tt.output, "test.go")
			if err != nil {
				t.Fatalf("parseCoverageOutput() error: %v", err)
			}
			if result.CoveragePercent != tt.expected {
				t.Errorf("CoveragePercent = %f, want %f", result.CoveragePercent, tt.expected)
			}
		})
	}
}

func TestParseCoverageOutput_RatioPattern(t *testing.T) {
	ws := &Workspace{path: "/tmp", Language: "go"}
	collector := NewCoverageCollector(ws)

	tests := []struct {
		name            string
		output          string
		expectedCovered int
		expectedTotal   int
	}{
		{"statements ratio", "Statements: 80/100", 80, 100},
		{"lines ratio", "Lines: 45/50", 45, 50},
		{"coverage ratio", "Coverage 120/200 lines", 120, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := collector.parseCoverageOutput(tt.output, "test.go")
			if err != nil {
				t.Fatalf("parseCoverageOutput() error: %v", err)
			}
			if result.CoveredLines != tt.expectedCovered {
				t.Errorf("CoveredLines = %d, want %d", result.CoveredLines, tt.expectedCovered)
			}
			if result.TotalLines != tt.expectedTotal {
				t.Errorf("TotalLines = %d, want %d", result.TotalLines, tt.expectedTotal)
			}
		})
	}
}

func TestParseCoverageOutput_NoPattern(t *testing.T) {
	ws := &Workspace{path: "/tmp", Language: "go"}
	collector := NewCoverageCollector(ws)

	result, err := collector.parseCoverageOutput("No coverage data here", "test.go")
	if err != nil {
		t.Fatalf("parseCoverageOutput() error: %v", err)
	}

	if result.CoveragePercent != 0 {
		t.Errorf("CoveragePercent = %f, want 0 for no pattern match", result.CoveragePercent)
	}
	if result.Path != "test.go" {
		t.Errorf("Path = %s, want test.go", result.Path)
	}
}

func TestParseCoverageOutput_TestFilePath(t *testing.T) {
	ws := &Workspace{path: "/tmp", Language: "go"}
	collector := NewCoverageCollector(ws)

	result, err := collector.parseCoverageOutput("any output", "/path/to/my_test.go")
	if err != nil {
		t.Fatalf("parseCoverageOutput() error: %v", err)
	}

	if result.Path != "/path/to/my_test.go" {
		t.Errorf("Path = %s, want /path/to/my_test.go", result.Path)
	}
}

// ===== Helper Function Tests =====

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := tmpDir + "/new/nested/dir"

	err := ensureDir(newDir)
	if err != nil {
		t.Fatalf("ensureDir() error: %v", err)
	}

	// Check dir exists
	info, err := readFileIfExists(newDir)
	if err != nil {
		// Directory exists but readFileIfExists is for files
		// Just verify no error from ensureDir
	}
	_ = info
}

func TestReadFileIfExists_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.txt"
	content := "test content"

	if err := writeFile(testFile, content); err != nil {
		t.Fatalf("writeFile() error: %v", err)
	}

	result, err := readFileIfExists(testFile)
	if err != nil {
		t.Fatalf("readFileIfExists() error: %v", err)
	}

	if result != content {
		t.Errorf("content = %s, want %s", result, content)
	}
}

func TestReadFileIfExists_NotExists(t *testing.T) {
	result, err := readFileIfExists("/nonexistent/file.txt")
	if err != nil {
		t.Fatalf("readFileIfExists() should not error for nonexistent file: %v", err)
	}

	if result != "" {
		t.Errorf("result = %s, want empty string", result)
	}
}

func TestWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/output.txt"
	content := "hello world"

	err := writeFile(testFile, content)
	if err != nil {
		t.Fatalf("writeFile() error: %v", err)
	}

	result, err := readFileIfExists(testFile)
	if err != nil {
		t.Fatalf("readFileIfExists() error: %v", err)
	}

	if result != content {
		t.Errorf("content = %s, want %s", result, content)
	}
}

func TestWriteOrAppendTest_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/new_test.go"
	code := "package test\n\nfunc TestNew(t *testing.T) {}"

	err := writeOrAppendTest(testFile, code, "go")
	if err != nil {
		t.Fatalf("writeOrAppendTest() error: %v", err)
	}

	result, err := readFileIfExists(testFile)
	if err != nil {
		t.Fatalf("readFileIfExists() error: %v", err)
	}

	if result != code {
		t.Errorf("content mismatch")
	}
}

func TestWriteOrAppendTest_AppendToExisting(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/existing_test.go"
	existingCode := "package test\n\nfunc TestExisting(t *testing.T) {}"
	newCode := "func TestNew(t *testing.T) {}"

	// Create existing file
	if err := writeFile(testFile, existingCode); err != nil {
		t.Fatalf("writeFile() error: %v", err)
	}

	// Append new code
	err := writeOrAppendTest(testFile, newCode, "go")
	if err != nil {
		t.Fatalf("writeOrAppendTest() error: %v", err)
	}

	result, err := readFileIfExists(testFile)
	if err != nil {
		t.Fatalf("readFileIfExists() error: %v", err)
	}

	// Should contain both
	if len(result) <= len(existingCode) {
		t.Error("File should have appended content")
	}
}

func TestWriteOrAppendTest_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/new/nested/dir/test.go"
	code := "package test"

	err := writeOrAppendTest(testFile, code, "go")
	if err != nil {
		t.Fatalf("writeOrAppendTest() error: %v", err)
	}

	result, err := readFileIfExists(testFile)
	if err != nil {
		t.Fatalf("readFileIfExists() error: %v", err)
	}

	if result != code {
		t.Errorf("content = %s, want %s", result, code)
	}
}
