package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TestRunner runs E2E tests using Playwright.
type TestRunner struct {
	config *RunnerConfig
}

// RunnerConfig configures the test runner.
type RunnerConfig struct {
	// WorkDir is the working directory for test execution
	WorkDir string `json:"workDir"`
	// TestDir is the directory containing test files
	TestDir string `json:"testDir"`
	// OutputDir is the directory for test results
	OutputDir string `json:"outputDir"`
	// Headless runs tests in headless mode
	Headless bool `json:"headless"`
	// Timeout is the maximum time for test execution
	Timeout time.Duration `json:"timeout"`
	// Retries is the number of retries for flaky tests
	Retries int `json:"retries"`
	// Workers is the number of parallel workers
	Workers int `json:"workers"`
	// Reporter is the test reporter type (json, html, list)
	Reporter string `json:"reporter"`
	// BaseURL is the base URL for the application under test
	BaseURL string `json:"baseUrl"`
	// Browser is the browser to use (chromium, firefox, webkit)
	Browser string `json:"browser"`
}

// DefaultRunnerConfig returns the default runner configuration.
func DefaultRunnerConfig() *RunnerConfig {
	return &RunnerConfig{
		WorkDir:  ".",
		TestDir:  "tests",
		OutputDir: "test-results",
		Headless: true,
		Timeout:  5 * time.Minute,
		Retries:  2,
		Workers:  4,
		Reporter: "json",
		Browser:  "chromium",
	}
}

// NewTestRunner creates a new test runner.
func NewTestRunner(config *RunnerConfig) *TestRunner {
	if config == nil {
		config = DefaultRunnerConfig()
	}
	return &TestRunner{config: config}
}

// RunResult represents the result of a test run.
type RunResult struct {
	// Success indicates if all tests passed
	Success bool `json:"success"`
	// TotalTests is the total number of tests
	TotalTests int `json:"totalTests"`
	// Passed is the number of passed tests
	Passed int `json:"passed"`
	// Failed is the number of failed tests
	Failed int `json:"failed"`
	// Skipped is the number of skipped tests
	Skipped int `json:"skipped"`
	// Duration is the total test duration
	Duration time.Duration `json:"duration"`
	// Tests contains individual test results
	Tests []TestResult `json:"tests"`
	// Errors contains any execution errors
	Errors []string `json:"errors,omitempty"`
	// OutputPath is the path to the detailed results
	OutputPath string `json:"outputPath,omitempty"`
}

// TestResult represents a single test result.
type TestResult struct {
	// Name is the test name
	Name string `json:"name"`
	// Status is the test status (passed, failed, skipped)
	Status string `json:"status"`
	// Duration is the test duration
	Duration time.Duration `json:"duration"`
	// Error is the error message if failed
	Error string `json:"error,omitempty"`
	// Screenshots contains paths to screenshots
	Screenshots []string `json:"screenshots,omitempty"`
	// Trace is the path to the trace file
	Trace string `json:"trace,omitempty"`
	// Retry indicates the retry number
	Retry int `json:"retry,omitempty"`
}

// Run executes all tests in the test directory.
func (r *TestRunner) Run(ctx context.Context) (*RunResult, error) {
	return r.RunTests(ctx, "")
}

// RunTests executes tests matching the given pattern.
func (r *TestRunner) RunTests(ctx context.Context, pattern string) (*RunResult, error) {
	startTime := time.Now()

	// Ensure output directory exists
	if err := os.MkdirAll(r.config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build playwright command
	args := r.buildPlaywrightArgs(pattern)

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// Execute playwright
	cmd := exec.CommandContext(execCtx, "npx", args...)
	cmd.Dir = r.config.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime)

	// Parse results
	result := &RunResult{
		Duration:   duration,
		OutputPath: filepath.Join(r.config.OutputDir, "results.json"),
	}

	// Check for execution errors
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.Errors = append(result.Errors, "Test execution timed out")
		} else {
			// Playwright returns non-zero on test failures, which is expected
			// Only add as error if it's not a test failure
			if !strings.Contains(stderr.String(), "failed") {
				result.Errors = append(result.Errors, fmt.Sprintf("Execution error: %v", err))
			}
		}
	}

	// Parse JSON results if available
	if err := r.parseJSONResults(result); err != nil {
		// Fall back to parsing stdout
		r.parseStdoutResults(result, stdout.String())
	}

	result.Success = result.Failed == 0 && len(result.Errors) == 0

	return result, nil
}

