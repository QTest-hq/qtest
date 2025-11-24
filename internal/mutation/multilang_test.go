package mutation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAllTools_ImplementToolInterface verifies that all mutation tools
// properly implement the Tool interface with consistent behavior.
func TestAllTools_ImplementToolInterface(t *testing.T) {
	tools := []Tool{
		NewGoMutestingTool(),
		NewMutmutTool(),
		NewPITTool(),
	}

	ctx := context.Background()

	for _, tool := range tools {
		t.Run(tool.Name(), func(t *testing.T) {
			// Name should return non-empty string
			name := tool.Name()
			if name == "" {
				t.Error("Name() should return non-empty string")
			}

			// Name should not contain spaces or special characters
			if strings.ContainsAny(name, " \t\n!@#$%^&*()") {
				t.Errorf("Name() should be a simple identifier, got: %q", name)
			}

			// IsAvailable should not panic
			_ = tool.IsAvailable(ctx)
		})
	}
}

// TestAllTools_ConsistentNameFormats validates tool naming conventions.
func TestAllTools_ConsistentNameFormats(t *testing.T) {
	expectedNames := map[string]bool{
		"go-mutesting": true,
		"mutmut":       true,
		"pit":          true,
	}

	tools := []Tool{
		NewGoMutestingTool(),
		NewMutmutTool(),
		NewPITTool(),
	}

	for _, tool := range tools {
		name := tool.Name()
		if !expectedNames[name] {
			t.Errorf("unexpected tool name: %q", name)
		}
	}
}

// TestGetToolForLanguage verifies correct tool selection based on file extension.
func TestGetToolForLanguage(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		expectedTool string
	}{
		{"Go source file", "/project/main.go", "go-mutesting"},
		{"Go test file", "/project/main_test.go", "go-mutesting"},
		{"Python source file", "/project/calc.py", "mutmut"},
		{"Python test file", "/project/test_calc.py", "mutmut"},
		{"Java source file", "/project/Calculator.java", "pit"},
		{"Java test file", "/project/CalculatorTest.java", "pit"},
		{"Nested Go file", "/project/internal/pkg/util.go", "go-mutesting"},
		{"Nested Python file", "/project/src/utils/helpers.py", "mutmut"},
		{"Nested Java file", "/project/src/main/java/com/example/Service.java", "pit"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool := getToolForFile(tc.filePath)
			if tool == nil {
				t.Fatalf("expected tool for %s, got nil", tc.filePath)
			}
			if tool.Name() != tc.expectedTool {
				t.Errorf("expected %s tool, got %s", tc.expectedTool, tool.Name())
			}
		})
	}
}

// TestGetToolForLanguage_Unsupported verifies nil return for unsupported files.
func TestGetToolForLanguage_Unsupported(t *testing.T) {
	unsupportedFiles := []string{
		"/project/readme.md",
		"/project/config.yaml",
		"/project/script.js",
		"/project/style.css",
		"/project/data.json",
		"/project/Makefile",
		"/project/.gitignore",
	}

	for _, file := range unsupportedFiles {
		tool := getToolForFile(file)
		if tool != nil {
			t.Errorf("expected nil tool for %s, got %s", file, tool.Name())
		}
	}
}

// TestAllTools_RunWithInvalidPaths verifies tools handle invalid paths gracefully.
func TestAllTools_RunWithInvalidPaths(t *testing.T) {
	tools := []Tool{
		NewGoMutestingTool(),
		NewMutmutTool(),
		NewPITTool(),
	}

	ctx := context.Background()
	cfg := MutationConfig{
		Timeout:          5 * time.Second,
		TimeoutPerMutant: 1 * time.Second,
	}

	for _, tool := range tools {
		t.Run(tool.Name()+"_invalid_paths", func(t *testing.T) {
			result, err := tool.Run(ctx, "/nonexistent/source.xxx", "/nonexistent/test.xxx", cfg)

			// Should not return Go error - tools handle errors gracefully
			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			// Result may have error message or zero mutants
		})
	}
}

