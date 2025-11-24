package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/QTest-hq/qtest/internal/flakiness"
)

// TestValidator validates E2E tests by running them multiple times
// and detecting flakiness.
type TestValidator struct {
	runner         *TestRunner
	flakinessTracker *flakiness.Tracker
	config         *ValidatorConfig
}

// ValidatorConfig configures the test validator.
type ValidatorConfig struct {
	// FlakinessRuns is the number of times to run tests for flakiness detection
	FlakinessRuns int `json:"flakinessRuns"`
	// FlakinessThreshold is the failure rate threshold to mark as flaky (0-1)
	FlakinessThreshold float64 `json:"flakinessThreshold"`
	// StabilityRuns is the minimum runs needed to determine stability
	StabilityRuns int `json:"stabilityRuns"`
	// Timeout is the maximum time for each test run
	Timeout time.Duration `json:"timeout"`
	// GlobalTimeout is the maximum time for all validation runs
	GlobalTimeout time.Duration `json:"globalTimeout"`
	// ParallelRuns runs flakiness tests in parallel
	ParallelRuns bool `json:"parallelRuns"`
	// ScreenshotsOnFailure captures screenshots on test failures
	ScreenshotsOnFailure bool `json:"screenshotsOnFailure"`
	// TraceOnFailure captures traces on test failures
	TraceOnFailure bool `json:"traceOnFailure"`
	// AbortOnTimeout immediately stops remaining tests on timeout
	AbortOnTimeout bool `json:"abortOnTimeout"`
	// CleanupOnTimeout runs cleanup even when tests timeout
	CleanupOnTimeout bool `json:"cleanupOnTimeout"`
}

// DefaultValidatorConfig returns default validator configuration.
func DefaultValidatorConfig() *ValidatorConfig {
	return &ValidatorConfig{
		FlakinessRuns:        3,
		FlakinessThreshold:   0.2, // 20% failure rate = flaky
		StabilityRuns:        3,
		Timeout:              5 * time.Minute,
		GlobalTimeout:        30 * time.Minute,
		ParallelRuns:         false,
		ScreenshotsOnFailure: true,
		TraceOnFailure:       true,
		AbortOnTimeout:       false,
		CleanupOnTimeout:     true,
	}
}

// NewTestValidator creates a new test validator.
func NewTestValidator(runner *TestRunner, config *ValidatorConfig) *TestValidator {
	if config == nil {
		config = DefaultValidatorConfig()
	}

	flakinessConfig := flakiness.DefaultTrackerConfig()
	flakinessConfig.WindowSize = config.FlakinessRuns
	flakinessConfig.MinRuns = config.StabilityRuns
	flakinessConfig.FlakyThreshold = config.FlakinessThreshold

	return &TestValidator{
		runner:           runner,
		flakinessTracker: flakiness.NewTracker(flakinessConfig),
		config:           config,
	}
}

// ValidationResult contains the complete validation results.
type ValidationResult struct {
	// Success indicates if all tests are stable
	Success bool `json:"success"`
	// TotalTests is the total number of tests validated
	TotalTests int `json:"totalTests"`
	// StableTests is the number of stable tests
	StableTests int `json:"stableTests"`
	// FlakyTests is the number of flaky tests detected
	FlakyTests int `json:"flakyTests"`
	// FailedTests is the number of consistently failing tests
	FailedTests int `json:"failedTests"`
	// TimedOutTests is the number of tests that timed out
	TimedOutTests int `json:"timedOutTests"`
	// Runs contains results from each validation run
	Runs []*RunResult `json:"runs"`
	// FlakinessScores contains flakiness scores for each test
	FlakinessScores []*flakiness.FlakinessScore `json:"flakinessScores,omitempty"`
	// Duration is the total validation duration
	Duration time.Duration `json:"duration"`
	// Errors contains any validation errors
	Errors []string `json:"errors,omitempty"`
	// Warnings contains validation warnings
	Warnings []string `json:"warnings,omitempty"`
	// TimedOut indicates if validation was aborted due to timeout
	TimedOut bool `json:"timedOut"`
}

