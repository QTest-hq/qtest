// Package integration provides end-to-end tests for QTest workflows
package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/QTest-hq/qtest/internal/codecov"
	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/QTest-hq/qtest/pkg/model"
)

// TestParseToModelWorkflow tests parsing source files and building a system model
func TestParseToModelWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Use the examples directory as test data
	examplesDir := filepath.Join("..", "..", "examples")
	if _, err := os.Stat(examplesDir); os.IsNotExist(err) {
		t.Skip("examples directory not found")
	}

	// Parse all Go files in examples
	p := parser.NewParser()
	adapter := model.NewParserAdapter("examples", "main", "")

	err := filepath.Walk(examplesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" {
			return nil
		}

		// Skip test files
		if len(path) > 8 && path[len(path)-8:] == "_test.go" {
			return nil
		}

		parsed, err := p.ParseFile(ctx, path)
		if err != nil {
			t.Logf("Warning: could not parse %s: %v", path, err)
			return nil
		}

		// Convert to model format
		pf := &model.ParsedFile{
			Path:     parsed.Path,
			Language: string(parsed.Language),
		}
		for _, fn := range parsed.Functions {
			params := make([]model.ParserParameter, len(fn.Parameters))
			for i, p := range fn.Parameters {
				params[i] = model.ParserParameter{
					Name: p.Name,
					Type: p.Type,
				}
			}
			pf.Functions = append(pf.Functions, model.ParserFunction{
				Name:       fn.Name,
				StartLine:  fn.StartLine,
				EndLine:    fn.EndLine,
				Parameters: params,
				ReturnType: fn.ReturnType,
				Exported:   fn.Exported,
			})
		}
		adapter.AddFile(pf)
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk examples: %v", err)
	}

	// Build the model
	sysModel, err := adapter.Build()
	if err != nil {
		t.Fatalf("Failed to build system model: %v", err)
	}

	// Verify model has content
	stats := sysModel.Stats()
	if stats["functions"] == 0 {
		t.Error("Expected functions in model")
	}

	t.Logf("Model built: %d functions, %d modules", stats["functions"], stats["modules"])
}

// TestCoverageWorkflow tests coverage collection workflow
// This test actually runs `go test` with coverage, so it's slow
func TestCoverageWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip by default as this runs the full test suite with coverage
	if os.Getenv("RUN_SLOW_TESTS") != "1" {
		t.Skip("skipping slow coverage test (set RUN_SLOW_TESTS=1 to run)")
	}

	ctx := context.Background()

	// Use current project as test target
	projectDir := filepath.Join("..", "..")
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Create collector
	collector := codecov.NewCollector(absDir, "go")

	// Collect coverage (this actually runs tests!)
	report, err := collector.Collect(ctx)
	if err != nil {
		t.Fatalf("Failed to collect coverage: %v", err)
	}

	// Verify report has content
	if report.TotalLines == 0 {
		t.Error("Expected total lines > 0")
	}
	if len(report.Files) == 0 {
		t.Error("Expected files in report")
	}

	t.Logf("Coverage: %.1f%% (%d/%d lines, %d files)",
		report.Percentage, report.CoveredLines, report.TotalLines, len(report.Files))

	// Test saving and loading report
	tempFile := filepath.Join(t.TempDir(), "coverage.json")
	if err := collector.SaveReport(report, tempFile); err != nil {
		t.Fatalf("Failed to save report: %v", err)
	}

	loaded, err := codecov.LoadReport(tempFile)
	if err != nil {
		t.Fatalf("Failed to load report: %v", err)
	}

	if loaded.TotalLines != report.TotalLines {
		t.Errorf("Loaded TotalLines = %d, want %d", loaded.TotalLines, report.TotalLines)
	}
}

// TestCoverageAnalysisWorkflow tests coverage gap analysis
func TestCoverageAnalysisWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a mock coverage report
	report := &codecov.CoverageReport{
		TotalLines:   100,
		CoveredLines: 60,
		Percentage:   60.0,
		Files: []codecov.FileCoverage{
			{
				Path:           "high_coverage.go",
				TotalLines:     50,
				CoveredLines:   45,
				Percentage:     90.0,
				UncoveredLines: []int{10, 15, 20, 25, 30},
			},
			{
				Path:           "low_coverage.go",
				TotalLines:     50,
				CoveredLines:   15,
				Percentage:     30.0,
				UncoveredLines: []int{1, 2, 3, 4, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50},
			},
		},
	}

	// Analyze gaps
	analyzer := codecov.NewAnalyzer(report, nil)
	result := analyzer.Analyze(80.0)

	// Verify analysis
	if result.TotalCoverage != 60.0 {
		t.Errorf("TotalCoverage = %.1f, want 60.0", result.TotalCoverage)
	}
	if result.TargetCoverage != 80.0 {
		t.Errorf("TargetCoverage = %.1f, want 80.0", result.TargetCoverage)
	}
	if len(result.Gaps) == 0 {
		t.Error("Expected gaps to be identified")
	}

	t.Logf("Analysis: %.1f%% current, %d gaps found", result.TotalCoverage, len(result.Gaps))

	// Generate test intents
	intents := analyzer.GenerateTestIntents(result.Gaps)
	if len(intents) == 0 {
		t.Error("Expected test intents to be generated")
	}

	t.Logf("Generated %d test intents", len(intents))
}

// TestParserLanguageDetection tests language detection
func TestParserLanguageDetection(t *testing.T) {
	tests := []struct {
		filename string
		wantLang parser.Language
	}{
		{"main.go", parser.LanguageGo},
		{"app.py", parser.LanguagePython},
		{"index.js", parser.LanguageJavaScript},
		{"app.ts", parser.LanguageTypeScript},
		{"Main.java", parser.LanguageJava},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := parser.DetectLanguage(tt.filename)
			if got != tt.wantLang {
				t.Errorf("DetectLanguage(%s) = %v, want %v", tt.filename, got, tt.wantLang)
			}
		})
	}
}