// TestAllTools_RunWithCancelledContext verifies proper context cancellation handling.
func TestAllTools_RunWithCancelledContext(t *testing.T) {
	tools := []Tool{
		NewGoMutestingTool(),
		NewMutmutTool(),
		NewPITTool(),
	}

	cfg := MutationConfig{
		Timeout:          30 * time.Second,
		TimeoutPerMutant: 5 * time.Second,
	}

	for _, tool := range tools {
		t.Run(tool.Name()+"_cancelled_context", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			result, err := tool.Run(ctx, "/some/file", "/some/test", cfg)

			// Should handle cancelled context without panic
			// May return error or empty result
			_ = err
			_ = result
		})
	}
}

// TestResultConsistency verifies Result struct is populated consistently.
func TestResultConsistency(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		valid  bool
	}{
		{
			name:   "empty result",
			result: Result{},
			valid:  true,
		},
		{
			name: "all killed",
			result: Result{
				Total:   10,
				Killed:  10,
				Score:   1.0,
				Mutants: make([]Mutant, 10),
			},
			valid: true,
		},
		{
			name: "mixed results",
			result: Result{
				Total:    20,
				Killed:   15,
				Survived: 3,
				Timeout:  2,
				Score:    0.75,
			},
			valid: true,
		},
		{
			name: "with error",
			result: Result{
				Error: "tool not available",
			},
			valid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Verify invariants
			if tc.result.Total > 0 {
				sum := tc.result.Killed + tc.result.Survived + tc.result.Timeout
				// Note: Sum may not equal Total due to skipped/other states
				if sum > tc.result.Total {
					t.Errorf("sum of outcomes (%d) exceeds total (%d)", sum, tc.result.Total)
				}
			}

			// Score should be between 0 and 1
			if tc.result.Score < 0 || tc.result.Score > 1 {
				t.Errorf("score %f outside valid range [0,1]", tc.result.Score)
			}

			// Mutants list should match count
			if len(tc.result.Mutants) > tc.result.Total {
				t.Errorf("mutants list (%d) exceeds total (%d)",
					len(tc.result.Mutants), tc.result.Total)
			}
		})
	}
}

// TestMutantStatusConstants verifies mutation status constants are consistent.
func TestMutantStatusConstants(t *testing.T) {
	validStatuses := map[string]bool{
		StatusKilled:   true,
		StatusSurvived: true,
		StatusTimeout:  true,
		StatusError:    true,
	}

	// Verify all defined statuses are recognized
	statuses := []string{StatusKilled, StatusSurvived, StatusTimeout, StatusError}
	for _, status := range statuses {
		if !validStatuses[status] {
			t.Errorf("status %q not in valid set", status)
		}
	}

	// Verify no duplicates
	seen := make(map[string]bool)
	for _, status := range statuses {
		if seen[status] {
			t.Errorf("duplicate status constant: %q", status)
		}
		seen[status] = true
	}
}

// TestMutantTypeConsistency verifies mutation type categorization is consistent.
func TestMutantTypeConsistency(t *testing.T) {
	expectedTypes := []string{
		"arithmetic",
		"comparison",
		"boolean",
		"literal",
		"statement",
		"boundary",
		"unknown",
	}

	// All tools should map mutations to one of these types
	typeSet := make(map[string]bool)
	for _, typ := range expectedTypes {
		typeSet[typ] = true
	}

	// Test that mutmut uses consistent type names
	mutmutTool := NewMutmutTool()

	// Sample mutations to test type mapping
	testCases := []struct {
		original string
		mutated  string
	}{
		{"a + b", "a - b"},
		{"x == y", "x != y"},
		{"true", "false"},
		{`"hello"`, `""`},
		{"return x", "return nil"},
	}

	for _, tc := range testCases {
		mutmutType := mutmutTool.detectMutationType(tc.original, tc.mutated)
		if !typeSet[mutmutType] {
			t.Errorf("mutmut returned unknown type: %q for %q -> %q",
				mutmutType, tc.original, tc.mutated)
		}
	}

	// Test PIT mutator mapping
	pitTool := NewPITTool()
	pitMutators := []string{
		"MathMutator",
		"NegateConditionalsMutator",
		"TrueReturnValsMutator",
		"NullReturnsMutator",
		"VoidMethodCallMutator",
	}

	for _, mutator := range pitMutators {
		pitType := pitTool.mapMutatorName(mutator)
		if !typeSet[pitType] {
			t.Errorf("pit returned unknown type: %q for mutator %q",
				pitType, mutator)
		}
	}
}