// TestValidationResult contains validation result for a single test.
type TestValidationResult struct {
	TestName        string                    `json:"testName"`
	Stable          bool                      `json:"stable"`
	FlakinessScore  *flakiness.FlakinessScore `json:"flakinessScore,omitempty"`
	PassCount       int                       `json:"passCount"`
	FailCount       int                       `json:"failCount"`
	TimeoutCount    int                       `json:"timeoutCount"`
	AverageDuration time.Duration             `json:"averageDuration"`
	Errors          []string                  `json:"errors,omitempty"`
}

// ValidateTests runs tests multiple times to detect flakiness.
func (v *TestValidator) ValidateTests(ctx context.Context) (*ValidationResult, error) {
	startTime := time.Now()

	// Create context with global timeout
	globalCtx, cancel := context.WithTimeout(ctx, v.config.GlobalTimeout)
	defer cancel()

	result := &ValidationResult{
		Runs: make([]*RunResult, 0, v.config.FlakinessRuns),
	}

	// Run tests multiple times
	if v.config.ParallelRuns {
		result.Runs, result.Errors = v.runTestsParallel(globalCtx)
	} else {
		result.Runs, result.Errors = v.runTestsSequential(globalCtx)
	}

	// Check if we timed out
	if globalCtx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.Errors = append(result.Errors, "Validation timed out")
	}

	// Record all results to flakiness tracker and analyze
	v.analyzeResults(result)

	result.Duration = time.Since(startTime)
	result.Success = result.FlakyTests == 0 && result.FailedTests == 0 && result.TimedOutTests == 0

	return result, nil
}

// runTestsSequential runs tests one at a time.
func (v *TestValidator) runTestsSequential(ctx context.Context) ([]*RunResult, []string) {
	var runs []*RunResult
	var errors []string

	for i := 0; i < v.config.FlakinessRuns; i++ {
		select {
		case <-ctx.Done():
			errors = append(errors, fmt.Sprintf("Run %d cancelled: %v", i+1, ctx.Err()))
			return runs, errors
		default:
		}

		runResult, err := v.runWithTimeout(ctx, i+1)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Run %d error: %v", i+1, err))
			if v.config.AbortOnTimeout && isTimeoutError(err) {
				break
			}
		}
		if runResult != nil {
			runs = append(runs, runResult)
		}
	}

	return runs, errors
}

// runTestsParallel runs tests concurrently.
func (v *TestValidator) runTestsParallel(ctx context.Context) ([]*RunResult, []string) {
	var (
		runs   = make([]*RunResult, v.config.FlakinessRuns)
		errors []string
		mu     sync.Mutex
		wg     sync.WaitGroup
	)

	for i := 0; i < v.config.FlakinessRuns; i++ {
		wg.Add(1)
		go func(runIndex int) {
			defer wg.Done()

			runResult, err := v.runWithTimeout(ctx, runIndex+1)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errors = append(errors, fmt.Sprintf("Run %d error: %v", runIndex+1, err))
			}
			runs[runIndex] = runResult
		}(i)
	}

	wg.Wait()

	// Filter out nil results
	var validRuns []*RunResult
	for _, r := range runs {
		if r != nil {
			validRuns = append(validRuns, r)
		}
	}

	return validRuns, errors
}

// runWithTimeout runs tests with timeout handling.
func (v *TestValidator) runWithTimeout(ctx context.Context, attempt int) (*RunResult, error) {
	// Create run-specific context with timeout
	runCtx, cancel := context.WithTimeout(ctx, v.config.Timeout)
	defer cancel()

	// Create a channel to receive the result
	type runResultWithErr struct {
		result *RunResult
		err    error
	}
	resultCh := make(chan runResultWithErr, 1)

	go func() {
		result, err := v.runner.Run(runCtx)
		resultCh <- runResultWithErr{result: result, err: err}
	}()

	select {
	case <-runCtx.Done():
		// Timeout occurred
		if v.config.CleanupOnTimeout {
			v.cleanup()
		}
		return &RunResult{
			Success:    false,
			TotalTests: 0,
			Errors:     []string{fmt.Sprintf("Run %d timed out after %v", attempt, v.config.Timeout)},
		}, &TimeoutError{
			Attempt:  attempt,
			Duration: v.config.Timeout,
			Reason:   runCtx.Err().Error(),
		}
	case res := <-resultCh:
		return res.result, res.err
	}
}

