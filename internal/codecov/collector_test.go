package codecov

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector("/tmp/test", "go")

	if c == nil {
		t.Fatal("NewCollector() returned nil")
	}
	if c.workDir != "/tmp/test" {
		t.Errorf("workDir = %s, want /tmp/test", c.workDir)
	}
	if c.language != "go" {
		t.Errorf("language = %s, want go", c.language)
	}
}

func TestCoverageReport_Fields(t *testing.T) {
	report := &CoverageReport{
		Timestamp:    time.Now(),
		Language:     "go",
		TotalLines:   1000,
		CoveredLines: 800,
		Percentage:   80.0,
		Files: []FileCoverage{
			{Path: "main.go", TotalLines: 500, CoveredLines: 400, Percentage: 80.0},
		},
		Uncovered: []UncoveredItem{
			{File: "main.go", StartLine: 10, EndLine: 20, Type: "line"},
		},
	}

	if report.Language != "go" {
		t.Errorf("Language = %s, want go", report.Language)
	}
	if report.TotalLines != 1000 {
		t.Errorf("TotalLines = %d, want 1000", report.TotalLines)
	}
	if report.CoveredLines != 800 {
		t.Errorf("CoveredLines = %d, want 800", report.CoveredLines)
	}
	if report.Percentage != 80.0 {
		t.Errorf("Percentage = %f, want 80.0", report.Percentage)
	}
	if len(report.Files) != 1 {
		t.Errorf("len(Files) = %d, want 1", len(report.Files))
	}
	if len(report.Uncovered) != 1 {
		t.Errorf("len(Uncovered) = %d, want 1", len(report.Uncovered))
	}
}

func TestFileCoverage_Fields(t *testing.T) {
	fc := FileCoverage{
		Path:           "main.go",
		TotalLines:     100,
		CoveredLines:   85,
		Percentage:     85.0,
		UncoveredLines: []int{10, 20, 30},
	}

	if fc.Path != "main.go" {
		t.Errorf("Path = %s, want main.go", fc.Path)
	}
	if fc.TotalLines != 100 {
		t.Errorf("TotalLines = %d, want 100", fc.TotalLines)
	}
	if fc.CoveredLines != 85 {
		t.Errorf("CoveredLines = %d, want 85", fc.CoveredLines)
	}
	if fc.Percentage != 85.0 {
		t.Errorf("Percentage = %f, want 85.0", fc.Percentage)
	}
	if len(fc.UncoveredLines) != 3 {
		t.Errorf("len(UncoveredLines) = %d, want 3", len(fc.UncoveredLines))
	}
}

func TestUncoveredItem_Fields(t *testing.T) {
	item := UncoveredItem{
		File:      "main.go",
		StartLine: 10,
		EndLine:   20,
		Type:      "function",
		Name:      "TestFunc",
	}

	if item.File != "main.go" {
		t.Errorf("File = %s, want main.go", item.File)
	}
	if item.StartLine != 10 {
		t.Errorf("StartLine = %d, want 10", item.StartLine)
	}
	if item.EndLine != 20 {
		t.Errorf("EndLine = %d, want 20", item.EndLine)
	}
	if item.Type != "function" {
		t.Errorf("Type = %s, want function", item.Type)
	}
	if item.Name != "TestFunc" {
		t.Errorf("Name = %s, want TestFunc", item.Name)
	}
}

func TestParseGoCoverage(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := filepath.Join(tmpDir, "coverage.out")

	// Create mock Go coverage file
	content := `mode: count
github.com/test/main.go:10.1,15.1 3 1
github.com/test/main.go:20.1,25.1 5 0
github.com/test/util.go:5.1,10.1 4 2
`
	if err := os.WriteFile(coverFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write coverage file: %v", err)
	}

	c := NewCollector(tmpDir, "go")
	report, err := c.parseGoCoverage(coverFile)
	if err != nil {
		t.Fatalf("parseGoCoverage() error = %v", err)
	}

	if report.Language != "go" {
		t.Errorf("Language = %s, want go", report.Language)
	}
	if len(report.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(report.Files))
	}

	// Check totals: 3+5+4 = 12 total, 3+4 = 7 covered
	if report.TotalLines != 12 {
		t.Errorf("TotalLines = %d, want 12", report.TotalLines)
	}
	if report.CoveredLines != 7 {
		t.Errorf("CoveredLines = %d, want 7", report.CoveredLines)
	}

	// Check uncovered lines were tracked
	if len(report.Uncovered) == 0 {
		t.Error("Uncovered should not be empty")
	}
}

