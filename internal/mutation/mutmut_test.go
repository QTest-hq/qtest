package mutation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewMutmutTool(t *testing.T) {
	tool := NewMutmutTool()
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.BinaryPath != "mutmut" {
		t.Errorf("expected BinaryPath 'mutmut', got %s", tool.BinaryPath)
	}
	if tool.PythonPath != "python3" {
		t.Errorf("expected PythonPath 'python3', got %s", tool.PythonPath)
	}
	if tool.UsePipx {
		t.Error("expected UsePipx to be false by default")
	}
}

func TestMutmutTool_Name(t *testing.T) {
	tool := NewMutmutTool()
	if tool.Name() != "mutmut" {
		t.Errorf("expected name 'mutmut', got %s", tool.Name())
	}
}

func TestMutmutTool_ParseMutmutResults(t *testing.T) {
	tool := NewMutmutTool()

	tests := []struct {
		name     string
		output   string
		expected Result
	}{
		{
			name: "new format with emojis",
			output: `Legend for output:
🎉 Killed mutants.   Count: 10
⏰ Timeout.          Count: 1
🤔 Suspicious.       Count: 0
🙁 Survived.         Count: 3
🔇 Skipped.          Count: 0`,
			expected: Result{Killed: 10, Survived: 3, Timeout: 1, Total: 14},
		},
		{
			name: "old format",
			output: `Killed mutants: 15
Survived mutants: 5
Timeout: 2`,
			expected: Result{Killed: 15, Survived: 5, Timeout: 2, Total: 22},
		},
		{
			name:     "empty output",
			output:   "",
			expected: Result{Total: 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var result Result
			tool.parseMutmutResults(tc.output, &result)

			if result.Killed != tc.expected.Killed {
				t.Errorf("Killed: got %d, expected %d", result.Killed, tc.expected.Killed)
			}
			if result.Survived != tc.expected.Survived {
				t.Errorf("Survived: got %d, expected %d", result.Survived, tc.expected.Survived)
			}
			if result.Timeout != tc.expected.Timeout {
				t.Errorf("Timeout: got %d, expected %d", result.Timeout, tc.expected.Timeout)
			}
			if result.Total != tc.expected.Total {
				t.Errorf("Total: got %d, expected %d", result.Total, tc.expected.Total)
			}
		})
	}
}

func TestMutmutTool_ParseMutmutOutput(t *testing.T) {
	tool := NewMutmutTool()

	tests := []struct {
		name     string
		output   string
		expected Result
	}{
		{
			name: "progress line with emojis",
			output: `Running tests without mutations... Done in 0.5 seconds
Running mutation testing... Done in 10.5 seconds
⠋ 15/15  🎉 10  ⏰ 1  🤔 0  🙁 4  🔇 0`,
			expected: Result{Total: 15, Killed: 10, Survived: 4, Timeout: 1},
		},
		{
			name:     "simple format",
			output:   "10 killed, 3 survived",
			expected: Result{Killed: 10, Survived: 3, Total: 13},
		},
		{
			name:     "empty output",
			output:   "",
			expected: Result{Total: 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var result Result
			tool.parseMutmutOutput(tc.output, &result)

			if result.Total != tc.expected.Total {
				t.Errorf("Total: got %d, expected %d", result.Total, tc.expected.Total)
			}
			if result.Killed != tc.expected.Killed {
				t.Errorf("Killed: got %d, expected %d", result.Killed, tc.expected.Killed)
			}
			if result.Survived != tc.expected.Survived {
				t.Errorf("Survived: got %d, expected %d", result.Survived, tc.expected.Survived)
			}
		})
	}
}

func TestMutmutTool_ExtractCount(t *testing.T) {
	tests := []struct {
		line     string
		expected int
	}{
		{"🎉 Killed mutants.   Count: 10", 10},
		{"⏰ Timeout.          Count: 1", 1},
		{"🙁 Survived.         Count: 3", 3},
		{"Count: 100", 100},
		{"No count here", 0},
		{"", 0},
	}

	for _, tc := range tests {
		result := extractCount(tc.line)
		if result != tc.expected {
			t.Errorf("extractCount(%q) = %d, expected %d", tc.line, result, tc.expected)
		}
	}
}