// RunFile executes a specific test file.
func (r *TestRunner) RunFile(ctx context.Context, filePath string) (*RunResult, error) {
	return r.RunTests(ctx, filePath)
}

// RunTestCase executes a specific test case by name.
func (r *TestRunner) RunTestCase(ctx context.Context, testName string) (*RunResult, error) {
	return r.RunTests(ctx, fmt.Sprintf("-g '%s'", testName))
}

func (r *TestRunner) buildPlaywrightArgs(pattern string) []string {
	args := []string{"playwright", "test"}

	// Add test pattern if specified
	if pattern != "" {
		if strings.HasSuffix(pattern, ".spec.ts") || strings.HasSuffix(pattern, ".spec.js") {
			args = append(args, pattern)
		} else {
			args = append(args, "--grep", pattern)
		}
	}

	// Reporter
	args = append(args, "--reporter", r.config.Reporter)

	// Output directory
	args = append(args, "--output", r.config.OutputDir)

	// Headless mode
	if r.config.Headless {
		args = append(args, "--headed=false")
	} else {
		args = append(args, "--headed")
	}

	// Retries
	args = append(args, "--retries", fmt.Sprintf("%d", r.config.Retries))

	// Workers
	args = append(args, "--workers", fmt.Sprintf("%d", r.config.Workers))

	// Browser
	if r.config.Browser != "" {
		args = append(args, "--browser", r.config.Browser)
	}

	// Base URL
	if r.config.BaseURL != "" {
		args = append(args, "--base-url", r.config.BaseURL)
	}

	return args
}