func TestParseGoCoverage_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := filepath.Join(tmpDir, "coverage.out")

	// Create empty coverage file (just mode line)
	content := `mode: count
`
	if err := os.WriteFile(coverFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write coverage file: %v", err)
	}

	c := NewCollector(tmpDir, "go")
	report, err := c.parseGoCoverage(coverFile)
	if err != nil {
		t.Fatalf("parseGoCoverage() error = %v", err)
	}

	if report.TotalLines != 0 {
		t.Errorf("TotalLines = %d, want 0", report.TotalLines)
	}
	if len(report.Files) != 0 {
		t.Errorf("len(Files) = %d, want 0", len(report.Files))
	}
}

func TestParseGoCoverage_FileNotFound(t *testing.T) {
	c := NewCollector("/tmp", "go")
	_, err := c.parseGoCoverage("/nonexistent/coverage.out")
	if err == nil {
		t.Error("parseGoCoverage() should return error for nonexistent file")
	}
}

func TestParsePythonCoverage(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := filepath.Join(tmpDir, "coverage.json")

	// Create mock Python coverage JSON
	content := `{
		"totals": {
			"covered_lines": 80,
			"num_statements": 100,
			"percent_covered": 80.0
		},
		"files": {
			"main.py": {
				"summary": {
					"covered_lines": 40,
					"num_statements": 50,
					"percent_covered": 80.0
				},
				"missing_lines": [10, 15, 20]
			},
			"util.py": {
				"summary": {
					"covered_lines": 40,
					"num_statements": 50,
					"percent_covered": 80.0
				},
				"missing_lines": [5, 8]
			}
		}
	}`
	if err := os.WriteFile(coverFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write coverage file: %v", err)
	}

	c := NewCollector(tmpDir, "python")
	report, err := c.parsePythonCoverage(coverFile)
	if err != nil {
		t.Fatalf("parsePythonCoverage() error = %v", err)
	}

	if report.Language != "python" {
		t.Errorf("Language = %s, want python", report.Language)
	}
	if report.TotalLines != 100 {
		t.Errorf("TotalLines = %d, want 100", report.TotalLines)
	}
	if report.CoveredLines != 80 {
		t.Errorf("CoveredLines = %d, want 80", report.CoveredLines)
	}
	if report.Percentage != 80.0 {
		t.Errorf("Percentage = %f, want 80.0", report.Percentage)
	}
	if len(report.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(report.Files))
	}
	// 3 missing from main.py + 2 from util.py = 5
	if len(report.Uncovered) != 5 {
		t.Errorf("len(Uncovered) = %d, want 5", len(report.Uncovered))
	}
}

func TestParsePythonCoverage_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := filepath.Join(tmpDir, "coverage.json")

	if err := os.WriteFile(coverFile, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write coverage file: %v", err)
	}

	c := NewCollector(tmpDir, "python")
	_, err := c.parsePythonCoverage(coverFile)
	if err == nil {
		t.Error("parsePythonCoverage() should return error for invalid JSON")
	}
}

func TestParseJSCoverage(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := filepath.Join(tmpDir, "coverage-summary.json")

	// Create mock Jest coverage summary JSON
	content := `{
		"total": {
			"lines": {"total": 100, "covered": 85, "skipped": 0, "pct": 85.0}
		},
		"src/main.js": {
			"lines": {"total": 50, "covered": 45, "skipped": 0, "pct": 90.0}
		},
		"src/util.js": {
			"lines": {"total": 50, "covered": 40, "skipped": 0, "pct": 80.0}
		}
	}`
	if err := os.WriteFile(coverFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write coverage file: %v", err)
	}

	c := NewCollector(tmpDir, "javascript")
	report, err := c.parseJSCoverage(coverFile)
	if err != nil {
		t.Fatalf("parseJSCoverage() error = %v", err)
	}

	if report.Language != "javascript" {
		t.Errorf("Language = %s, want javascript", report.Language)
	}
	if report.TotalLines != 100 {
		t.Errorf("TotalLines = %d, want 100", report.TotalLines)
	}
	if report.CoveredLines != 85 {
		t.Errorf("CoveredLines = %d, want 85", report.CoveredLines)
	}
	if report.Percentage != 85.0 {
		t.Errorf("Percentage = %f, want 85.0", report.Percentage)
	}
	if len(report.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(report.Files))
	}
}

func TestParseJSCoverage_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	coverFile := filepath.Join(tmpDir, "coverage-summary.json")

	if err := os.WriteFile(coverFile, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("Failed to write coverage file: %v", err)
	}

	c := NewCollector(tmpDir, "javascript")
	_, err := c.parseJSCoverage(coverFile)
	if err == nil {
		t.Error("parseJSCoverage() should return error for invalid JSON")
	}
}