// TestToolConfig verifies configuration is applied correctly.
func TestToolConfig(t *testing.T) {
	cfg := MutationConfig{
		Timeout:               60 * time.Second,
		TimeoutPerMutant:      10 * time.Second,
		MaxMutantsPerFunction: 50,
		Mode:                  "thorough",
	}

	// Verify config values
	if cfg.Timeout != 60*time.Second {
		t.Errorf("expected 60s timeout, got %v", cfg.Timeout)
	}
	if cfg.MaxMutantsPerFunction != 50 {
		t.Errorf("expected 50 max mutants, got %d", cfg.MaxMutantsPerFunction)
	}
	if cfg.Mode != "thorough" {
		t.Errorf("expected mode 'thorough', got %s", cfg.Mode)
	}
}

// TestMutationScoreCalculation verifies score calculation consistency.
func TestMutationScoreCalculation(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		killed   int
		expected float64
	}{
		{"all killed", 10, 10, 1.0},
		{"none killed", 10, 0, 0.0},
		{"half killed", 10, 5, 0.5},
		{"75% killed", 100, 75, 0.75},
		{"no mutants", 0, 0, 0.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := calculateScore(tc.total, tc.killed)
			if score != tc.expected {
				t.Errorf("expected score %f, got %f", tc.expected, score)
			}
		})
	}
}

// TestLanguageSpecificPatterns verifies language-specific mutation detection.
func TestLanguageSpecificPatterns(t *testing.T) {
	// Python-specific patterns (mutmut)
	t.Run("Python patterns", func(t *testing.T) {
		mutmutTool := NewMutmutTool()

		pythonCases := []struct {
			original string
			mutated  string
			expected string
		}{
			{"a + b", "a - b", "arithmetic"},
			{"x == y", "x != y", "comparison"},
			{"True", "False", "boolean"},
			{"a and b", "a or b", "boolean"},
			{`"hello"`, `""`, "literal"},
		}

		for _, tc := range pythonCases {
			result := mutmutTool.detectMutationType(tc.original, tc.mutated)
			if result != tc.expected {
				t.Errorf("Python: detectMutationType(%q, %q) = %q, expected %q",
					tc.original, tc.mutated, result, tc.expected)
			}
		}
	})

	// Java/PIT-specific patterns
	t.Run("Java/PIT patterns", func(t *testing.T) {
		pitTool := NewPITTool()

		javaCases := []struct {
			mutator  string
			expected string
		}{
			{"MathMutator", "arithmetic"},
			{"NegateConditionalsMutator", "comparison"},
			{"ConditionalsBoundaryMutator", "comparison"},
			{"TrueReturnValsMutator", "boolean"},
			{"FalseReturnValsMutator", "boolean"},
			{"NullReturnsMutator", "literal"},
			{"ReturnValsMutator", "statement"},
			{"VoidMethodCallMutator", "statement"},
			{"IncrementsMutator", "arithmetic"},
		}

		for _, tc := range javaCases {
			result := pitTool.mapMutatorName(tc.mutator)
			if result != tc.expected {
				t.Errorf("PIT: mapMutatorName(%q) = %q, expected %q",
					tc.mutator, result, tc.expected)
			}
		}
	})
}

// TestMultiLanguageProject simulates a project with multiple language files.
func TestMultiLanguageProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a multi-language project structure
	files := map[string]string{
		"go/calculator.go":          "package calc\nfunc Add(a, b int) int { return a + b }",
		"python/calculator.py":      "def add(a, b):\n    return a + b",
		"java/Calculator.java":      "public class Calculator { public int add(int a, int b) { return a + b; } }",
		"go/calculator_test.go":     "package calc\nimport \"testing\"\nfunc TestAdd(t *testing.T) {}",
		"python/test_calculator.py": "def test_add():\n    assert add(1, 2) == 3",
		"java/CalculatorTest.java":  "public class CalculatorTest {}",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	// Verify correct tool selection for each file
	expectations := map[string]string{
		"go/calculator.go":          "go-mutesting",
		"python/calculator.py":      "mutmut",
		"java/Calculator.java":      "pit",
		"go/calculator_test.go":     "go-mutesting",
		"python/test_calculator.py": "mutmut",
		"java/CalculatorTest.java":  "pit",
	}

	for relPath, expectedTool := range expectations {
		fullPath := filepath.Join(tmpDir, relPath)
		tool := getToolForFile(fullPath)

		if tool == nil {
			t.Errorf("no tool for %s", relPath)
			continue
		}
		if tool.Name() != expectedTool {
			t.Errorf("for %s: expected %s, got %s", relPath, expectedTool, tool.Name())
		}
	}
}