// analyzeResults processes all run results and calculates flakiness.
func (v *TestValidator) analyzeResults(result *ValidationResult) {
	// Collect all unique test names
	testResults := make(map[string]*TestValidationResult)

	for runIdx, run := range result.Runs {
		if run == nil {
			continue
		}

		for _, test := range run.Tests {
			tvr, exists := testResults[test.Name]
			if !exists {
				tvr = &TestValidationResult{
					TestName: test.Name,
				}
				testResults[test.Name] = tvr
			}

			// Track pass/fail/timeout
			switch test.Status {
			case "passed":
				tvr.PassCount++
			case "failed":
				tvr.FailCount++
				if test.Error != "" {
					tvr.Errors = append(tvr.Errors, fmt.Sprintf("Run %d: %s", runIdx+1, test.Error))
				}
			case "timedout", "timeout":
				tvr.TimeoutCount++
			}

			// Track duration for average
			tvr.AverageDuration += test.Duration

			// Record to flakiness tracker
			v.flakinessTracker.RecordRun(flakiness.RunResult{
				TestID:    test.Name,
				TestName:  test.Name,
				Passed:    test.Status == "passed",
				Duration:  test.Duration,
				Timestamp: time.Now(),
				Error:     test.Error,
				Attempt:   runIdx + 1,
			})
		}
	}

	// Calculate final statistics
	result.TotalTests = len(testResults)

	for _, tvr := range testResults {
		totalRuns := tvr.PassCount + tvr.FailCount + tvr.TimeoutCount
		if totalRuns > 0 {
			tvr.AverageDuration = tvr.AverageDuration / time.Duration(totalRuns)
		}

		// Get flakiness score
		tvr.FlakinessScore = v.flakinessTracker.CalculateScore(tvr.TestName)
		if tvr.FlakinessScore != nil {
			result.FlakinessScores = append(result.FlakinessScores, tvr.FlakinessScore)
		}

		// Classify test
		if tvr.TimeoutCount > 0 {
			result.TimedOutTests++
		} else if tvr.PassCount == 0 {
			// Never passed = consistently failing
			result.FailedTests++
		} else if tvr.FailCount > 0 {
			// Mixed results = flaky
			result.FlakyTests++
		} else {
			// All passed = stable
			result.StableTests++
			tvr.Stable = true
		}
	}
}

// cleanup performs cleanup after timeout.
func (v *TestValidator) cleanup() {
	// Kill any lingering playwright processes
	exec.Command("pkill", "-f", "playwright").Run()
	exec.Command("pkill", "-f", "chromium").Run()
	exec.Command("pkill", "-f", "firefox").Run()
	exec.Command("pkill", "-f", "webkit").Run()

	// Clean up any temp files
	tempDir := filepath.Join(os.TempDir(), "playwright-*")
	matches, _ := filepath.Glob(tempDir)
	for _, m := range matches {
		os.RemoveAll(m)
	}
}

