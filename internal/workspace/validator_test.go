package workspace

import (
	"testing"
	"time"
)

func TestNewTestValidator(t *testing.T) {
	ws := &Workspace{
		path: "/tmp/test-workspace",
	}

	validator := NewTestValidator(ws)

	if validator == nil {
		t.Fatal("NewTestValidator() returned nil")
	}
	if validator.ws != ws {
		t.Error("ws reference mismatch")
	}
	if validator.artifacts == nil {
		t.Error("artifacts should not be nil")
	}
}

func TestValidationResult_Fields(t *testing.T) {
	result := ValidationResult{
		TestFile:  "test_file.go",
		Target:    "TestFunc",
		Passed:    true,
		Output:    "PASS\n",
		Duration:  time.Second * 5,
		TestCount: 10,
		PassCount: 9,
		FailCount: 1,
		SkipCount: 0,
	}

	if result.TestFile != "test_file.go" {
		t.Errorf("TestFile = %s, want test_file.go", result.TestFile)
	}
	if result.Target != "TestFunc" {
		t.Errorf("Target = %s, want TestFunc", result.Target)
	}
	if !result.Passed {
		t.Error("Passed should be true")
	}
	if result.TestCount != 10 {
		t.Errorf("TestCount = %d, want 10", result.TestCount)
	}
	if result.PassCount != 9 {
		t.Errorf("PassCount = %d, want 9", result.PassCount)
	}
}

func TestValidationResult_WithError(t *testing.T) {
	result := ValidationResult{
		TestFile: "test_file.go",
		Passed:   false,
		Error:    "tests failed (exit code 1)",
	}

	if result.Passed {
		t.Error("Passed should be false")
	}
	if result.Error == "" {
		t.Error("Error should not be empty")
	}
}

func TestValidateSummary_Fields(t *testing.T) {
	summary := ValidateSummary{
		Total:       100,
		Passed:      90,
		Failed:      8,
		Skipped:     2,
		Duration:    time.Minute * 5,
		PassRate:    90.0,
		FailedTests: []string{"test1.go", "test2.go"},
	}

	if summary.Total != 100 {
		t.Errorf("Total = %d, want 100", summary.Total)
	}
	if summary.PassRate != 90.0 {
		t.Errorf("PassRate = %f, want 90.0", summary.PassRate)
	}
	if len(summary.FailedTests) != 2 {
		t.Errorf("len(FailedTests) = %d, want 2", len(summary.FailedTests))
	}
}

func TestSummarize(t *testing.T) {
	results := []ValidationResult{
		{TestFile: "test1.go", Passed: true, Duration: time.Second},
		{TestFile: "test2.go", Passed: true, Duration: time.Second},
		{TestFile: "test3.go", Passed: false, Duration: time.Second},
	}

	summary := Summarize(results)

	if summary.Total != 3 {
		t.Errorf("Total = %d, want 3", summary.Total)
	}
	if summary.Passed != 2 {
		t.Errorf("Passed = %d, want 2", summary.Passed)
	}
	if summary.Failed != 1 {
		t.Errorf("Failed = %d, want 1", summary.Failed)
	}
	if summary.Duration != time.Second*3 {
		t.Errorf("Duration = %v, want 3s", summary.Duration)
	}
	// PassRate should be 2/3 * 100 = 66.67
	expectedRate := float64(2) / float64(3) * 100
	if summary.PassRate != expectedRate {
		t.Errorf("PassRate = %f, want %f", summary.PassRate, expectedRate)
	}
	if len(summary.FailedTests) != 1 {
		t.Errorf("len(FailedTests) = %d, want 1", len(summary.FailedTests))
	}
	if summary.FailedTests[0] != "test3.go" {
		t.Errorf("FailedTests[0] = %s, want test3.go", summary.FailedTests[0])
	}
}

func TestSummarize_Empty(t *testing.T) {
	results := []ValidationResult{}

	summary := Summarize(results)

	if summary.Total != 0 {
		t.Errorf("Total = %d, want 0", summary.Total)
	}
	if summary.PassRate != 0 {
		t.Errorf("PassRate = %f, want 0", summary.PassRate)
	}
}

func TestSummarize_AllPassed(t *testing.T) {
	results := []ValidationResult{
		{TestFile: "test1.go", Passed: true},
		{TestFile: "test2.go", Passed: true},
	}

	summary := Summarize(results)

	if summary.PassRate != 100 {
		t.Errorf("PassRate = %f, want 100", summary.PassRate)
	}
	if len(summary.FailedTests) != 0 {
		t.Errorf("len(FailedTests) = %d, want 0", len(summary.FailedTests))
	}
}

