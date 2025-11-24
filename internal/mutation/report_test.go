package mutation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewReporter_Report(t *testing.T) {
	reporter := NewReporter("/tmp/reports")
	if reporter == nil {
		t.Fatal("expected non-nil reporter")
	}
	if reporter.outputDir != "/tmp/reports" {
		t.Errorf("expected outputDir /tmp/reports, got %s", reporter.outputDir)
	}
}

func TestGenerateJSONReport(t *testing.T) {
	tmpDir := t.TempDir()
	reporter := NewReporter(tmpDir)

	result := &Result{
		SourceFile: "/path/to/source.go",
		TestFile:   "/path/to/source_test.go",
		Total:      10,
		Killed:     8,
		Survived:   2,
		Score:      0.8,
		Duration:   5 * time.Second,
		Mutants: []Mutant{
			{Type: "arithmetic", Line: 10, Status: StatusKilled, Description: "Changed + to -"},
			{Type: "comparison", Line: 15, Status: StatusSurvived, Description: "Changed == to !="},
		},
	}

	path, err := reporter.GenerateReport(result, FormatJSON)
	if err != nil {
		t.Fatalf("failed to generate JSON report: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("report file not created: %s", path)
	}

	// Verify file is in output dir
	if !strings.HasPrefix(path, tmpDir) {
		t.Errorf("report not in output dir: %s", path)
	}

	// Verify it's valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	var parsed Result
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("report is not valid JSON: %v", err)
	}

	if parsed.SourceFile != result.SourceFile {
		t.Errorf("SourceFile mismatch: got %s", parsed.SourceFile)
	}
	if parsed.Total != result.Total {
		t.Errorf("Total mismatch: got %d", parsed.Total)
	}
	if parsed.Score != result.Score {
		t.Errorf("Score mismatch: got %f", parsed.Score)
	}
}

