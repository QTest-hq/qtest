package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/flakiness"
)

func TestDefaultValidatorConfig(t *testing.T) {
	config := DefaultValidatorConfig()

	if config.FlakinessRuns != 3 {
		t.Errorf("Expected FlakinessRuns=3, got %d", config.FlakinessRuns)
	}
	if config.FlakinessThreshold != 0.2 {
		t.Errorf("Expected FlakinessThreshold=0.2, got %f", config.FlakinessThreshold)
	}
	if config.StabilityRuns != 3 {
		t.Errorf("Expected StabilityRuns=3, got %d", config.StabilityRuns)
	}
	if config.Timeout != 5*time.Minute {
		t.Errorf("Expected Timeout=5m, got %v", config.Timeout)
	}
	if config.GlobalTimeout != 30*time.Minute {
		t.Errorf("Expected GlobalTimeout=30m, got %v", config.GlobalTimeout)
	}
	if !config.ScreenshotsOnFailure {
		t.Error("Expected ScreenshotsOnFailure=true")
	}
	if !config.TraceOnFailure {
		t.Error("Expected TraceOnFailure=true")
	}
	if !config.CleanupOnTimeout {
		t.Error("Expected CleanupOnTimeout=true")
	}
}

func TestNewTestValidator(t *testing.T) {
	runner := NewTestRunner(nil)
	validator := NewTestValidator(runner, nil)

	if validator == nil {
		t.Fatal("Expected non-nil validator")
	}
	if validator.runner != runner {
		t.Error("Expected runner to be set")
	}
	if validator.flakinessTracker == nil {
		t.Error("Expected flakiness tracker to be initialized")
	}
	if validator.config == nil {
		t.Error("Expected config to be initialized")
	}
}

func TestNewTestValidatorWithConfig(t *testing.T) {
	runner := NewTestRunner(nil)
	config := &ValidatorConfig{
		FlakinessRuns:      5,
		FlakinessThreshold: 0.3,
		StabilityRuns:      5,
		Timeout:            10 * time.Minute,
		GlobalTimeout:      60 * time.Minute,
	}

	validator := NewTestValidator(runner, config)

	if validator.config.FlakinessRuns != 5 {
		t.Errorf("Expected FlakinessRuns=5, got %d", validator.config.FlakinessRuns)
	}
	if validator.config.FlakinessThreshold != 0.3 {
		t.Errorf("Expected FlakinessThreshold=0.3, got %f", validator.config.FlakinessThreshold)
	}
}