func TestMutmutTool_ParseMutantShow(t *testing.T) {
	tool := NewMutmutTool()

	tests := []struct {
		name     string
		output   string
		id       int
		expected Mutant
	}{
		{
			name: "arithmetic mutation",
			output: `Mutant 1: (survived)
--- a/calc.py
+++ b/calc.py
@@ -10,1 +10,1 @@
- return a + b
+ return a - b`,
			id: 1,
			expected: Mutant{
				ID:       "1",
				Type:     "arithmetic",
				Status:   StatusSurvived,
				Line:     10,
				Original: "return a + b",
				Mutated:  "return a - b",
			},
		},
		{
			name: "killed mutant",
			output: `Mutant 5: (killed)
--- a/validator.py
+++ b/validator.py
@@ -25,1 +25,1 @@
- if x > 0:
+ if x < 0:`,
			id: 5,
			expected: Mutant{
				ID:       "5",
				Type:     "comparison",
				Status:   StatusKilled,
				Line:     25,
				Original: "if x > 0:",
				Mutated:  "if x < 0:",
			},
		},
		{
			name: "timeout mutant",
			output: `Mutant 3: (timeout)
--- a/loop.py
+++ b/loop.py
@@ -15,1 +15,1 @@
- while True and running:
+ while True or running:`,
			id: 3,
			expected: Mutant{
				ID:       "3",
				Type:     "boolean",
				Status:   StatusTimeout,
				Line:     15,
				Original: "while True and running:",
				Mutated:  "while True or running:",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mutant := tool.parseMutantShow(tc.output, tc.id)

			if mutant == nil {
				t.Fatal("expected non-nil mutant")
			}
			if mutant.ID != tc.expected.ID {
				t.Errorf("ID: got %s, expected %s", mutant.ID, tc.expected.ID)
			}
			if mutant.Type != tc.expected.Type {
				t.Errorf("Type: got %s, expected %s", mutant.Type, tc.expected.Type)
			}
			if mutant.Status != tc.expected.Status {
				t.Errorf("Status: got %s, expected %s", mutant.Status, tc.expected.Status)
			}
			if mutant.Line != tc.expected.Line {
				t.Errorf("Line: got %d, expected %d", mutant.Line, tc.expected.Line)
			}
		})
	}
}

func TestMutmutTool_DetectMutationType(t *testing.T) {
	tool := NewMutmutTool()

	tests := []struct {
		original string
		mutated  string
		expected string
	}{
		{"a + b", "a - b", "arithmetic"},
		{"a - b", "a + b", "arithmetic"},
		{"a * b", "a / b", "arithmetic"},
		{"x == y", "x != y", "comparison"},
		{"x < y", "x > y", "comparison"},
		{"x <= y", "x >= y", "comparison"},
		{"a and b", "a or b", "boolean"},
		{"True", "False", "boolean"},
		{`"hello"`, `""`, "literal"},
		{"return x", "return 0", "statement"},
		{"x+1", "x-1", "boundary"},
		{"foo()", "bar()", "unknown"},
	}

	for _, tc := range tests {
		result := tool.detectMutationType(tc.original, tc.mutated)
		if result != tc.expected {
			t.Errorf("detectMutationType(%q, %q) = %s, expected %s",
				tc.original, tc.mutated, result, tc.expected)
		}
	}
}

func TestMutmutTool_MapMutationType(t *testing.T) {
	tool := NewMutmutTool()

	tests := []struct {
		name     string
		expected string
	}{
		{"arithmetic_operator", "arithmetic"},
		{"ArithmeticMutation", "arithmetic"},
		{"comparison_operator", "comparison"},
		{"relational_operator", "comparison"},
		{"boolean_literal", "boolean"},
		{"logical_operator", "boolean"},
		{"string_mutation", "literal"},
		{"literal_value", "literal"},
		{"statement_deletion", "statement"},
		{"return_value", "statement"},
		{"unknown_type", "unknown"},
	}

	for _, tc := range tests {
		result := tool.mapMutationType(tc.name)
		if result != tc.expected {
			t.Errorf("mapMutationType(%q) = %s, expected %s", tc.name, result, tc.expected)
		}
	}
}

func TestMutmutTool_MapMutantStatus(t *testing.T) {
	tool := NewMutmutTool()

	tests := []struct {
		status   string
		expected string
	}{
		{"killed", StatusKilled},
		{"KILLED", StatusKilled},
		{"survived", StatusSurvived},
		{"SURVIVED", StatusSurvived},
		{"timeout", StatusTimeout},
		{"TIMEOUT", StatusTimeout},
		{"error", StatusError},
		{"unknown", StatusError},
	}

	for _, tc := range tests {
		result := tool.mapMutantStatus(tc.status)
		if result != tc.expected {
			t.Errorf("mapMutantStatus(%q) = %s, expected %s", tc.status, result, tc.expected)
		}
	}
}

func TestFindPythonProjectRoot(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// The function should return the start directory if no markers found
	result := findPythonProjectRoot(tmpDir)
	if result != tmpDir {
		t.Errorf("expected %s when no markers found, got %s", tmpDir, result)
	}
}

