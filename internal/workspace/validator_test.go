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
