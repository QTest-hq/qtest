package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// TestValidator runs and validates generated tests
type TestValidator struct {
	ws        *Workspace
	artifacts *ArtifactManager
}

// NewTestValidator creates a new test validator
func NewTestValidator(ws *Workspace) *TestValidator {
	return &TestValidator{
		ws:        ws,
		artifacts: NewArtifactManager(ws),
	}
}

// ValidationResult holds the result of validating a single test
type ValidationResult struct {
	TestFile   string        `json:"test_file"`
	Target     string        `json:"target"`
	Passed     bool          `json:"passed"`
	Output     string        `json:"output"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	TestCount  int           `json:"test_count"`
	PassCount  int           `json:"pass_count"`
	FailCount  int           `json:"fail_count"`
	SkipCount  int           `json:"skip_count"`
}

// ValidateAll runs all generated tests and returns results
func (v *TestValidator) ValidateAll(ctx context.Context) ([]ValidationResult, error) {
	var results []ValidationResult
	var testResults []TestResult

	startTime := time.Now()

	for _, target := range v.ws.State.Targets {
		if target.TestFile == "" || target.Status != StatusCompleted {
			continue
		}

		result := v.ValidateTest(ctx, target)
		results = append(results, result)

		// Convert to TestResult for artifact
		status := "passed"
		errMsg := ""
		if !result.Passed {
			status = "failed"
			errMsg = result.Error
		}

		testResults = append(testResults, TestResult{
			ID:         target.ID,
			Name:       target.Name,
			File:       target.TestFile,
			Target:     target.Name,
			Status:     status,
			DurationMs: int(result.Duration.Milliseconds()),
			Error:      errMsg,
		})
	}

	// Generate execution report
	duration := time.Since(startTime)
	if _, err := v.artifacts.GenerateExecutionReport(testResults, duration); err != nil {
		log.Warn().Err(err).Msg("failed to generate execution report")
	}

	return results, nil
}

// ValidateTest runs a single test file and returns the result
func (v *TestValidator) ValidateTest(ctx context.Context, target *TargetState) ValidationResult {
	result := ValidationResult{
		TestFile: target.TestFile,
		Target:   target.Name,
	}

	startTime := time.Now()

	// Determine how to run the test based on file extension
	ext := filepath.Ext(target.TestFile)
	var cmd *exec.Cmd

	switch ext {
	case ".go":
		cmd = v.goTestCommand(ctx, target.TestFile)
	case ".py":
		cmd = v.pythonTestCommand(ctx, target.TestFile)
	case ".js", ".ts":
		cmd = v.jestTestCommand(ctx, target.TestFile)
	default:
		result.Error = fmt.Sprintf("unsupported test file type: %s", ext)
		return result
	}

	// Run the test
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = v.ws.RepoPath

	err := cmd.Run()
	result.Duration = time.Since(startTime)
	result.Output = stdout.String() + stderr.String()

	if err != nil {
		// Test failed or error running
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Non-zero exit code means test failure
			result.Passed = false
			result.Error = fmt.Sprintf("tests failed (exit code %d)", exitErr.ExitCode())
		} else {
			result.Error = err.Error()
			return result
		}
	} else {
		result.Passed = true
	}

	// Parse test counts from output
	v.parseTestOutput(ext, result.Output, &result)

	return result
}

// goTestCommand creates a command to run Go tests
func (v *TestValidator) goTestCommand(ctx context.Context, testFile string) *exec.Cmd {
	// Get the package path from the test file
	dir := filepath.Dir(testFile)
	relDir, _ := filepath.Rel(v.ws.RepoPath, dir)
	pkg := "./" + relDir

	return exec.CommandContext(ctx, "go", "test", "-v", "-json", pkg, "-run", extractTestName(testFile))
}

// pythonTestCommand creates a command to run Python tests
func (v *TestValidator) pythonTestCommand(ctx context.Context, testFile string) *exec.Cmd {
	relPath, _ := filepath.Rel(v.ws.RepoPath, testFile)
	return exec.CommandContext(ctx, "python", "-m", "pytest", "-v", relPath)
}

// jestTestCommand creates a command to run Jest tests
func (v *TestValidator) jestTestCommand(ctx context.Context, testFile string) *exec.Cmd {
	relPath, _ := filepath.Rel(v.ws.RepoPath, testFile)
	return exec.CommandContext(ctx, "npx", "jest", "--verbose", relPath)
}

// parseTestOutput parses test output to extract counts
func (v *TestValidator) parseTestOutput(ext string, output string, result *ValidationResult) {
	switch ext {
	case ".go":
		v.parseGoTestOutput(output, result)
	case ".py":
		v.parsePytestOutput(output, result)
	case ".js", ".ts":
		v.parseJestOutput(output, result)
	}
}

// parseGoTestOutput parses Go test JSON output
func (v *TestValidator) parseGoTestOutput(output string, result *ValidationResult) {
	// Go test -json outputs one JSON object per line
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		var event struct {
			Action  string `json:"Action"`
			Package string `json:"Package"`
			Test    string `json:"Test"`
			Output  string `json:"Output"`
		}

		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.Test == "" {
			continue // Package-level event
		}

		switch event.Action {
		case "pass":
			result.PassCount++
			result.TestCount++
		case "fail":
			result.FailCount++
			result.TestCount++
		case "skip":
			result.SkipCount++
			result.TestCount++
		}
	}
}

// parsePytestOutput parses pytest output
func (v *TestValidator) parsePytestOutput(output string, result *ValidationResult) {
	// Look for summary line like "5 passed, 2 failed, 1 skipped"
	re := regexp.MustCompile(`(\d+)\s+passed`)
	if matches := re.FindStringSubmatch(output); len(matches) > 1 {
		result.PassCount, _ = strconv.Atoi(matches[1])
	}

	re = regexp.MustCompile(`(\d+)\s+failed`)
	if matches := re.FindStringSubmatch(output); len(matches) > 1 {
		result.FailCount, _ = strconv.Atoi(matches[1])
	}

	re = regexp.MustCompile(`(\d+)\s+skipped`)
	if matches := re.FindStringSubmatch(output); len(matches) > 1 {
		result.SkipCount, _ = strconv.Atoi(matches[1])
	}

	result.TestCount = result.PassCount + result.FailCount + result.SkipCount
}

// parseJestOutput parses Jest output
func (v *TestValidator) parseJestOutput(output string, result *ValidationResult) {
	// Look for summary like "Tests: 5 passed, 2 failed, 7 total"
	re := regexp.MustCompile(`Tests:\s+(\d+)\s+passed`)
	if matches := re.FindStringSubmatch(output); len(matches) > 1 {
		result.PassCount, _ = strconv.Atoi(matches[1])
	}

	re = regexp.MustCompile(`(\d+)\s+failed`)
	if matches := re.FindStringSubmatch(output); len(matches) > 1 {
		result.FailCount, _ = strconv.Atoi(matches[1])
	}

	re = regexp.MustCompile(`(\d+)\s+skipped`)
	if matches := re.FindStringSubmatch(output); len(matches) > 1 {
		result.SkipCount, _ = strconv.Atoi(matches[1])
	}

	re = regexp.MustCompile(`(\d+)\s+total`)
	if matches := re.FindStringSubmatch(output); len(matches) > 1 {
		result.TestCount, _ = strconv.Atoi(matches[1])
	}
}

// extractTestName extracts a test name pattern from a test file path
func extractTestName(testFile string) string {
	base := filepath.Base(testFile)
	// Remove _test.go suffix
	name := strings.TrimSuffix(base, "_test.go")
	// Convert to CamelCase for Go test pattern
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return "Test" + strings.Join(parts, "")
}

// ValidateSummary returns a summary of validation results
type ValidateSummary struct {
	Total      int           `json:"total"`
	Passed     int           `json:"passed"`
	Failed     int           `json:"failed"`
	Skipped    int           `json:"skipped"`
	Duration   time.Duration `json:"duration"`
	PassRate   float64       `json:"pass_rate"`
	FailedTests []string     `json:"failed_tests,omitempty"`
}

// Summarize creates a summary from validation results
func Summarize(results []ValidationResult) ValidateSummary {
	summary := ValidateSummary{}

	var totalDuration time.Duration

	for _, r := range results {
		summary.Total++
		totalDuration += r.Duration

		if r.Passed {
			summary.Passed++
		} else {
			summary.Failed++
			summary.FailedTests = append(summary.FailedTests, r.TestFile)
		}
	}

	summary.Duration = totalDuration

	if summary.Total > 0 {
		summary.PassRate = float64(summary.Passed) / float64(summary.Total) * 100
	}

	return summary
}