func TestGenerateTextReport(t *testing.T) {
	tmpDir := t.TempDir()
	reporter := NewReporter(tmpDir)

	result := &Result{
		SourceFile: "/path/to/source.go",
		TestFile:   "/path/to/source_test.go",
		Total:      10,
		Killed:     7,
		Survived:   2,
		Timeout:    1,
		Score:      0.7,
		Duration:   3 * time.Second,
		Mutants: []Mutant{
			{Type: "arithmetic", Line: 10, Status: StatusKilled, Description: "Changed + to -"},
			{Type: "comparison", Line: 15, Status: StatusSurvived, Description: "Changed == to !="},
			{Type: "boundary", Line: 20, Status: StatusTimeout, Description: "Changed < to <="},
		},
	}

	path, err := reporter.GenerateReport(result, FormatText)
	if err != nil {
		t.Fatalf("failed to generate text report: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("report file not created: %s", path)
	}

	// Read and verify content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	content := string(data)

	// Check for expected sections
	expectedStrings := []string{
		"MUTATION TESTING REPORT",
		"Source File:",
		"Test File:",
		"Total Mutants:",
		"Killed:",
		"Survived:",
		"Timeout:",
		"Score:",
		"MUTANT DETAILS",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(content, expected) {
			t.Errorf("report missing expected string: %s", expected)
		}
	}

	// Check for mutant status icons
	if !strings.Contains(content, "[✓]") {
		t.Error("report missing killed icon")
	}
	if !strings.Contains(content, "[✗]") {
		t.Error("report missing survived icon")
	}
	if !strings.Contains(content, "[⏱]") {
		t.Error("report missing timeout icon")
	}
}

func TestGenerateHTMLReport(t *testing.T) {
	tmpDir := t.TempDir()
	reporter := NewReporter(tmpDir)

	result := &Result{
		SourceFile: "/path/to/source.go",
		TestFile:   "/path/to/source_test.go",
		Total:      10,
		Killed:     8,
		Survived:   2,
		Score:      0.8,
		Duration:   5 * time.Second,
		Mutants: []Mutant{
			{Type: "arithmetic", Line: 10, Status: StatusKilled, Description: "Changed + to -"},
			{Type: "comparison", Line: 15, Status: StatusSurvived, Description: "Changed == to !="},
		},
	}

	path, err := reporter.GenerateReport(result, FormatHTML)
	if err != nil {
		t.Fatalf("failed to generate HTML report: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("report file not created: %s", path)
	}

	// Verify it has .html extension
	if !strings.HasSuffix(path, ".html") {
		t.Errorf("HTML report should have .html extension: %s", path)
	}

	// Read and verify content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	content := string(data)

	// Check for expected HTML elements
	expectedStrings := []string{
		"<!DOCTYPE html>",
		"<html",
		"Mutation Testing Report",
		"/path/to/source.go",
		"/path/to/source_test.go",
		"80.0%",
		"Total Mutants",
		"Killed",
		"Survived",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(content, expected) {
			t.Errorf("HTML report missing expected string: %s", expected)
		}
	}
}

func TestGenerateReportUnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	reporter := NewReporter(tmpDir)

	result := &Result{}

	_, err := reporter.GenerateReport(result, "invalid")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestQualityClass_Report(t *testing.T) {
	tests := []struct {
		quality  string
		expected string
	}{
		{"good", "quality-good"},
		{"acceptable", "quality-acceptable"},
		{"poor", "quality-poor"},
		{"unknown", "quality-poor"},
	}

	for _, tc := range tests {
		result := qualityClass(tc.quality)
		if result != tc.expected {
			t.Errorf("qualityClass(%q) = %q, expected %q", tc.quality, result, tc.expected)
		}
	}
}

func TestStatusIcon_Report(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{StatusKilled, "✓"},
		{StatusSurvived, "✗"},
		{StatusTimeout, "⏱"},
		{StatusError, "?"},
		{"unknown", "?"},
	}

	for _, tc := range tests {
		result := statusIcon(tc.status)
		if result != tc.expected {
			t.Errorf("statusIcon(%q) = %q, expected %q", tc.status, result, tc.expected)
		}
	}
}

func TestStatusClass_Report(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{StatusKilled, "status-killed"},
		{StatusSurvived, "status-survived"},
		{StatusTimeout, "status-timeout"},
		{StatusError, "status-error"},
		{"unknown", "status-error"},
	}

	for _, tc := range tests {
		result := statusClass(tc.status)
		if result != tc.expected {
			t.Errorf("statusClass(%q) = %q, expected %q", tc.status, result, tc.expected)
		}
	}
}

func TestGenerateReportCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "nested", "output", "dir")
	reporter := NewReporter(outputDir)

	result := &Result{
		SourceFile: "source.go",
		TestFile:   "source_test.go",
		Total:      5,
		Killed:     4,
		Score:      0.8,
	}

	// Directory shouldn't exist yet
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatal("output dir shouldn't exist before test")
	}

	path, err := reporter.GenerateReport(result, FormatJSON)
	if err != nil {
		t.Fatalf("failed to generate report: %v", err)
	}

	// Directory should be created
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("output dir should be created")
	}

	// File should exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("report file should exist")
	}
}