func TestTimeoutError(t *testing.T) {
	err := &TimeoutError{
		Attempt:  1,
		Duration: 5 * time.Minute,
		Reason:   "context deadline exceeded",
	}

	expected := "test run 1 timed out after 5m0s: context deadline exceeded"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout error",
			err:      &TimeoutError{Attempt: 1, Duration: time.Minute, Reason: "test"},
			expected: true,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimeoutError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDefaultTimeoutConfig(t *testing.T) {
	config := DefaultTimeoutConfig()

	if config.ActionTimeout != 30*time.Second {
		t.Errorf("Expected ActionTimeout=30s, got %v", config.ActionTimeout)
	}
	if config.NavigationTimeout != 30*time.Second {
		t.Errorf("Expected NavigationTimeout=30s, got %v", config.NavigationTimeout)
	}
	if config.ExpectTimeout != 5*time.Second {
		t.Errorf("Expected ExpectTimeout=5s, got %v", config.ExpectTimeout)
	}
	if config.TestTimeout != 2*time.Minute {
		t.Errorf("Expected TestTimeout=2m, got %v", config.TestTimeout)
	}
	if config.GlobalTimeout != 30*time.Minute {
		t.Errorf("Expected GlobalTimeout=30m, got %v", config.GlobalTimeout)
	}
}

func TestTimeoutHandler_StartAndCancelTimeout(t *testing.T) {
	handler := NewTimeoutHandler(nil)

	called := false
	handler.StartTimeout("test-1", 100*time.Millisecond, func() {
		called = true
	})

	// Cancel before timeout
	handler.CancelTimeout("test-1")

	// Wait to ensure callback wasn't called
	time.Sleep(150 * time.Millisecond)

	if called {
		t.Error("Callback should not have been called after cancel")
	}
}

func TestTimeoutHandler_TimeoutTriggered(t *testing.T) {
	handler := NewTimeoutHandler(nil)

	called := make(chan bool, 1)
	handler.StartTimeout("test-1", 50*time.Millisecond, func() {
		called <- true
	})

	select {
	case <-called:
		// Success
	case <-time.After(200 * time.Millisecond):
		t.Error("Timeout callback was not triggered")
	}
}

func TestTimeoutHandler_WithTimeout(t *testing.T) {
	handler := NewTimeoutHandler(nil)

	// Test successful completion
	err := handler.WithTimeout(context.Background(), 1*time.Second, func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test timeout
	err = handler.WithTimeout(context.Background(), 50*time.Millisecond, func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	if err == nil {
		t.Error("Expected timeout error")
	}
	if !isTimeoutError(err) {
		t.Errorf("Expected timeout error type, got %T", err)
	}
}

func TestSeverityFromRate(t *testing.T) {
	tests := []struct {
		rate     float64
		expected string
	}{
		{0.6, "high"},
		{0.5, "medium"},
		{0.3, "medium"},
		{0.2, "low"},
		{0.1, "low"},
	}

	for _, tt := range tests {
		result := severityFromRate(tt.rate)
		if result != tt.expected {
			t.Errorf("severityFromRate(%f) = %s, want %s", tt.rate, result, tt.expected)
		}
	}
}

func TestTestValidator_GetFlakinessTracker(t *testing.T) {
	runner := NewTestRunner(nil)
	validator := NewTestValidator(runner, nil)

	tracker := validator.GetFlakinessTracker()
	if tracker == nil {
		t.Error("Expected non-nil tracker")
	}
}

func TestValidationResult_Fields(t *testing.T) {
	result := &ValidationResult{
		Success:       true,
		TotalTests:    10,
		StableTests:   8,
		FlakyTests:    1,
		FailedTests:   1,
		TimedOutTests: 0,
		Duration:      5 * time.Minute,
	}

	if !result.Success {
		t.Error("Expected Success=true")
	}
	if result.TotalTests != 10 {
		t.Errorf("Expected TotalTests=10, got %d", result.TotalTests)
	}
	if result.StableTests != 8 {
		t.Errorf("Expected StableTests=8, got %d", result.StableTests)
	}
	if result.FlakyTests != 1 {
		t.Errorf("Expected FlakyTests=1, got %d", result.FlakyTests)
	}
}

func TestTestValidationResult_Fields(t *testing.T) {
	score := &flakiness.FlakinessScore{
		TestID:         "test-1",
		Score:          0.15,
		Classification: "flaky",
	}

	result := &TestValidationResult{
		TestName:        "Test Login Flow",
		Stable:          false,
		FlakinessScore:  score,
		PassCount:       2,
		FailCount:       1,
		TimeoutCount:    0,
		AverageDuration: 5 * time.Second,
		Errors:          []string{"Run 3: element not found"},
	}

	if result.TestName != "Test Login Flow" {
		t.Errorf("Expected TestName='Test Login Flow', got %s", result.TestName)
	}
	if result.Stable {
		t.Error("Expected Stable=false")
	}
	if result.PassCount != 2 {
		t.Errorf("Expected PassCount=2, got %d", result.PassCount)
	}
	if result.FailCount != 1 {
		t.Errorf("Expected FailCount=1, got %d", result.FailCount)
	}
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}
}

func TestStabilityReport_Fields(t *testing.T) {
	report := &StabilityReport{
		TestName:   "Test Checkout",
		IsStable:   false,
		Confidence: 0.7,
		Issues: []StabilityIssue{
			{
				Type:        "flaky",
				Description: "20% failure rate across 5 runs",
				Severity:    "low",
				Occurrences: 1,
			},
		},
		Recommendations: []string{"Investigate race conditions"},
	}

	if report.TestName != "Test Checkout" {
		t.Errorf("Expected TestName='Test Checkout', got %s", report.TestName)
	}
	if report.IsStable {
		t.Error("Expected IsStable=false")
	}
	if report.Confidence != 0.7 {
		t.Errorf("Expected Confidence=0.7, got %f", report.Confidence)
	}
	if len(report.Issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(report.Issues))
	}
	if len(report.Recommendations) != 1 {
		t.Errorf("Expected 1 recommendation, got %d", len(report.Recommendations))
	}
}

func TestStabilityIssue_Fields(t *testing.T) {
	issue := StabilityIssue{
		Type:        "timeout",
		Description: "Test timed out 3 times",
		Severity:    "high",
		Occurrences: 3,
	}

	if issue.Type != "timeout" {
		t.Errorf("Expected Type='timeout', got %s", issue.Type)
	}
	if issue.Severity != "high" {
		t.Errorf("Expected Severity='high', got %s", issue.Severity)
	}
	if issue.Occurrences != 3 {
		t.Errorf("Expected Occurrences=3, got %d", issue.Occurrences)
	}
}

func TestTestValidator_GenerateStabilityReport_NoHistory(t *testing.T) {
	runner := NewTestRunner(nil)
	validator := NewTestValidator(runner, nil)

	report := validator.GenerateStabilityReport("nonexistent-test")

	if report == nil {
		t.Fatal("Expected non-nil report")
	}
	if report.TestName != "nonexistent-test" {
		t.Errorf("Expected TestName='nonexistent-test', got %s", report.TestName)
	}
	if len(report.Recommendations) == 0 {
		t.Error("Expected recommendations for test with no history")
	}
}

func TestTestValidator_GenerateRecommendations(t *testing.T) {
	runner := NewTestRunner(nil)
	validator := NewTestValidator(runner, nil)

	issues := []StabilityIssue{
		{Type: "timeout"},
		{Type: "selector"},
		{Type: "network"},
		{Type: "assertion"},
		{Type: "flaky"},
		{Type: "inconsistent"},
	}

	recs := validator.generateRecommendations(issues, nil)

	if len(recs) != 6 {
		t.Errorf("Expected 6 recommendations, got %d", len(recs))
	}

	// Test with highly flaky score
	score := &flakiness.FlakinessScore{
		Classification: "highly_flaky",
	}
	recs = validator.generateRecommendations(issues, score)

	if len(recs) != 7 {
		t.Errorf("Expected 7 recommendations (including quarantine), got %d", len(recs))
	}
}

func TestTimeoutHandler_ReplaceTimeout(t *testing.T) {
	handler := NewTimeoutHandler(nil)

	called1 := false
	called2 := false

	// Start first timeout
	handler.StartTimeout("test-1", 100*time.Millisecond, func() {
		called1 = true
	})

	// Replace with new timeout
	handler.StartTimeout("test-1", 50*time.Millisecond, func() {
		called2 = true
	})

	// Wait for both potential timeouts
	time.Sleep(150 * time.Millisecond)

	if called1 {
		t.Error("First callback should have been cancelled")
	}
	if !called2 {
		t.Error("Second callback should have been called")
	}
}

func TestCategorizeFailure_Validator(t *testing.T) {
	tests := []struct {
		errorMsg string
		expected string
	}{
		{"Test timed out waiting for element", "timeout"},
		{"Timeout exceeded", "timeout"},
		{"expect(value).toEqual(5)", "assertion"},
		{"Assertion failed: expected 5, got 3", "assertion"},
		{"Could not find selector #button", "selector"},
		{"Locator not found", "selector"},
		{"Network request failed", "network"},
		{"Connection refused", "network"},
		{"Unknown error occurred", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.errorMsg, func(t *testing.T) {
			result := categorizeFailure(tt.errorMsg)
			if result != tt.expected {
				t.Errorf("categorizeFailure(%q) = %s, want %s", tt.errorMsg, result, tt.expected)
			}
		})
	}
}

func TestRunResult_Success(t *testing.T) {
	// Test successful run
	result := &RunResult{
		TotalTests: 5,
		Passed:     5,
		Failed:     0,
		Skipped:    0,
	}
	// Success is computed in RunTests, simulate here
	result.Success = result.Failed == 0 && len(result.Errors) == 0

	if !result.Success {
		t.Error("Expected Success=true for all passing tests")
	}

	// Test failed run
	result2 := &RunResult{
		TotalTests: 5,
		Passed:     3,
		Failed:     2,
		Skipped:    0,
	}
	result2.Success = result2.Failed == 0 && len(result2.Errors) == 0

	if result2.Success {
		t.Error("Expected Success=false for failing tests")
	}
}

func TestValidationResult_SuccessLogic(t *testing.T) {
	tests := []struct {
		name          string
		flakyTests    int
		failedTests   int
		timedOutTests int
		expectedOK    bool
	}{
		{
			name:          "all stable",
			flakyTests:    0,
			failedTests:   0,
			timedOutTests: 0,
			expectedOK:    true,
		},
		{
			name:          "has flaky tests",
			flakyTests:    1,
			failedTests:   0,
			timedOutTests: 0,
			expectedOK:    false,
		},
		{
			name:          "has failed tests",
			flakyTests:    0,
			failedTests:   1,
			timedOutTests: 0,
			expectedOK:    false,
		},
		{
			name:          "has timed out tests",
			flakyTests:    0,
			failedTests:   0,
			timedOutTests: 1,
			expectedOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{
				FlakyTests:    tt.flakyTests,
				FailedTests:   tt.failedTests,
				TimedOutTests: tt.timedOutTests,
			}
			result.Success = result.FlakyTests == 0 && result.FailedTests == 0 && result.TimedOutTests == 0

			if result.Success != tt.expectedOK {
				t.Errorf("Expected Success=%v, got %v", tt.expectedOK, result.Success)
			}
		})
	}
}

