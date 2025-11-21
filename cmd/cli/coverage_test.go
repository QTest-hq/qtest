package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/codecov"
)

func TestDetectProjectLanguage(t *testing.T) {
	// Test with current directory (has go.mod)
	lang := detectProjectLanguage(".")
	if lang != "go" {
		t.Errorf("detectProjectLanguage('.') = %s, want go", lang)
	}
}

func TestDetectProjectLanguage_Default(t *testing.T) {
	// Test with temp directory (no marker files)
	dir := t.TempDir()
	lang := detectProjectLanguage(dir)
	if lang != "go" {
		t.Errorf("detectProjectLanguage(empty) = %s, want go (default)", lang)
	}
}

func TestDetectProjectLanguage_Python(t *testing.T) {
	dir := t.TempDir()

	// Create requirements.txt
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("pytest"), 0644); err != nil {
		t.Fatal(err)
	}

	lang := detectProjectLanguage(dir)
	if lang != "python" {
		t.Errorf("detectProjectLanguage(python project) = %s, want python", lang)
	}
}

func TestDetectProjectLanguage_JavaScript(t *testing.T) {
	dir := t.TempDir()

	// Create package.json
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	lang := detectProjectLanguage(dir)
	if lang != "javascript" {
		t.Errorf("detectProjectLanguage(js project) = %s, want javascript", lang)
	}
}

func TestDisplayCoverageReport(t *testing.T) {
	// Just verify it doesn't panic with valid input
	report := &codecov.CoverageReport{
		Timestamp:    time.Now(),
		Language:     "go",
		TotalLines:   100,
		CoveredLines: 80,
		Percentage:   80.0,
		Files: []codecov.FileCoverage{
			{Path: "foo.go", TotalLines: 50, CoveredLines: 40, Percentage: 80.0},
			{Path: "bar.go", TotalLines: 50, CoveredLines: 40, Percentage: 80.0},
		},
	}

	// Should not panic
	displayCoverageReport(report)
}

func TestDisplayCoverageReport_EmptyFiles(t *testing.T) {
	report := &codecov.CoverageReport{
		TotalLines:   0,
		CoveredLines: 0,
		Percentage:   0.0,
		Files:        []codecov.FileCoverage{},
	}

	// Should not panic
	displayCoverageReport(report)
}

func TestDisplayCoverageReport_LowCoverage(t *testing.T) {
	report := &codecov.CoverageReport{
		TotalLines:   100,
		CoveredLines: 30,
		Percentage:   30.0,
		Files: []codecov.FileCoverage{
			{Path: "low.go", TotalLines: 100, CoveredLines: 30, Percentage: 30.0},
		},
	}

	// Should not panic
	displayCoverageReport(report)
}

func TestGenerateCoverageHTML(t *testing.T) {
	dir := t.TempDir()

	report := &codecov.CoverageReport{
		Timestamp:    time.Now(),
		Language:     "go",
		TotalLines:   100,
		CoveredLines: 85,
		Percentage:   85.0,
		Files: []codecov.FileCoverage{
			{Path: "main.go", TotalLines: 50, CoveredLines: 45, Percentage: 90.0},
			{Path: "utils.go", TotalLines: 50, CoveredLines: 40, Percentage: 80.0},
		},
	}

	outputPath := filepath.Join(dir, "coverage.html")
	err := generateCoverageHTML(report, outputPath)
	if err != nil {
		t.Fatalf("generateCoverageHTML() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("HTML file was not created")
	}

	// Verify content
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read HTML file: %v", err)
	}

	contentStr := string(content)

	// Check for key content
	if !strings.Contains(contentStr, "Coverage Report") {
		t.Error("HTML should contain 'Coverage Report'")
	}
	if !strings.Contains(contentStr, "85.0%") {
		t.Error("HTML should contain coverage percentage")
	}
	if !strings.Contains(contentStr, "main.go") {
		t.Error("HTML should contain file names")
	}
	if !strings.Contains(contentStr, "quality-good") {
		t.Error("HTML should have good quality class for 85%")
	}
}

func TestGenerateCoverageHTML_LowCoverage(t *testing.T) {
	dir := t.TempDir()

	report := &codecov.CoverageReport{
		Timestamp:    time.Now(),
		TotalLines:   100,
		CoveredLines: 30,
		Percentage:   30.0,
		Files: []codecov.FileCoverage{
			{Path: "bad.go", TotalLines: 100, CoveredLines: 30, Percentage: 30.0},
		},
	}

	outputPath := filepath.Join(dir, "coverage.html")
	err := generateCoverageHTML(report, outputPath)
	if err != nil {
		t.Fatalf("generateCoverageHTML() error: %v", err)
	}

	content, _ := os.ReadFile(outputPath)
	if !strings.Contains(string(content), "quality-poor") {
		t.Error("HTML should have poor quality class for 30%")
	}
}

func TestGenerateCoverageHTML_AcceptableCoverage(t *testing.T) {
	dir := t.TempDir()

	report := &codecov.CoverageReport{
		Timestamp:    time.Now(),
		TotalLines:   100,
		CoveredLines: 60,
		Percentage:   60.0,
		Files:        []codecov.FileCoverage{},
	}

	outputPath := filepath.Join(dir, "coverage.html")
	err := generateCoverageHTML(report, outputPath)
	if err != nil {
		t.Fatalf("generateCoverageHTML() error: %v", err)
	}

	content, _ := os.ReadFile(outputPath)
	if !strings.Contains(string(content), "quality-acceptable") {
		t.Error("HTML should have acceptable quality class for 60%")
	}
}

func TestGenerateCoverageHTML_NestedDir(t *testing.T) {
	dir := t.TempDir()

	report := &codecov.CoverageReport{
		Timestamp:    time.Now(),
		TotalLines:   10,
		CoveredLines: 10,
		Percentage:   100.0,
	}

	// Test with nested directory that doesn't exist
	outputPath := filepath.Join(dir, "nested", "dir", "coverage.html")
	err := generateCoverageHTML(report, outputPath)
	if err != nil {
		t.Fatalf("generateCoverageHTML() should create nested dirs: %v", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("HTML file should be created in nested directory")
	}
}