func (r *TestRunner) parseJSONResults(result *RunResult) error {
	jsonPath := filepath.Join(r.config.OutputDir, "results.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}

	var rawResult struct {
		Suites []struct {
			Title string `json:"title"`
			Specs []struct {
				Title string `json:"title"`
				Tests []struct {
					Title    string `json:"title"`
					Status   string `json:"status"`
					Duration int    `json:"duration"`
					Errors   []struct {
						Message string `json:"message"`
					} `json:"errors"`
					Attachments []struct {
						Name string `json:"name"`
						Path string `json:"path"`
					} `json:"attachments"`
				} `json:"tests"`
			} `json:"specs"`
		} `json:"suites"`
		Stats struct {
			Total    int `json:"total"`
			Passed   int `json:"passed"`
			Failed   int `json:"failed"`
			Skipped  int `json:"skipped"`
			Duration int `json:"duration"`
		} `json:"stats"`
	}

	if err := json.Unmarshal(data, &rawResult); err != nil {
		return err
	}

	result.TotalTests = rawResult.Stats.Total
	result.Passed = rawResult.Stats.Passed
	result.Failed = rawResult.Stats.Failed
	result.Skipped = rawResult.Stats.Skipped

	// Extract individual test results
	for _, suite := range rawResult.Suites {
		for _, spec := range suite.Specs {
			for _, test := range spec.Tests {
				tr := TestResult{
					Name:     fmt.Sprintf("%s > %s > %s", suite.Title, spec.Title, test.Title),
					Status:   test.Status,
					Duration: time.Duration(test.Duration) * time.Millisecond,
				}

				if len(test.Errors) > 0 {
					tr.Error = test.Errors[0].Message
				}

				for _, att := range test.Attachments {
					if strings.Contains(att.Name, "screenshot") {
						tr.Screenshots = append(tr.Screenshots, att.Path)
					} else if strings.Contains(att.Name, "trace") {
						tr.Trace = att.Path
					}
				}

				result.Tests = append(result.Tests, tr)
			}
		}
	}

	return nil
}

func (r *TestRunner) parseStdoutResults(result *RunResult, stdout string) {
	// Parse Playwright's stdout format
	scanner := bufio.NewScanner(strings.NewReader(stdout))

	// Patterns for matching test results
	passedPattern := regexp.MustCompile(`(\d+) passed`)
	failedPattern := regexp.MustCompile(`(\d+) failed`)
	skippedPattern := regexp.MustCompile(`(\d+) skipped`)
	testPattern := regexp.MustCompile(`\s+(✓|×|⊘)\s+(.+)\s+\((\d+(?:\.\d+)?)(m?s)\)`)

	for scanner.Scan() {
		line := scanner.Text()

		// Parse summary line
		if matches := passedPattern.FindStringSubmatch(line); matches != nil {
			fmt.Sscanf(matches[1], "%d", &result.Passed)
		}
		if matches := failedPattern.FindStringSubmatch(line); matches != nil {
			fmt.Sscanf(matches[1], "%d", &result.Failed)
		}
		if matches := skippedPattern.FindStringSubmatch(line); matches != nil {
			fmt.Sscanf(matches[1], "%d", &result.Skipped)
		}

		// Parse individual test lines
		if matches := testPattern.FindStringSubmatch(line); matches != nil {
			status := "passed"
			if matches[1] == "×" {
				status = "failed"
			} else if matches[1] == "⊘" {
				status = "skipped"
			}

			var duration time.Duration
			if matches[4] == "ms" {
				var ms float64
				fmt.Sscanf(matches[3], "%f", &ms)
				duration = time.Duration(ms) * time.Millisecond
			} else {
				var s float64
				fmt.Sscanf(matches[3], "%f", &s)
				duration = time.Duration(s * float64(time.Second))
			}

			result.Tests = append(result.Tests, TestResult{
				Name:     strings.TrimSpace(matches[2]),
				Status:   status,
				Duration: duration,
			})
		}
	}

	result.TotalTests = result.Passed + result.Failed + result.Skipped
}

// ValidateResults validates test results and returns a summary.
func (r *TestRunner) ValidateResults(result *RunResult) *ValidationSummary {
	summary := &ValidationSummary{
		TotalTests:   result.TotalTests,
		PassedTests:  result.Passed,
		FailedTests:  result.Failed,
		SkippedTests: result.Skipped,
		PassRate:     0,
		Duration:     result.Duration,
	}

	if result.TotalTests > 0 {
		summary.PassRate = float64(result.Passed) / float64(result.TotalTests) * 100
	}

	// Categorize failures
	for _, test := range result.Tests {
		if test.Status == "failed" {
			failure := TestFailure{
				TestName: test.Name,
				Error:    test.Error,
				Category: categorizeFailure(test.Error),
			}
			summary.Failures = append(summary.Failures, failure)
		}
	}

	return summary
}

// ValidationSummary provides a summary of test validation.
type ValidationSummary struct {
	TotalTests   int           `json:"totalTests"`
	PassedTests  int           `json:"passedTests"`
	FailedTests  int           `json:"failedTests"`
	SkippedTests int           `json:"skippedTests"`
	PassRate     float64       `json:"passRate"`
	Duration     time.Duration `json:"duration"`
	Failures     []TestFailure `json:"failures,omitempty"`
}

// TestFailure represents a test failure with categorization.
type TestFailure struct {
	TestName string `json:"testName"`
	Error    string `json:"error"`
	Category string `json:"category"` // timeout, assertion, selector, network, unknown
}

func categorizeFailure(errorMsg string) string {
	errorLower := strings.ToLower(errorMsg)

	// Check timeout first - it's the most common and can overlap with selector errors
	switch {
	case strings.Contains(errorLower, "timeout") || strings.Contains(errorLower, "timed out"):
		return "timeout"
	case strings.Contains(errorLower, "expect") || strings.Contains(errorLower, "assert"):
		return "assertion"
	case strings.Contains(errorLower, "selector") || strings.Contains(errorLower, "locator"):
		return "selector"
	case strings.Contains(errorLower, "network") || strings.Contains(errorLower, "connection"):
		return "network"
	default:
		return "unknown"
	}
}

// GenerateReport generates a test report.
func (r *TestRunner) GenerateReport(result *RunResult, format string) (string, error) {
	switch format {
	case "json":
		return r.generateJSONReport(result)
	case "html":
		return r.generateHTMLReport(result)
	case "markdown":
		return r.generateMarkdownReport(result)
	default:
		return "", fmt.Errorf("unsupported report format: %s", format)
	}
}

func (r *TestRunner) generateJSONReport(result *RunResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *TestRunner) generateHTMLReport(result *RunResult) (string, error) {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
  <title>E2E Test Report</title>
  <style>
    body { font-family: -apple-system, sans-serif; margin: 40px; }
    .summary { background: #f5f5f5; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
    .passed { color: #22c55e; }
    .failed { color: #ef4444; }
    .skipped { color: #f59e0b; }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 12px; text-align: left; border-bottom: 1px solid #e5e5e5; }
    th { background: #f5f5f5; }
  </style>
</head>
<body>
`)

	// Summary
	sb.WriteString("<div class=\"summary\">\n")
	sb.WriteString(fmt.Sprintf("<h1>E2E Test Report</h1>\n"))
	sb.WriteString(fmt.Sprintf("<p><strong>Total:</strong> %d tests</p>\n", result.TotalTests))
	sb.WriteString(fmt.Sprintf("<p class=\"passed\"><strong>Passed:</strong> %d</p>\n", result.Passed))
	sb.WriteString(fmt.Sprintf("<p class=\"failed\"><strong>Failed:</strong> %d</p>\n", result.Failed))
	sb.WriteString(fmt.Sprintf("<p class=\"skipped\"><strong>Skipped:</strong> %d</p>\n", result.Skipped))
	sb.WriteString(fmt.Sprintf("<p><strong>Duration:</strong> %s</p>\n", result.Duration))
	sb.WriteString("</div>\n")

	// Test details
	sb.WriteString("<h2>Test Results</h2>\n")
	sb.WriteString("<table>\n")
	sb.WriteString("<tr><th>Test</th><th>Status</th><th>Duration</th><th>Error</th></tr>\n")

	for _, test := range result.Tests {
		statusClass := test.Status
		sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td class=\"%s\">%s</td><td>%s</td><td>%s</td></tr>\n",
			test.Name, statusClass, test.Status, test.Duration, test.Error))
	}

	sb.WriteString("</table>\n")
	sb.WriteString("</body></html>")

	return sb.String(), nil
}

func (r *TestRunner) generateMarkdownReport(result *RunResult) (string, error) {
	var sb strings.Builder

	sb.WriteString("# E2E Test Report\n\n")
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Tests:** %d\n", result.TotalTests))
	sb.WriteString(fmt.Sprintf("- **Passed:** %d\n", result.Passed))
	sb.WriteString(fmt.Sprintf("- **Failed:** %d\n", result.Failed))
	sb.WriteString(fmt.Sprintf("- **Skipped:** %d\n", result.Skipped))
	sb.WriteString(fmt.Sprintf("- **Duration:** %s\n\n", result.Duration))

	if result.Success {
		sb.WriteString("**Status:** All tests passed!\n\n")
	} else {
		sb.WriteString("**Status:** Some tests failed.\n\n")
	}

	sb.WriteString("## Test Results\n\n")
	sb.WriteString("| Test | Status | Duration | Error |\n")
	sb.WriteString("|------|--------|----------|-------|\n")

	for _, test := range result.Tests {
		statusIcon := "✅"
		if test.Status == "failed" {
			statusIcon = "❌"
		} else if test.Status == "skipped" {
			statusIcon = "⏭️"
		}

		errorMsg := "-"
		if test.Error != "" {
			// Truncate long errors
			if len(test.Error) > 50 {
				errorMsg = test.Error[:50] + "..."
			} else {
				errorMsg = test.Error
			}
		}

		sb.WriteString(fmt.Sprintf("| %s | %s %s | %s | %s |\n",
			test.Name, statusIcon, test.Status, test.Duration, errorMsg))
	}

	return sb.String(), nil
}