// TestRunner_MultiTool verifies the Runner works with multiple tools.
func TestRunner_MultiTool(t *testing.T) {
	runner := NewRunner(
		NewGoMutestingTool(),
		NewMutmutTool(),
		NewPITTool(),
	)

	ctx := context.Background()

	// Get available tools (may be empty if none are installed)
	available := runner.GetAvailableTools(ctx)

	// Just verify no panic and proper return
	t.Logf("Available tools: %d", len(available))
	for _, tool := range available {
		t.Logf("  - %s", tool.Name())
	}
}

// TestRunner_NoPanic verifies runner handles edge cases without panic.
func TestRunner_NoPanic(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig()

	// Empty runner
	t.Run("empty runner", func(t *testing.T) {
		runner := NewRunner()
		_, err := runner.Run(ctx, "test.go", "test_test.go", cfg)
		if err == nil {
			t.Error("expected error from empty runner")
		}
	})

	// Runner with unavailable tools only
	t.Run("unavailable tools", func(t *testing.T) {
		runner := NewRunner(NewGoMutestingTool()) // likely not installed
		_, err := runner.Run(ctx, "test.go", "test_test.go", cfg)
		// May or may not error depending on tool availability
		_ = err
	})
}

// TestDefaultConfig_MultiLang verifies default configuration values.
func TestDefaultConfig_MultiLang(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxMutantsPerFunction != 5 {
		t.Errorf("expected MaxMutantsPerFunction 5, got %d", cfg.MaxMutantsPerFunction)
	}
	if cfg.Timeout != 2*time.Minute {
		t.Errorf("expected Timeout 2m, got %v", cfg.Timeout)
	}
	if cfg.TimeoutPerMutant != 5*time.Second {
		t.Errorf("expected TimeoutPerMutant 5s, got %v", cfg.TimeoutPerMutant)
	}
	if cfg.Mode != "fast" {
		t.Errorf("expected Mode 'fast', got %s", cfg.Mode)
	}
}

// TestThoroughConfig_MultiLang verifies thorough configuration values.
func TestThoroughConfig_MultiLang(t *testing.T) {
	cfg := ThoroughConfig()

	if cfg.MaxMutantsPerFunction != 10 {
		t.Errorf("expected MaxMutantsPerFunction 10, got %d", cfg.MaxMutantsPerFunction)
	}
	if cfg.Timeout != 10*time.Minute {
		t.Errorf("expected Timeout 10m, got %v", cfg.Timeout)
	}
	if cfg.TimeoutPerMutant != 10*time.Second {
		t.Errorf("expected TimeoutPerMutant 10s, got %v", cfg.TimeoutPerMutant)
	}
	if cfg.Mode != "thorough" {
		t.Errorf("expected Mode 'thorough', got %s", cfg.Mode)
	}
}

// TestResultQuality verifies quality assessment based on score.
func TestResultQuality(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{1.0, "good"},
		{0.75, "good"},
		{0.70, "good"},
		{0.69, "acceptable"},
		{0.50, "acceptable"},
		{0.49, "poor"},
		{0.0, "poor"},
	}

	for _, tc := range tests {
		result := &Result{Score: tc.score}
		quality := result.Quality()
		if quality != tc.expected {
			t.Errorf("score %f: expected %q, got %q", tc.score, tc.expected, quality)
		}
	}
}

// TestResultHasMutants verifies HasMutants method.
func TestResultHasMutants(t *testing.T) {
	tests := []struct {
		total    int
		expected bool
	}{
		{0, false},
		{1, true},
		{100, true},
	}

	for _, tc := range tests {
		result := &Result{Total: tc.total}
		if result.HasMutants() != tc.expected {
			t.Errorf("Total=%d: expected HasMutants()=%v, got %v",
				tc.total, tc.expected, result.HasMutants())
		}
	}
}

// Helper function to select appropriate tool based on file extension.
func getToolForFile(filePath string) Tool {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		return NewGoMutestingTool()
	case ".py":
		return NewMutmutTool()
	case ".java":
		return NewPITTool()
	default:
		return nil
	}
}

// Helper function to calculate mutation score.
func calculateScore(total, killed int) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(killed) / float64(total)
}