func TestGetUncoveredFunctions(t *testing.T) {
	report := &CoverageReport{
		Files: []FileCoverage{
			{Path: "main.go", Percentage: 50.0, TotalLines: 100},
			{Path: "util.go", Percentage: 90.0, TotalLines: 50},
			{Path: "test.go", Percentage: 70.0, TotalLines: 30},
		},
		Uncovered: []UncoveredItem{
			{File: "main.go", StartLine: 10, EndLine: 20, Type: "line"},
		},
	}

	c := NewCollector("/tmp", "go")

	// With 80% threshold, main.go and test.go should be flagged
	uncovered := c.GetUncoveredFunctions(report, 80.0)

	// Should include main.go (50%), test.go (70%), and the original uncovered item
	if len(uncovered) < 2 {
		t.Errorf("len(uncovered) = %d, want at least 2", len(uncovered))
	}
}

func TestSaveAndLoadReport(t *testing.T) {
	tmpDir := t.TempDir()
	reportPath := filepath.Join(tmpDir, "report.json")

	original := &CoverageReport{
		Timestamp:    time.Now().Truncate(time.Second),
		Language:     "go",
		TotalLines:   1000,
		CoveredLines: 800,
		Percentage:   80.0,
		Files: []FileCoverage{
			{Path: "main.go", TotalLines: 500, CoveredLines: 400, Percentage: 80.0},
		},
		Uncovered: []UncoveredItem{
			{File: "main.go", StartLine: 10, EndLine: 20, Type: "line", Name: "TestFunc"},
		},
	}

	c := NewCollector(tmpDir, "go")
	if err := c.SaveReport(original, reportPath); err != nil {
		t.Fatalf("SaveReport() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Fatal("Report file was not created")
	}

	// Load it back
	loaded, err := LoadReport(reportPath)
	if err != nil {
		t.Fatalf("LoadReport() error = %v", err)
	}

	if loaded.Language != original.Language {
		t.Errorf("Language = %s, want %s", loaded.Language, original.Language)
	}
	if loaded.TotalLines != original.TotalLines {
		t.Errorf("TotalLines = %d, want %d", loaded.TotalLines, original.TotalLines)
	}
	if loaded.CoveredLines != original.CoveredLines {
		t.Errorf("CoveredLines = %d, want %d", loaded.CoveredLines, original.CoveredLines)
	}
	if loaded.Percentage != original.Percentage {
		t.Errorf("Percentage = %f, want %f", loaded.Percentage, original.Percentage)
	}
	if len(loaded.Files) != len(original.Files) {
		t.Errorf("len(Files) = %d, want %d", len(loaded.Files), len(original.Files))
	}
	if len(loaded.Uncovered) != len(original.Uncovered) {
		t.Errorf("len(Uncovered) = %d, want %d", len(loaded.Uncovered), len(original.Uncovered))
	}
}

func TestLoadReport_FileNotFound(t *testing.T) {
	_, err := LoadReport("/nonexistent/report.json")
	if err == nil {
		t.Error("LoadReport() should return error for nonexistent file")
	}
}

func TestLoadReport_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	reportPath := filepath.Join(tmpDir, "report.json")

	if err := os.WriteFile(reportPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err := LoadReport(reportPath)
	if err == nil {
		t.Error("LoadReport() should return error for invalid JSON")
	}
}

func TestPytestCoverageJSON_Fields(t *testing.T) {
	// Test the struct can be instantiated correctly
	pyCov := PytestCoverageJSON{}

	// These are nested anonymous structs, verify they work
	pyCov.Totals.CoveredLines = 80
	pyCov.Totals.NumStatements = 100
	pyCov.Totals.PercentCovered = 80.0

	if pyCov.Totals.CoveredLines != 80 {
		t.Errorf("CoveredLines = %d, want 80", pyCov.Totals.CoveredLines)
	}
	if pyCov.Totals.NumStatements != 100 {
		t.Errorf("NumStatements = %d, want 100", pyCov.Totals.NumStatements)
	}
}

func TestJestCoverageSummary_Fields(t *testing.T) {
	summary := JestCoverageSummary{
		Total: make(map[string]struct {
			Total   int     `json:"total"`
			Covered int     `json:"covered"`
			Skipped int     `json:"skipped"`
			Pct     float64 `json:"pct"`
		}),
	}

	summary.Total["lines"] = struct {
		Total   int     `json:"total"`
		Covered int     `json:"covered"`
		Skipped int     `json:"skipped"`
		Pct     float64 `json:"pct"`
	}{
		Total:   100,
		Covered: 85,
		Pct:     85.0,
	}

	if summary.Total["lines"].Total != 100 {
		t.Errorf("Total = %d, want 100", summary.Total["lines"].Total)
	}
}
