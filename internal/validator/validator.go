package validator

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// TestResult holds the result of running a test
type TestResult struct {
	Passed    bool          `json:"passed"`
	TestFile  string        `json:"test_file"`
	Output    string        `json:"output"`
	Errors    []TestError   `json:"errors,omitempty"`
	Duration  time.Duration `json:"duration"`
	ExitCode  int           `json:"exit_code"`
}

// TestError represents a single test failure
type TestError struct {
	TestName    string `json:"test_name"`
	Message     string `json:"message"`
	Expected    string `json:"expected,omitempty"`
	Actual      string `json:"actual,omitempty"`
	StackTrace  string `json:"stack_trace,omitempty"`
	Line        int    `json:"line,omitempty"`
}

// Validator runs and validates generated tests
type Validator struct {
	workDir  string
	language string
}

// NewValidator creates a new test validator
func NewValidator(workDir, language string) *Validator {
	return &Validator{
		workDir:  workDir,
		language: language,
	}
}

// RunTests executes tests and returns results
func (v *Validator) RunTests(ctx context.Context, testFile string) (*TestResult, error) {
	start := time.Now()

	var cmd *exec.Cmd
	var runner string

	switch v.language {
	case "javascript", "typescript":
		// Try npm test, jest, or npx jest
		runner = "jest"
		cmd = exec.CommandContext(ctx, "npx", "jest", testFile, "--json", "--testLocationInResults")
	case "python":
		runner = "pytest"
		cmd = exec.CommandContext(ctx, "pytest", testFile, "-v", "--tb=short")
	case "go":
		runner = "go test"
		dir := filepath.Dir(testFile)
		cmd = exec.CommandContext(ctx, "go", "test", "-v", "-json", "./...")
		cmd.Dir = dir
	default:
		return nil, fmt.Errorf("unsupported language: %s", v.language)
	}

	cmd.Dir = v.workDir

	log.Debug().Str("runner", runner).Str("file", testFile).Msg("running tests")

	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	result := &TestResult{
		Passed:   exitCode == 0,
		TestFile: testFile,
		Output:   string(output),
		Duration: time.Since(start),
		ExitCode: exitCode,
	}

	// Parse errors if tests failed
	if !result.Passed {
		result.Errors = v.parseErrors(string(output))
	}

	log.Info().
		Bool("passed", result.Passed).
		Int("errors", len(result.Errors)).
		Dur("duration", result.Duration).
		Msg("test run complete")

	return result, nil
}

// parseErrors extracts test failures from output
func (v *Validator) parseErrors(output string) []TestError {
	var errors []TestError

	switch v.language {
	case "javascript", "typescript":
		errors = parseJestErrors(output)
	case "python":
		errors = parsePytestErrors(output)
	case "go":
		errors = parseGoTestErrors(output)
	}

	return errors
}

// parseJestErrors extracts errors from Jest output
func parseJestErrors(output string) []TestError {
	var errors []TestError
	lines := strings.Split(output, "\n")

	var currentTest string
	var currentError TestError
	inError := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Detect test name
		if strings.Contains(line, "✕") || strings.Contains(line, "FAIL") {
			if strings.Contains(line, "✕") {
				parts := strings.SplitN(line, "✕", 2)
				if len(parts) > 1 {
					currentTest = strings.TrimSpace(parts[1])
				}
			}
			inError = true
			currentError = TestError{TestName: currentTest}
		}

		// Capture error message
		if inError {
			if strings.Contains(line, "Expected:") {
				currentError.Expected = strings.TrimPrefix(line, "Expected:")
				currentError.Expected = strings.TrimSpace(currentError.Expected)
			}
			if strings.Contains(line, "Received:") {
				currentError.Actual = strings.TrimPrefix(line, "Received:")
				currentError.Actual = strings.TrimSpace(currentError.Actual)
			}
			if strings.Contains(line, "Error:") || strings.Contains(line, "TypeError:") {
				currentError.Message = line
			}

			// End of error block
			if line == "" && currentError.TestName != "" {
				errors = append(errors, currentError)
				inError = false
				currentError = TestError{}
			}
		}
	}

	// Add last error if exists
	if currentError.TestName != "" {
		errors = append(errors, currentError)
	}

	return errors
}