func TestGenerateTextReportNoMutants(t *testing.T) {
	tmpDir := t.TempDir()
	reporter := NewReporter(tmpDir)

	result := &Result{
		SourceFile: "source.go",
		TestFile:   "source_test.go",
		Total:      0,
		Score:      0,
	}

	path, err := reporter.GenerateReport(result, FormatText)
	if err != nil {
		t.Fatalf("failed to generate report: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	content := string(data)

	// Should not have mutant details section content
	if strings.Contains(content, "Mutant #1") {
		t.Error("report shouldn't have mutant details when no mutants")
	}
}

func TestGenerateHTMLReportNoMutants(t *testing.T) {
	tmpDir := t.TempDir()
	reporter := NewReporter(tmpDir)

	result := &Result{
		SourceFile: "source.go",
		TestFile:   "source_test.go",
		Total:      0,
		Score:      0,
	}

	path, err := reporter.GenerateReport(result, FormatHTML)
	if err != nil {
		t.Fatalf("failed to generate report: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	content := string(data)

	// Should show "no mutants" message
	if !strings.Contains(content, "No mutant details available") {
		t.Error("HTML report should show 'no mutants' message when no mutants")
	}
}

func TestGenerateHTMLReportWithAllStatuses(t *testing.T) {
	tmpDir := t.TempDir()
	reporter := NewReporter(tmpDir)

	result := &Result{
		SourceFile: "source.go",
		TestFile:   "source_test.go",
		Total:      4,
		Killed:     1,
		Survived:   1,
		Timeout:    1,
		Score:      0.25,
		Mutants: []Mutant{
			{Type: "arithmetic", Line: 10, Status: StatusKilled},
			{Type: "comparison", Line: 20, Status: StatusSurvived},
			{Type: "boundary", Line: 30, Status: StatusTimeout},
			{Type: "logical", Line: 40, Status: StatusError},
		},
	}

	path, err := reporter.GenerateReport(result, FormatHTML)
	if err != nil {
		t.Fatalf("failed to generate report: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	content := string(data)

	// Check all status classes are present
	expectedClasses := []string{
		"status-killed",
		"status-survived",
		"status-timeout",
		"status-error",
	}

	for _, class := range expectedClasses {
		if !strings.Contains(content, class) {
			t.Errorf("HTML report missing status class: %s", class)
		}
	}
}

func TestReportFormatConstants(t *testing.T) {
	// Verify format constants have expected values
	if FormatHTML != "html" {
		t.Errorf("FormatHTML = %q, expected 'html'", FormatHTML)
	}
	if FormatJSON != "json" {
		t.Errorf("FormatJSON = %q, expected 'json'", FormatJSON)
	}
	if FormatText != "text" {
		t.Errorf("FormatText = %q, expected 'text'", FormatText)
	}
}

func TestHTMLReportDataStruct(t *testing.T) {
	data := htmlReportData{
		Title:          "Test Report",
		GeneratedAt:    "2024-01-01 12:00:00",
		SourceFile:     "source.go",
		TestFile:       "source_test.go",
		TotalMutants:   10,
		Killed:         8,
		Survived:       2,
		Timeout:        0,
		Score:          80.0,
		ScoreFormatted: "80.0%",
		Quality:        "good",
		QualityClass:   "quality-good",
		Duration:       "5s",
	}

	if data.Title != "Test Report" {
		t.Error("htmlReportData Title mismatch")
	}
	if data.TotalMutants != 10 {
		t.Error("htmlReportData TotalMutants mismatch")
	}
	if data.Score != 80.0 {
		t.Error("htmlReportData Score mismatch")
	}
}

func TestGenerateJSONReportWithAllFields(t *testing.T) {
	tmpDir := t.TempDir()
	reporter := NewReporter(tmpDir)

	result := &Result{
		SourceFile: "/path/to/source.go",
		TestFile:   "/path/to/source_test.go",
		Total:      10,
		Killed:     7,
		Survived:   2,
		Timeout:    1,
		Score:      0.7,
		Duration:   5*time.Second + 500*time.Millisecond,
		Error:      "",
		Mutants: []Mutant{
			{
				Type:        "arithmetic",
				Line:        10,
				Status:      StatusKilled,
				Description: "Changed + to -",
				Original:    "a + b",
				Mutated:     "a - b",
			},
		},
	}

	path, err := reporter.GenerateReport(result, FormatJSON)
	if err != nil {
		t.Fatalf("failed to generate report: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	// Verify all expected fields are in JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	expectedFields := []string{"source_file", "test_file", "total", "killed", "survived", "timeout", "score", "mutants"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("JSON report missing field: %s", field)
		}
	}
}