// ValidateSingleTest validates a single test for flakiness.
func (v *TestValidator) ValidateSingleTest(ctx context.Context, testName string) (*TestValidationResult, error) {
	result := &TestValidationResult{
		TestName: testName,
	}

	for i := 0; i < v.config.FlakinessRuns; i++ {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		runCtx, cancel := context.WithTimeout(ctx, v.config.Timeout)
		runResult, err := v.runner.RunTestCase(runCtx, testName)
		cancel()

		if err != nil {
			result.TimeoutCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Run %d: %v", i+1, err))
			continue
		}

		if runResult != nil && len(runResult.Tests) > 0 {
			test := runResult.Tests[0]
			switch test.Status {
			case "passed":
				result.PassCount++
			case "failed":
				result.FailCount++
				if test.Error != "" {
					result.Errors = append(result.Errors, fmt.Sprintf("Run %d: %s", i+1, test.Error))
				}
			}
			result.AverageDuration += test.Duration

			// Record to tracker
			v.flakinessTracker.RecordRun(flakiness.RunResult{
				TestID:    testName,
				TestName:  testName,
				Passed:    test.Status == "passed",
				Duration:  test.Duration,
				Timestamp: time.Now(),
				Error:     test.Error,
				Attempt:   i + 1,
			})
		}
	}

	// Calculate average duration
	totalRuns := result.PassCount + result.FailCount + result.TimeoutCount
	if totalRuns > 0 {
		result.AverageDuration = result.AverageDuration / time.Duration(totalRuns)
	}

	// Get flakiness score
	result.FlakinessScore = v.flakinessTracker.CalculateScore(testName)

	// Determine stability
	result.Stable = result.FailCount == 0 && result.TimeoutCount == 0

	return result, nil
}

// GetFlakinessTracker returns the flakiness tracker for external access.
func (v *TestValidator) GetFlakinessTracker() *flakiness.Tracker {
	return v.flakinessTracker
}

// TimeoutError represents a test timeout error.
type TimeoutError struct {
	Attempt  int
	Duration time.Duration
	Reason   string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("test run %d timed out after %v: %s", e.Attempt, e.Duration, e.Reason)
}

// isTimeoutError checks if an error is a timeout error.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*TimeoutError)
	if ok {
		return true
	}
	if err == context.DeadlineExceeded {
		return true
	}
	return false
}

// TimeoutConfig represents timeout configuration for different scenarios.
type TimeoutConfig struct {
	// ActionTimeout is the timeout for individual actions
	ActionTimeout time.Duration `json:"actionTimeout"`
	// NavigationTimeout is the timeout for page navigation
	NavigationTimeout time.Duration `json:"navigationTimeout"`
	// ExpectTimeout is the timeout for assertions/expectations
	ExpectTimeout time.Duration `json:"expectTimeout"`
	// TestTimeout is the timeout for an individual test
	TestTimeout time.Duration `json:"testTimeout"`
	// GlobalTimeout is the timeout for the entire test suite
	GlobalTimeout time.Duration `json:"globalTimeout"`
}

// DefaultTimeoutConfig returns sensible default timeouts.
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		ActionTimeout:     30 * time.Second,
		NavigationTimeout: 30 * time.Second,
		ExpectTimeout:     5 * time.Second,
		TestTimeout:       2 * time.Minute,
		GlobalTimeout:     30 * time.Minute,
	}
}

// TimeoutHandler provides graceful timeout handling.
type TimeoutHandler struct {
	config *TimeoutConfig
	mu     sync.Mutex
	timers map[string]*time.Timer
}

// NewTimeoutHandler creates a new timeout handler.
func NewTimeoutHandler(config *TimeoutConfig) *TimeoutHandler {
	if config == nil {
		config = DefaultTimeoutConfig()
	}
	return &TimeoutHandler{
		config: config,
		timers: make(map[string]*time.Timer),
	}
}

// StartTimeout starts a timeout timer for the given ID.
func (h *TimeoutHandler) StartTimeout(id string, timeout time.Duration, onTimeout func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Cancel existing timer if any
	if existing, ok := h.timers[id]; ok {
		existing.Stop()
	}

	h.timers[id] = time.AfterFunc(timeout, func() {
		h.mu.Lock()
		delete(h.timers, id)
		h.mu.Unlock()
		onTimeout()
	})
}

// CancelTimeout cancels a timeout timer.
func (h *TimeoutHandler) CancelTimeout(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if timer, ok := h.timers[id]; ok {
		timer.Stop()
		delete(h.timers, id)
	}
}

// WithTimeout wraps a function with timeout handling.
func (h *TimeoutHandler) WithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- fn(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return &TimeoutError{
			Duration: timeout,
			Reason:   ctx.Err().Error(),
		}
	}
}