// parsePytestErrors extracts errors from pytest output
func parsePytestErrors(output string) []TestError {
	var errors []TestError
	lines := strings.Split(output, "\n")

	var currentError TestError
	inError := false

	for _, line := range lines {
		// Detect failed test
		if strings.Contains(line, "FAILED") {
			parts := strings.Split(line, "::")
			if len(parts) >= 2 {
				currentError = TestError{
					TestName: parts[len(parts)-1],
				}
				inError = true
			}
		}

		if inError {
			// Capture assertion errors
			if strings.Contains(line, "AssertionError") || strings.Contains(line, "assert") {
				currentError.Message = strings.TrimSpace(line)
			}
			if strings.Contains(line, "E       ") {
				msg := strings.TrimPrefix(line, "E       ")
				if currentError.Message == "" {
					currentError.Message = msg
				} else {
					currentError.Message += " " + msg
				}
			}

			// End of error
			if strings.HasPrefix(line, "=") && strings.Contains(line, "short test summary") {
				if currentError.TestName != "" {
					errors = append(errors, currentError)
				}
				inError = false
			}
		}
	}

	return errors
}

// parseGoTestErrors extracts errors from go test output
func parseGoTestErrors(output string) []TestError {
	var errors []TestError
	lines := strings.Split(output, "\n")

	var currentError TestError

	for _, line := range lines {
		// Detect failed test
		if strings.Contains(line, "--- FAIL:") {
			parts := strings.Split(line, "--- FAIL:")
			if len(parts) > 1 {
				testName := strings.TrimSpace(parts[1])
				testName = strings.Split(testName, " ")[0]
				currentError = TestError{TestName: testName}
			}
		}

		// Capture error details
		if strings.Contains(line, "Error Trace:") || strings.Contains(line, "Error:") {
			if currentError.TestName != "" {
				msg := strings.TrimSpace(line)
				if currentError.Message == "" {
					currentError.Message = msg
				} else {
					currentError.Message += " | " + msg
				}
			}
		}

		// Capture expected/actual
		if strings.Contains(line, "expected:") {
			currentError.Expected = strings.TrimSpace(strings.TrimPrefix(line, "expected:"))
		}
		if strings.Contains(line, "actual:") || strings.Contains(line, "got:") {
			currentError.Actual = strings.TrimSpace(line)
		}

		// End of test
		if strings.HasPrefix(line, "FAIL") && currentError.TestName != "" {
			errors = append(errors, currentError)
			currentError = TestError{}
		}
	}

	return errors
}

// FormatErrorsForLLM formats errors for LLM consumption
func (v *Validator) FormatErrorsForLLM(result *TestResult) string {
	if result.Passed {
		return "All tests passed successfully."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Test file: %s\n", result.TestFile))
	sb.WriteString(fmt.Sprintf("Status: FAILED (%d errors)\n\n", len(result.Errors)))

	for i, err := range result.Errors {
		sb.WriteString(fmt.Sprintf("Error %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("  Test: %s\n", err.TestName))
		if err.Message != "" {
			sb.WriteString(fmt.Sprintf("  Message: %s\n", err.Message))
		}
		if err.Expected != "" {
			sb.WriteString(fmt.Sprintf("  Expected: %s\n", err.Expected))
		}
		if err.Actual != "" {
			sb.WriteString(fmt.Sprintf("  Actual: %s\n", err.Actual))
		}
		sb.WriteString("\n")
	}

	// Include relevant output
	sb.WriteString("Raw output (truncated):\n")
	output := result.Output
	if len(output) > 2000 {
		output = output[:2000] + "...[truncated]"
	}
	sb.WriteString(output)

	return sb.String()
}