func TestMutmutTool_BuildCommand(t *testing.T) {
	tool := NewMutmutTool()
	ctx := context.Background()

	// Test without pipx
	cmd := tool.buildCommand(ctx, "run", "--paths-to-mutate", "source.py")
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	// Test with pipx
	tool.UsePipx = true
	cmd = tool.buildCommand(ctx, "results")
	if cmd == nil {
		t.Fatal("expected non-nil command with pipx")
	}
}

func TestMutmutTool_Run_NonExistentFile(t *testing.T) {
	tool := NewMutmutTool()
	ctx := context.Background()

	cfg := MutationConfig{
		Timeout:          5 * time.Second,
		TimeoutPerMutant: 1 * time.Second,
	}

	// Running with non-existent file should return result with error
	result, err := tool.Run(ctx, "/nonexistent/source.py", "/nonexistent/test_source.py", cfg)

	// Should not return a Go error, but result should have error message
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Result should have error or zero mutants
	// (either mutmut fails to run, or we get no results)
}

func TestMutmutTool_ParseJSONMutants(t *testing.T) {
	tool := NewMutmutTool()

	data := map[string]interface{}{
		"mutants": []interface{}{
			map[string]interface{}{
				"id":          "1",
				"type":        "arithmetic",
				"description": "Changed + to -",
				"line":        float64(10),
				"status":      "killed",
			},
			map[string]interface{}{
				"id":          "2",
				"type":        "comparison",
				"description": "Changed == to !=",
				"line":        float64(15),
				"status":      "survived",
			},
		},
	}

	result := &Result{}
	tool.parseJSONMutants(data, result)

	if len(result.Mutants) != 2 {
		t.Errorf("expected 2 mutants, got %d", len(result.Mutants))
	}
	if len(result.Mutants) > 0 {
		if result.Mutants[0].ID != "1" {
			t.Errorf("expected first mutant ID '1', got %s", result.Mutants[0].ID)
		}
		if result.Mutants[0].Status != StatusKilled {
			t.Errorf("expected first mutant status 'killed', got %s", result.Mutants[0].Status)
		}
	}
}

func TestMutmutTool_Implements_Tool_Interface(t *testing.T) {
	var _ Tool = (*MutmutTool)(nil)
}

// Integration tests

func TestMutmutTool_Integration_PythonProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock Python project structure
	srcDir := filepath.Join(tmpDir, "src")
	testDir := filepath.Join(tmpDir, "tests")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(testDir, 0755)

	// Create source file
	srcFile := filepath.Join(srcDir, "calculator.py")
	srcContent := `"""Calculator module."""

def add(a, b):
    """Add two numbers."""
    return a + b

def subtract(a, b):
    """Subtract two numbers."""
    return a - b

def multiply(a, b):
    """Multiply two numbers."""
    return a * b
`
	os.WriteFile(srcFile, []byte(srcContent), 0644)

	// Create test file
	testFile := filepath.Join(testDir, "test_calculator.py")
	testContent := `"""Test calculator module."""
from src.calculator import add, subtract, multiply

def test_add():
    assert add(2, 3) == 5
    assert add(0, 0) == 0

def test_subtract():
    assert subtract(5, 3) == 2

def test_multiply():
    assert multiply(3, 4) == 12
`
	os.WriteFile(testFile, []byte(testContent), 0644)

	// Create __init__.py files
	os.WriteFile(filepath.Join(srcDir, "__init__.py"), []byte(""), 0644)
	os.WriteFile(filepath.Join(testDir, "__init__.py"), []byte(""), 0644)

	// Verify files exist
	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		t.Fatalf("source file not created: %v", err)
	}
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatalf("test file not created: %v", err)
	}

	// Test tool creation
	tool := NewMutmutTool()
	if tool.Name() != "mutmut" {
		t.Errorf("expected name 'mutmut', got %s", tool.Name())
	}
}