// StabilityReport provides a detailed stability analysis.
type StabilityReport struct {
	// TestName is the name of the test
	TestName string `json:"testName"`
	// IsStable indicates if the test is stable
	IsStable bool `json:"isStable"`
	// Confidence is the confidence level (0-1)
	Confidence float64 `json:"confidence"`
	// Issues lists detected issues
	Issues []StabilityIssue `json:"issues,omitempty"`
	// Recommendations provides suggestions for improvement
	Recommendations []string `json:"recommendations,omitempty"`
}

// StabilityIssue represents a detected stability issue.
type StabilityIssue struct {
	Type        string `json:"type"`        // "flaky", "timeout", "network", "selector"
	Description string `json:"description"`
	Severity    string `json:"severity"`    // "low", "medium", "high"
	Occurrences int    `json:"occurrences"`
}

// GenerateStabilityReport generates a stability report for a test.
func (v *TestValidator) GenerateStabilityReport(testName string) *StabilityReport {
	score := v.flakinessTracker.CalculateScore(testName)
	history := v.flakinessTracker.GetHistory(testName)

	report := &StabilityReport{
		TestName: testName,
	}

	if score == nil || history == nil {
		report.Recommendations = append(report.Recommendations, "Run more tests to gather stability data")
		return report
	}

	// Calculate confidence based on number of runs
	if history.TotalRuns >= 10 {
		report.Confidence = 0.9
	} else if history.TotalRuns >= 5 {
		report.Confidence = 0.7
	} else if history.TotalRuns >= 3 {
		report.Confidence = 0.5
	} else {
		report.Confidence = 0.3
	}

	report.IsStable = score.Classification == "stable"

	// Analyze issues
	if score.FailureRate > 0.1 {
		report.Issues = append(report.Issues, StabilityIssue{
			Type:        "flaky",
			Description: fmt.Sprintf("%.0f%% failure rate across %d runs", score.FailureRate*100, history.TotalRuns),
			Severity:    severityFromRate(score.FailureRate),
			Occurrences: history.FailedRuns,
		})
	}

	if score.TransitionRate > 0.3 {
		report.Issues = append(report.Issues, StabilityIssue{
			Type:        "inconsistent",
			Description: fmt.Sprintf("High state transition rate: %.0f%%", score.TransitionRate*100),
			Severity:    "medium",
			Occurrences: history.Transitions,
		})
	}

	// Analyze error patterns
	errorCounts := make(map[string]int)
	for _, run := range history.Runs {
		if !run.Passed && run.Error != "" {
			category := categorizeFailure(run.Error)
			errorCounts[category]++
		}
	}

	for category, count := range errorCounts {
		if count > 1 {
			report.Issues = append(report.Issues, StabilityIssue{
				Type:        category,
				Description: fmt.Sprintf("%s errors detected", category),
				Severity:    severityFromCount(count, history.TotalRuns),
				Occurrences: count,
			})
		}
	}

	// Generate recommendations
	report.Recommendations = v.generateRecommendations(report.Issues, score)

	return report
}

func severityFromRate(rate float64) string {
	if rate > 0.5 {
		return "high"
	} else if rate > 0.2 {
		return "medium"
	}
	return "low"
}

func severityFromCount(count, total int) string {
	rate := float64(count) / float64(total)
	return severityFromRate(rate)
}

func (v *TestValidator) generateRecommendations(issues []StabilityIssue, score *flakiness.FlakinessScore) []string {
	var recs []string

	for _, issue := range issues {
		switch issue.Type {
		case "timeout":
			recs = append(recs, "Consider increasing test timeout or optimizing slow operations")
		case "selector":
			recs = append(recs, "Use more robust selectors (data-testid, aria-label) instead of CSS classes")
		case "network":
			recs = append(recs, "Add proper network idle waits or mock network responses")
		case "assertion":
			recs = append(recs, "Review assertion timing; consider using retry assertions")
		case "flaky":
			recs = append(recs, "Investigate race conditions or timing issues in the test")
		case "inconsistent":
			recs = append(recs, "Test behavior varies between runs; check for shared state or side effects")
		}
	}

	if score != nil && score.Classification == "highly_flaky" {
		recs = append(recs, "Consider quarantining this test until stability issues are resolved")
	}

	return recs
}