func TestSummarize_AllFailed(t *testing.T) {
	results := []ValidationResult{
		{TestFile: "test1.go", Passed: false},
		{TestFile: "test2.go", Passed: false},
	}

	summary := Summarize(results)

	if summary.PassRate != 0 {
		t.Errorf("PassRate = %f, want 0", summary.PassRate)
	}
	if summary.Failed != 2 {
		t.Errorf("Failed = %d, want 2", summary.Failed)
	}
}

func TestExtractTestName(t *testing.T) {
	tests := []struct {
		testFile string
		want     string
	}{
		{"add_test.go", "TestAdd"},
		{"math_utils_test.go", "TestMathUtils"},
		{"user_service_test.go", "TestUserService"},
		{"simple_test.go", "TestSimple"},
	}

	for _, tt := range tests {
		t.Run(tt.testFile, func(t *testing.T) {
			got := extractTestName(tt.testFile)
			if got != tt.want {
				t.Errorf("extractTestName(%s) = %s, want %s", tt.testFile, got, tt.want)
			}
		})
	}
}

func TestNewTestValidatorWithDocker_Disabled(t *testing.T) {
	ws := &Workspace{
		path: "/tmp/test-workspace",
	}

	// Docker disabled
	validator := NewTestValidatorWithDocker(ws, &DockerConfig{Enabled: false})

	if validator == nil {
		t.Fatal("NewTestValidatorWithDocker() returned nil")
	}
	if validator.useDocker {
		t.Error("useDocker should be false when DockerConfig.Enabled is false")
	}
}

func TestNewTestValidatorWithDocker_NilConfig(t *testing.T) {
	ws := &Workspace{
		path: "/tmp/test-workspace",
	}

	// Nil config should not panic
	validator := NewTestValidatorWithDocker(ws, nil)

	if validator == nil {
		t.Fatal("NewTestValidatorWithDocker() returned nil")
	}
	if validator.useDocker {
		t.Error("useDocker should be false when config is nil")
	}
}

func TestTestValidator_extToLanguage(t *testing.T) {
	v := &TestValidator{}

	tests := []struct {
		ext  string
		want string
	}{
		{".go", "go"},
		{".py", "python"},
		{".js", "javascript"},
		{".ts", "typescript"},
		{".java", "java"},
		{".rb", ""},
		{".rs", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := v.extToLanguage(tt.ext)
			if got != tt.want {
				t.Errorf("extToLanguage(%s) = %s, want %s", tt.ext, got, tt.want)
			}
		})
	}
}

func TestTestValidator_parseGoTestOutput(t *testing.T) {
	v := &TestValidator{}

	// Sample Go test -json output
	output := `{"Action":"run","Package":"pkg/example","Test":"TestAdd"}
{"Action":"output","Package":"pkg/example","Test":"TestAdd","Output":"=== RUN   TestAdd\n"}
{"Action":"pass","Package":"pkg/example","Test":"TestAdd"}
{"Action":"run","Package":"pkg/example","Test":"TestSubtract"}
{"Action":"pass","Package":"pkg/example","Test":"TestSubtract"}
{"Action":"run","Package":"pkg/example","Test":"TestMultiply"}
{"Action":"fail","Package":"pkg/example","Test":"TestMultiply"}
{"Action":"run","Package":"pkg/example","Test":"TestDivide"}
{"Action":"skip","Package":"pkg/example","Test":"TestDivide"}
`
	result := &ValidationResult{}
	v.parseGoTestOutput(output, result)

	if result.TestCount != 4 {
		t.Errorf("TestCount = %d, want 4", result.TestCount)
	}
	if result.PassCount != 2 {
		t.Errorf("PassCount = %d, want 2", result.PassCount)
	}
	if result.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", result.FailCount)
	}
	if result.SkipCount != 1 {
		t.Errorf("SkipCount = %d, want 1", result.SkipCount)
	}
}

func TestTestValidator_parseGoTestOutput_Empty(t *testing.T) {
	v := &TestValidator{}
	result := &ValidationResult{}

	v.parseGoTestOutput("", result)

	if result.TestCount != 0 {
		t.Errorf("TestCount = %d, want 0", result.TestCount)
	}
}