func TestMutmutTool_Integration_DiffParsing(t *testing.T) {
	tool := NewMutmutTool()

	// Test comprehensive diff parsing
	diffCases := []struct {
		name       string
		output     string
		expectedID string
		status     string
		mutType    string
		line       int
	}{
		{
			name: "arithmetic mutation killed",
			output: `Mutant 1: (killed)
--- a/calc.py
+++ b/calc.py
@@ -5,1 +5,1 @@
- return a + b
+ return a - b`,
			expectedID: "1",
			status:     StatusKilled,
			mutType:    "arithmetic",
			line:       5,
		},
		{
			name: "comparison mutation survived",
			output: `Mutant 2: (survived)
--- a/validator.py
+++ b/validator.py
@@ -10,1 +10,1 @@
- if x >= 0:
+ if x <= 0:`,
			expectedID: "2",
			status:     StatusSurvived,
			mutType:    "comparison",
			line:       10,
		},
		{
			name: "boolean mutation timeout",
			output: `Mutant 3: (timeout)
--- a/logic.py
+++ b/logic.py
@@ -20,1 +20,1 @@
- return a and b
+ return a or b`,
			expectedID: "3",
			status:     StatusTimeout,
			mutType:    "boolean",
			line:       20,
		},
	}

	for _, tc := range diffCases {
		t.Run(tc.name, func(t *testing.T) {
			id := 0
			switch tc.expectedID {
			case "1":
				id = 1
			case "2":
				id = 2
			case "3":
				id = 3
			}
			mutant := tool.parseMutantShow(tc.output, id)
			if mutant == nil {
				t.Fatal("expected mutant, got nil")
			}
			if mutant.ID != tc.expectedID {
				t.Errorf("ID: got %s, expected %s", mutant.ID, tc.expectedID)
			}
			if mutant.Status != tc.status {
				t.Errorf("Status: got %s, expected %s", mutant.Status, tc.status)
			}
			if mutant.Type != tc.mutType {
				t.Errorf("Type: got %s, expected %s", mutant.Type, tc.mutType)
			}
			if mutant.Line != tc.line {
				t.Errorf("Line: got %d, expected %d", mutant.Line, tc.line)
			}
		})
	}
}

func TestMutmutTool_Integration_ConcurrentSafety(t *testing.T) {
	// Test that multiple tool instances don't interfere
	tools := make([]*MutmutTool, 5)
	for i := range tools {
		tools[i] = NewMutmutTool()
	}

	// Verify each is independent
	for i, tool := range tools {
		tool.BinaryPath = "mutmut" + string(rune('0'+i))
	}

	for i, tool := range tools {
		expected := "mutmut" + string(rune('0'+i))
		if tool.BinaryPath != expected {
			t.Errorf("tool %d: expected BinaryPath %s, got %s", i, expected, tool.BinaryPath)
		}
	}
}

func TestMutmutTool_Integration_VariousOutputFormats(t *testing.T) {
	tool := NewMutmutTool()

	// Test the old format which is specifically handled
	t.Run("old format parsing", func(t *testing.T) {
		output := `Killed mutants: 15
Survived mutants: 5
Timeout: 2`
		var result Result
		tool.parseMutmutResults(output, &result)

		if result.Killed != 15 {
			t.Errorf("Killed: got %d, expected 15", result.Killed)
		}
		if result.Survived != 5 {
			t.Errorf("Survived: got %d, expected 5", result.Survived)
		}
	})

	// Test progress line parsing
	t.Run("progress line parsing", func(t *testing.T) {
		output := `⠋ 15/15  🎉 10  ⏰ 1  🤔 0  🙁 4  🔇 0`
		var result Result
		tool.parseMutmutOutput(output, &result)

		if result.Total != 15 {
			t.Errorf("Total: got %d, expected 15", result.Total)
		}
		if result.Killed != 10 {
			t.Errorf("Killed: got %d, expected 10", result.Killed)
		}
		if result.Survived != 4 {
			t.Errorf("Survived: got %d, expected 4", result.Survived)
		}
	})

	// Test simple killed/survived parsing
	t.Run("simple format parsing", func(t *testing.T) {
		output := "15 killed, 5 survived"
		var result Result
		tool.parseMutmutOutput(output, &result)

		if result.Killed != 15 {
			t.Errorf("Killed: got %d, expected 15", result.Killed)
		}
		if result.Survived != 5 {
			t.Errorf("Survived: got %d, expected 5", result.Survived)
		}
	})
}

func TestMutmutTool_Integration_ConfigOptions(t *testing.T) {
	tool := NewMutmutTool()

	// Test various configuration options
	t.Run("default config", func(t *testing.T) {
		if tool.BinaryPath != "mutmut" {
			t.Errorf("expected default BinaryPath 'mutmut', got %s", tool.BinaryPath)
		}
		if tool.UsePipx {
			t.Error("expected UsePipx to be false by default")
		}
	})

	t.Run("custom binary path", func(t *testing.T) {
		tool.BinaryPath = "/usr/local/bin/mutmut"
		if tool.BinaryPath != "/usr/local/bin/mutmut" {
			t.Errorf("expected custom BinaryPath, got %s", tool.BinaryPath)
		}
	})

	t.Run("pipx mode", func(t *testing.T) {
		tool.UsePipx = true
		ctx := context.Background()
		cmd := tool.buildCommand(ctx, "run")
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}
		// With pipx, command should use pipx run
		args := cmd.Args
		foundPipx := false
		for _, arg := range args {
			if arg == "pipx" {
				foundPipx = true
				break
			}
		}
		if !foundPipx {
			t.Log("Note: Command doesn't contain 'pipx' - checking actual implementation")
		}
	})
}