func TestSeverityFromCount(t *testing.T) {
	tests := []struct {
		count    int
		total    int
		expected string
	}{
		{6, 10, "high"},    // 60%
		{5, 10, "medium"},  // 50%
		{3, 10, "medium"},  // 30%
		{2, 10, "low"},     // 20%
		{1, 10, "low"},     // 10%
	}

	for _, tt := range tests {
		result := severityFromCount(tt.count, tt.total)
		if result != tt.expected {
			t.Errorf("severityFromCount(%d, %d) = %s, want %s", tt.count, tt.total, result, tt.expected)
		}
	}
}

// TestAnalyzeResults tests the result analysis logic
func TestAnalyzeResults_ClassifyTests(t *testing.T) {
	runner := NewTestRunner(nil)
	validator := NewTestValidator(runner, nil)

	result := &ValidationResult{
		Runs: []*RunResult{
			{
				Tests: []TestResult{
					{Name: "stable-test", Status: "passed"},
					{Name: "flaky-test", Status: "passed"},
					{Name: "failing-test", Status: "failed", Error: "error"},
				},
			},
			{
				Tests: []TestResult{
					{Name: "stable-test", Status: "passed"},
					{Name: "flaky-test", Status: "failed", Error: "intermittent"},
					{Name: "failing-test", Status: "failed", Error: "error"},
				},
			},
			{
				Tests: []TestResult{
					{Name: "stable-test", Status: "passed"},
					{Name: "flaky-test", Status: "passed"},
					{Name: "failing-test", Status: "failed", Error: "error"},
				},
			},
		},
	}

	validator.analyzeResults(result)

	if result.TotalTests != 3 {
		t.Errorf("Expected TotalTests=3, got %d", result.TotalTests)
	}
	if result.StableTests != 1 {
		t.Errorf("Expected StableTests=1, got %d", result.StableTests)
	}
	if result.FlakyTests != 1 {
		t.Errorf("Expected FlakyTests=1, got %d", result.FlakyTests)
	}
	if result.FailedTests != 1 {
		t.Errorf("Expected FailedTests=1, got %d", result.FailedTests)
	}
}

// Test that validator properly handles nil runs in the list
func TestAnalyzeResults_WithNilRuns(t *testing.T) {
	runner := NewTestRunner(nil)
	validator := NewTestValidator(runner, nil)

	result := &ValidationResult{
		Runs: []*RunResult{
			{
				Tests: []TestResult{
					{Name: "test-1", Status: "passed"},
				},
			},
			nil, // Nil run (e.g., from timeout)
			{
				Tests: []TestResult{
					{Name: "test-1", Status: "passed"},
				},
			},
		},
	}

	// Should not panic
	validator.analyzeResults(result)

	if result.TotalTests != 1 {
		t.Errorf("Expected TotalTests=1, got %d", result.TotalTests)
	}
}
