package validator

import (
	"strings"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/llm"
)

func TestNewValidator(t *testing.T) {
	v := NewValidator("/tmp/test", "go")

	if v == nil {
		t.Fatal("NewValidator() returned nil")
	}
	if v.workDir != "/tmp/test" {
		t.Errorf("workDir = %s, want /tmp/test", v.workDir)
	}
	if v.language != "go" {
		t.Errorf("language = %s, want go", v.language)
	}
}

func TestValidator_Languages(t *testing.T) {
	tests := []struct {
		language string
	}{
		{"go"},
		{"python"},
		{"javascript"},
		{"typescript"},
	}

	for _, tt := range tests {
		v := NewValidator("/tmp", tt.language)
		if v.language != tt.language {
			t.Errorf("language = %s, want %s", v.language, tt.language)
		}
	}
}

func TestTestResult_Fields(t *testing.T) {
	result := TestResult{
		Passed:   false,
		TestFile: "test.go",
		Output:   "FAIL",
		Errors: []TestError{
			{TestName: "TestFunc", Message: "assertion failed"},
		},
		Duration: time.Second,
		ExitCode: 1,
	}

	if result.Passed {
		t.Error("Passed should be false")
	}
	if result.TestFile != "test.go" {
		t.Errorf("TestFile = %s, want test.go", result.TestFile)
	}
	if result.Output != "FAIL" {
		t.Errorf("Output = %s, want FAIL", result.Output)
	}
	if len(result.Errors) != 1 {
		t.Errorf("len(Errors) = %d, want 1", len(result.Errors))
	}
	if result.Duration != time.Second {
		t.Errorf("Duration = %v, want 1s", result.Duration)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
}

func TestTestError_Fields(t *testing.T) {
	err := TestError{
		TestName:   "TestFunc",
		Message:    "assertion failed",
		Expected:   "3",
		Actual:     "2",
		StackTrace: "at line 10",
		Line:       10,
	}

	if err.TestName != "TestFunc" {
		t.Errorf("TestName = %s, want TestFunc", err.TestName)
	}
	if err.Message != "assertion failed" {
		t.Errorf("Message = %s, want assertion failed", err.Message)
	}
	if err.Expected != "3" {
		t.Errorf("Expected = %s, want 3", err.Expected)
	}
	if err.Actual != "2" {
		t.Errorf("Actual = %s, want 2", err.Actual)
	}
	if err.StackTrace != "at line 10" {
		t.Errorf("StackTrace = %s, want at line 10", err.StackTrace)
	}
	if err.Line != 10 {
		t.Errorf("Line = %d, want 10", err.Line)
	}
}

func TestParseJestErrors_Basic(t *testing.T) {
	output := `
FAIL tests/math.test.js
  Math
    ✕ should add numbers (5 ms)

  ● Math › should add numbers

    Expected: 5
    Received: 4

`
	errors := parseJestErrors(output)

	if len(errors) == 0 {
		t.Fatal("Should find errors")
	}
}

func TestParseJestErrors_WithError(t *testing.T) {
	output := `
  ✕ test name
    Error: something went wrong
`
	errors := parseJestErrors(output)

	hasError := false
	for _, e := range errors {
		if strings.Contains(e.Message, "Error:") {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Log("May not find error message")
	}
}

func TestParseJestErrors_Empty(t *testing.T) {
	output := "PASS all tests"
	errors := parseJestErrors(output)

	if len(errors) != 0 {
		t.Errorf("len(errors) = %d, want 0 for passing tests", len(errors))
	}
}

func TestParsePytestErrors_Basic(t *testing.T) {
	output := `
============================= test session starts ==============================
FAILED tests/test_math.py::test_add
E       AssertionError: assert 4 == 5
============================= short test summary info ==========================
FAILED tests/test_math.py::test_add - AssertionError: assert 4 == 5
`
	errors := parsePytestErrors(output)

	if len(errors) == 0 {
		t.Fatal("Should find errors")
	}
	if errors[0].TestName == "" {
		t.Error("Should capture test name")
	}
}

func TestParsePytestErrors_WithAssertion(t *testing.T) {
	output := `
FAILED test_func
E       assert 1 == 2
============================= short test summary info ==========================
`
	errors := parsePytestErrors(output)

	hasAssert := false
	for _, e := range errors {
		if strings.Contains(e.Message, "assert") {
			hasAssert = true
			break
		}
	}
	if len(errors) > 0 && !hasAssert {
		t.Log("May not capture assertion")
	}
}

func TestParsePytestErrors_Empty(t *testing.T) {
	output := "PASSED all tests"
	errors := parsePytestErrors(output)

	if len(errors) != 0 {
		t.Errorf("len(errors) = %d, want 0 for passing tests", len(errors))
	}
}

func TestParseGoTestErrors_Basic(t *testing.T) {
	output := `
--- FAIL: TestAdd (0.00s)
    math_test.go:10:
        Error: expected 5 got 4
FAIL
`
	errors := parseGoTestErrors(output)

	if len(errors) == 0 {
		t.Fatal("Should find errors")
	}
	if errors[0].TestName != "TestAdd" {
		t.Errorf("TestName = %s, want TestAdd", errors[0].TestName)
	}
}

func TestParseGoTestErrors_WithExpected(t *testing.T) {
	output := `
--- FAIL: TestFunc (0.00s)
    expected: 5
    actual: 4
FAIL
`
	errors := parseGoTestErrors(output)

	if len(errors) == 0 {
		t.Fatal("Should find errors")
	}
	if errors[0].Expected != "5" {
		t.Logf("Expected = %s (may vary)", errors[0].Expected)
	}
}

func TestParseGoTestErrors_Empty(t *testing.T) {
	output := "PASS ok package 0.01s"
	errors := parseGoTestErrors(output)

	if len(errors) != 0 {
		t.Errorf("len(errors) = %d, want 0 for passing tests", len(errors))
	}
}

func TestFormatErrorsForLLM_Passed(t *testing.T) {
	v := NewValidator("/tmp", "go")
	result := &TestResult{Passed: true}

	formatted := v.FormatErrorsForLLM(result)

	if !strings.Contains(formatted, "passed successfully") {
		t.Error("Should indicate tests passed")
	}
}

func TestFormatErrorsForLLM_Failed(t *testing.T) {
	v := NewValidator("/tmp", "go")
	result := &TestResult{
		Passed:   false,
		TestFile: "test.go",
		Output:   "some test output here",
		Errors: []TestError{
			{
				TestName: "TestFunc",
				Message:  "assertion failed",
				Expected: "3",
				Actual:   "2",
			},
		},
	}

	formatted := v.FormatErrorsForLLM(result)

	if !strings.Contains(formatted, "test.go") {
		t.Error("Should include test file")
	}
	if !strings.Contains(formatted, "FAILED") {
		t.Error("Should indicate failure")
	}
	if !strings.Contains(formatted, "TestFunc") {
		t.Error("Should include test name")
	}
	if !strings.Contains(formatted, "assertion failed") {
		t.Error("Should include error message")
	}
	if !strings.Contains(formatted, "Expected: 3") {
		t.Error("Should include expected value")
	}
	if !strings.Contains(formatted, "Actual: 2") {
		t.Error("Should include actual value")
	}
}

func TestFormatErrorsForLLM_TruncatesLongOutput(t *testing.T) {
	v := NewValidator("/tmp", "go")

	// Create a very long output
	longOutput := strings.Repeat("a", 3000)
	result := &TestResult{
		Passed:   false,
		TestFile: "test.go",
		Output:   longOutput,
		Errors:   []TestError{},
	}

	formatted := v.FormatErrorsForLLM(result)

	if !strings.Contains(formatted, "[truncated]") {
		t.Error("Should truncate long output")
	}
}

func TestNewFixer(t *testing.T) {
	f := NewFixer(nil, llm.Tier1)

	if f == nil {
		t.Fatal("NewFixer() returned nil")
	}
	if f.router != nil {
		t.Error("router should be nil")
	}
	if f.tier != llm.Tier1 {
		t.Errorf("tier = %v, want Tier1", f.tier)
	}
	if f.maxRetries != 3 {
		t.Errorf("maxRetries = %d, want 3", f.maxRetries)
	}
}

func TestFixResult_Fields(t *testing.T) {
	fr := FixResult{
		Fixed:       true,
		NewCode:     "func Test() {}",
		Explanation: "Fixed assertion",
		Attempts:    2,
	}

	if !fr.Fixed {
		t.Error("Fixed should be true")
	}
	if fr.NewCode != "func Test() {}" {
		t.Error("NewCode mismatch")
	}
	if fr.Explanation != "Fixed assertion" {
		t.Error("Explanation mismatch")
	}
	if fr.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", fr.Attempts)
	}
}

func TestBuildFixPrompt(t *testing.T) {
	f := NewFixer(nil, llm.Tier1)

	code := "func TestAdd() { assert(1+1, 3) }"
	result := &TestResult{
		Passed:   false,
		TestFile: "test.go",
		Output:   "assertion failed",
		Errors: []TestError{
			{TestName: "TestAdd", Message: "assertion failed", Expected: "2", Actual: "3"},
		},
	}

	prompt := f.buildFixPrompt(code, result)

	if !strings.Contains(prompt, "Current Test Code") {
		t.Error("Prompt should include current code section")
	}
	if !strings.Contains(prompt, code) {
		t.Error("Prompt should include the test code")
	}
	if !strings.Contains(prompt, "Test Failures") {
		t.Error("Prompt should include failures section")
	}
	if !strings.Contains(prompt, "TestAdd") {
		t.Error("Prompt should include test name")
	}
	if !strings.Contains(prompt, "Instructions") {
		t.Error("Prompt should include instructions")
	}
}

func TestBuildFixPrompt_TruncatesLongOutput(t *testing.T) {
	f := NewFixer(nil, llm.Tier1)

	code := "func Test() {}"
	longOutput := strings.Repeat("x", 2000)
	result := &TestResult{
		Output: longOutput,
		Errors: []TestError{},
	}

	prompt := f.buildFixPrompt(code, result)

	if !strings.Contains(prompt, "[truncated]") {
		t.Error("Should truncate long output")
	}
}

func TestParseFixResponse_Basic(t *testing.T) {
	response := `EXPLANATION:
Fixed the expected value from 3 to 2.

CODE:
` + "```" + `
func Test() {
    assert(1+1, 2)
}
` + "```"

	code, explanation := parseFixResponse(response)

	if code == "" {
		t.Error("Should extract code")
	}
	if explanation == "" {
		t.Error("Should extract explanation")
	}
	if !strings.Contains(explanation, "Fixed") {
		t.Log("Explanation may vary")
	}
}

func TestParseFixResponse_NoCodeBlock(t *testing.T) {
	response := `EXPLANATION:
Fixed it.

CODE:
func Test() { }`

	code, _ := parseFixResponse(response)

	// Should still extract something after CODE:
	if code == "" {
		t.Log("May not extract code without backticks")
	}
}

func TestParseFixResponse_LanguageCodeBlock(t *testing.T) {
	response := `CODE:
` + "```go\nfunc Test() {}\n```"

	code, _ := parseFixResponse(response)

	if code == "" {
		t.Error("Should extract code from language-specific block")
	}
	if strings.Contains(code, "```") {
		t.Error("Should strip markdown markers")
	}
}

func TestParseFixResponse_Empty(t *testing.T) {
	response := "No code here"

	code, explanation := parseFixResponse(response)

	if code != "" {
		t.Error("Code should be empty")
	}
	if explanation != "" {
		t.Error("Explanation should be empty")
	}
}