func TestTestValidator_parseGoTestOutput_InvalidJSON(t *testing.T) {
	v := &TestValidator{}
	result := &ValidationResult{}

	// Invalid JSON should be skipped
	v.parseGoTestOutput("not valid json\n{invalid}", result)

	if result.TestCount != 0 {
		t.Errorf("TestCount = %d, want 0", result.TestCount)
	}
}

func TestTestValidator_parsePytestOutput(t *testing.T) {
	v := &TestValidator{}

	output := `============================= test session starts ==============================
collected 10 items

test_math.py::test_add PASSED
test_math.py::test_subtract PASSED
test_math.py::test_multiply FAILED
test_math.py::test_divide SKIPPED

============================= short test summary info =============================
FAILED test_math.py::test_multiply - AssertionError
====================== 5 passed, 2 failed, 3 skipped in 0.15s =====================
`
	result := &ValidationResult{}
	v.parsePytestOutput(output, result)

	if result.PassCount != 5 {
		t.Errorf("PassCount = %d, want 5", result.PassCount)
	}
	if result.FailCount != 2 {
		t.Errorf("FailCount = %d, want 2", result.FailCount)
	}
	if result.SkipCount != 3 {
		t.Errorf("SkipCount = %d, want 3", result.SkipCount)
	}
	if result.TestCount != 10 {
		t.Errorf("TestCount = %d, want 10", result.TestCount)
	}
}

func TestTestValidator_parsePytestOutput_AllPassed(t *testing.T) {
	v := &TestValidator{}

	output := `============================= test session starts ==============================
====================== 8 passed in 0.10s =====================
`
	result := &ValidationResult{}
	v.parsePytestOutput(output, result)

	if result.PassCount != 8 {
		t.Errorf("PassCount = %d, want 8", result.PassCount)
	}
	if result.TestCount != 8 {
		t.Errorf("TestCount = %d, want 8", result.TestCount)
	}
}

func TestTestValidator_parseJestOutput(t *testing.T) {
	v := &TestValidator{}

	output := `PASS  src/__tests__/math.test.js
  ✓ adds 1 + 2 to equal 3 (2ms)
  ✓ subtracts 5 - 3 to equal 2 (1ms)
  ✕ multiplies 2 * 3 to equal 6 (3ms)

Tests: 2 passed, 1 failed, 1 skipped, 4 total
Snapshots:   0 total
Time:        1.234s
`
	result := &ValidationResult{}
	v.parseJestOutput(output, result)

	if result.PassCount != 2 {
		t.Errorf("PassCount = %d, want 2", result.PassCount)
	}
	if result.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", result.FailCount)
	}
	if result.SkipCount != 1 {
		t.Errorf("SkipCount = %d, want 1", result.SkipCount)
	}
	if result.TestCount != 4 {
		t.Errorf("TestCount = %d, want 4", result.TestCount)
	}
}

func TestTestValidator_parseJestOutput_AllPassed(t *testing.T) {
	v := &TestValidator{}

	output := `PASS  src/__tests__/math.test.js
Tests: 5 passed, 5 total
`
	result := &ValidationResult{}
	v.parseJestOutput(output, result)

	if result.PassCount != 5 {
		t.Errorf("PassCount = %d, want 5", result.PassCount)
	}
	if result.TestCount != 5 {
		t.Errorf("TestCount = %d, want 5", result.TestCount)
	}
}

func TestTestValidator_parseTestOutput_ByExtension(t *testing.T) {
	v := &TestValidator{}

	tests := []struct {
		ext    string
		output string
		want   int // expected pass count
	}{
		{".go", `{"Action":"pass","Test":"Test1"}`, 1},
		{".py", "3 passed", 3},
		{".js", "Tests: 5 passed", 5},
		{".ts", "Tests: 7 passed", 7},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := &ValidationResult{}
			v.parseTestOutput(tt.ext, tt.output, result)

			if result.PassCount != tt.want {
				t.Errorf("parseTestOutput(%s) PassCount = %d, want %d", tt.ext, result.PassCount, tt.want)
			}
		})
	}
}

func TestTestValidator_parseTestOutput_UnknownExtension(t *testing.T) {
	v := &TestValidator{}
	result := &ValidationResult{}

	// Unknown extension should not crash
	v.parseTestOutput(".unknown", "some output", result)

	// Should remain zero
	if result.PassCount != 0 {
		t.Errorf("PassCount = %d, want 0 for unknown extension", result.PassCount)
	}
}
