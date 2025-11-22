// Package validator provides test validation and quality analysis
package validator

import (
	"regexp"
	"strings"
)

// AssertionAnalysis contains analysis of assertions in test code
type AssertionAnalysis struct {
	TotalAssertions   int                 `json:"total_assertions"`
	AssertionsByTest  map[string]int      `json:"assertions_by_test"`
	TrivialAssertions int                 `json:"trivial_assertions"`
	AssertionTypes    map[string]int      `json:"assertion_types"`
	Issues            []AssertionIssue    `json:"issues,omitempty"`
	FunctionsCalled   []string            `json:"functions_called"`
	TargetFuncCalled  bool                `json:"target_func_called"`
}

// AssertionIssue represents a problem with an assertion
type AssertionIssue struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Line        int    `json:"line,omitempty"`
	TestName    string `json:"test_name,omitempty"`
}

// IssueTypes for assertion problems
const (
	IssueNoAssertions       = "no_assertions"
	IssueTrivialAssertion   = "trivial_assertion"
	IssueConstantComparison = "constant_comparison"
	IssueTautology          = "tautology"
	IssueTargetNotCalled    = "target_not_called"
)

// Analyzer performs static analysis on test code
type Analyzer struct {
	language       string
	targetFunction string
}

// NewAnalyzer creates a new test analyzer
func NewAnalyzer(language, targetFunction string) *Analyzer {
	return &Analyzer{
		language:       language,
		targetFunction: targetFunction,
	}
}

// AnalyzeAssertions analyzes test code for assertions and quality
func (a *Analyzer) AnalyzeAssertions(code string) *AssertionAnalysis {
	result := &AssertionAnalysis{
		AssertionsByTest: make(map[string]int),
		AssertionTypes:   make(map[string]int),
		Issues:           []AssertionIssue{},
		FunctionsCalled:  []string{},
	}

	switch a.language {
	case "go":
		a.analyzeGoAssertions(code, result)
	case "python":
		a.analyzePythonAssertions(code, result)
	case "javascript", "typescript":
		a.analyzeJSAssertions(code, result)
	}

	// Check if target function is called
	if a.targetFunction != "" {
		result.TargetFuncCalled = a.isTargetFunctionCalled(code)
		if !result.TargetFuncCalled {
			result.Issues = append(result.Issues, AssertionIssue{
				Type:        IssueTargetNotCalled,
				Description: "Target function '" + a.targetFunction + "' is never called in the test",
			})
		}
	}

	return result
}

// analyzeGoAssertions analyzes Go test assertions
func (a *Analyzer) analyzeGoAssertions(code string, result *AssertionAnalysis) {
	lines := strings.Split(code, "\n")

	// Patterns for Go assertions
	testFuncPattern := regexp.MustCompile(`func\s+(Test\w+)\s*\(`)
	assertPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\.(Equal|NotEqual|True|False|Nil|NotNil|Error|NoError|Contains|Len|Empty|NotEmpty|Greater|Less|Panics)\s*\(`),
		regexp.MustCompile(`t\.(Error|Errorf|Fatal|Fatalf|Fail|FailNow)\s*\(`),
		regexp.MustCompile(`if\s+.*\s*!=\s*.*\s*\{\s*t\.`),
	}
	// Patterns for trivial assertions
	trivialPatterns := []*regexp.Regexp{
		regexp.MustCompile(`Equal\s*\(\s*(\d+)\s*,\s*(\d+)\s*\)`),                     // Equal(5, 5)
		regexp.MustCompile(`Equal\s*\(\s*"([^"]+)"\s*,\s*"([^"]+)"\s*\)`),             // Equal("a", "a")
		regexp.MustCompile(`Equal\s*\(\s*(\w+)\s*,\s*(\1)\s*\)`),                      // Equal(x, x)
		regexp.MustCompile(`True\s*\(\s*true\s*\)`),                                   // True(true)
		regexp.MustCompile(`False\s*\(\s*false\s*\)`),                                 // False(false)
	}

	currentTest := ""
	for lineNum, line := range lines {
		// Track current test function
		if match := testFuncPattern.FindStringSubmatch(line); len(match) > 1 {
			currentTest = match[1]
			result.AssertionsByTest[currentTest] = 0
		}

		// Count assertions
		for _, pattern := range assertPatterns {
			if pattern.MatchString(line) {
				result.TotalAssertions++
				if currentTest != "" {
					result.AssertionsByTest[currentTest]++
				}

				// Categorize assertion type
				if strings.Contains(line, ".Equal") {
					result.AssertionTypes["equality"]++
				} else if strings.Contains(line, ".Error") || strings.Contains(line, ".NoError") {
					result.AssertionTypes["error"]++
				} else if strings.Contains(line, ".Nil") || strings.Contains(line, ".NotNil") {
					result.AssertionTypes["nil"]++
				} else if strings.Contains(line, ".True") || strings.Contains(line, ".False") {
					result.AssertionTypes["boolean"]++
				} else {
					result.AssertionTypes["other"]++
				}
			}
		}

		// Check for trivial assertions
		for _, pattern := range trivialPatterns {
			if matches := pattern.FindStringSubmatch(line); len(matches) > 0 {
				// Check if it's actually trivial (same values)
				if len(matches) >= 3 && matches[1] == matches[2] {
					result.TrivialAssertions++
					result.Issues = append(result.Issues, AssertionIssue{
						Type:        IssueTrivialAssertion,
						Description: "Trivial assertion comparing identical values",
						Line:        lineNum + 1,
						TestName:    currentTest,
					})
				}
			}
		}

		// Extract function calls
		funcCallPattern := regexp.MustCompile(`(\w+)\s*\(`)
		if matches := funcCallPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				if len(match) > 1 && !isGoKeyword(match[1]) {
					result.FunctionsCalled = append(result.FunctionsCalled, match[1])
				}
			}
		}
	}

	// Check for tests with no assertions
	for testName, count := range result.AssertionsByTest {
		if count == 0 {
			result.Issues = append(result.Issues, AssertionIssue{
				Type:        IssueNoAssertions,
				Description: "Test function has no assertions",
				TestName:    testName,
			})
		}
	}
}

// analyzePythonAssertions analyzes Python test assertions
func (a *Analyzer) analyzePythonAssertions(code string, result *AssertionAnalysis) {
	lines := strings.Split(code, "\n")

	testFuncPattern := regexp.MustCompile(`def\s+(test_\w+)\s*\(`)
	assertPatterns := []*regexp.Regexp{
		regexp.MustCompile(`assert\s+`),
		regexp.MustCompile(`self\.assert\w+\s*\(`),
		regexp.MustCompile(`pytest\.\w+\s*\(`),
	}
	trivialPatterns := []*regexp.Regexp{
		regexp.MustCompile(`assert\s+(\d+)\s*==\s*(\d+)`),
		regexp.MustCompile(`assert\s+True`),
		regexp.MustCompile(`assert\s+(\w+)\s*==\s*(\1)\s*$`),
	}

	currentTest := ""
	for lineNum, line := range lines {
		if match := testFuncPattern.FindStringSubmatch(line); len(match) > 1 {
			currentTest = match[1]
			result.AssertionsByTest[currentTest] = 0
		}

		for _, pattern := range assertPatterns {
			if pattern.MatchString(line) {
				result.TotalAssertions++
				if currentTest != "" {
					result.AssertionsByTest[currentTest]++
				}

				if strings.Contains(line, "==") {
					result.AssertionTypes["equality"]++
				} else if strings.Contains(line, "raises") {
					result.AssertionTypes["error"]++
				} else if strings.Contains(line, "None") {
					result.AssertionTypes["nil"]++
				} else {
					result.AssertionTypes["other"]++
				}
			}
		}

		for _, pattern := range trivialPatterns {
			if matches := pattern.FindStringSubmatch(line); len(matches) > 0 {
				if len(matches) >= 3 && matches[1] == matches[2] {
					result.TrivialAssertions++
					result.Issues = append(result.Issues, AssertionIssue{
						Type:        IssueTrivialAssertion,
						Description: "Trivial assertion comparing identical values",
						Line:        lineNum + 1,
						TestName:    currentTest,
					})
				}
			}
		}

		funcCallPattern := regexp.MustCompile(`(\w+)\s*\(`)
		if matches := funcCallPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				if len(match) > 1 && !isPythonKeyword(match[1]) {
					result.FunctionsCalled = append(result.FunctionsCalled, match[1])
				}
			}
		}
	}

	for testName, count := range result.AssertionsByTest {
		if count == 0 {
			result.Issues = append(result.Issues, AssertionIssue{
				Type:        IssueNoAssertions,
				Description: "Test function has no assertions",
				TestName:    testName,
			})
		}
	}
}

// analyzeJSAssertions analyzes JavaScript/TypeScript test assertions
func (a *Analyzer) analyzeJSAssertions(code string, result *AssertionAnalysis) {
	lines := strings.Split(code, "\n")

	testFuncPattern := regexp.MustCompile(`(it|test)\s*\(\s*['"]([^'"]+)['"]`)
	assertPatterns := []*regexp.Regexp{
		regexp.MustCompile(`expect\s*\(.+\)\.(toBe|toEqual|toStrictEqual|toBeTruthy|toBeFalsy|toBeNull|toBeUndefined|toThrow|toContain|toHaveLength)\s*\(`),
		regexp.MustCompile(`assert\.\w+\s*\(`),
	}
	trivialPatterns := []*regexp.Regexp{
		regexp.MustCompile(`expect\s*\(\s*(\d+)\s*\)\.toBe\s*\(\s*(\d+)\s*\)`),
		regexp.MustCompile(`expect\s*\(\s*true\s*\)\.toBe\s*\(\s*true\s*\)`),
		regexp.MustCompile(`expect\s*\(\s*(\w+)\s*\)\.toBe\s*\(\s*(\1)\s*\)`),
	}

	currentTest := ""
	for lineNum, line := range lines {
		if match := testFuncPattern.FindStringSubmatch(line); len(match) > 2 {
			currentTest = match[2]
			result.AssertionsByTest[currentTest] = 0
		}

		for _, pattern := range assertPatterns {
			if pattern.MatchString(line) {
				result.TotalAssertions++
				if currentTest != "" {
					result.AssertionsByTest[currentTest]++
				}

				if strings.Contains(line, "toBe") || strings.Contains(line, "toEqual") {
					result.AssertionTypes["equality"]++
				} else if strings.Contains(line, "toThrow") {
					result.AssertionTypes["error"]++
				} else if strings.Contains(line, "toBeNull") || strings.Contains(line, "toBeUndefined") {
					result.AssertionTypes["nil"]++
				} else if strings.Contains(line, "toBeTruthy") || strings.Contains(line, "toBeFalsy") {
					result.AssertionTypes["boolean"]++
				} else {
					result.AssertionTypes["other"]++
				}
			}
		}

		for _, pattern := range trivialPatterns {
			if matches := pattern.FindStringSubmatch(line); len(matches) > 0 {
				if len(matches) >= 3 && matches[1] == matches[2] {
					result.TrivialAssertions++
					result.Issues = append(result.Issues, AssertionIssue{
						Type:        IssueTrivialAssertion,
						Description: "Trivial assertion comparing identical values",
						Line:        lineNum + 1,
						TestName:    currentTest,
					})
				}
			}
		}

		funcCallPattern := regexp.MustCompile(`(\w+)\s*\(`)
		if matches := funcCallPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				if len(match) > 1 && !isJSKeyword(match[1]) {
					result.FunctionsCalled = append(result.FunctionsCalled, match[1])
				}
			}
		}
	}

	for testName, count := range result.AssertionsByTest {
		if count == 0 {
			result.Issues = append(result.Issues, AssertionIssue{
				Type:        IssueNoAssertions,
				Description: "Test function has no assertions",
				TestName:    testName,
			})
		}
	}
}

// isTargetFunctionCalled checks if the target function is called in the test
func (a *Analyzer) isTargetFunctionCalled(code string) bool {
	if a.targetFunction == "" {
		return true // No target specified, assume OK
	}

	// Simple check - look for function name followed by (
	pattern := regexp.MustCompile(regexp.QuoteMeta(a.targetFunction) + `\s*\(`)
	return pattern.MatchString(code)
}

// Helper functions to filter keywords
func isGoKeyword(s string) bool {
	keywords := map[string]bool{
		"if": true, "else": true, "for": true, "range": true, "func": true,
		"return": true, "var": true, "const": true, "type": true, "struct": true,
		"interface": true, "package": true, "import": true, "switch": true,
		"case": true, "default": true, "go": true, "defer": true, "select": true,
		"chan": true, "map": true, "make": true, "new": true, "append": true,
		"len": true, "cap": true, "copy": true, "delete": true, "panic": true,
		"recover": true, "close": true, "nil": true, "true": true, "false": true,
	}
	return keywords[s]
}

func isPythonKeyword(s string) bool {
	keywords := map[string]bool{
		"if": true, "else": true, "elif": true, "for": true, "while": true,
		"def": true, "class": true, "return": true, "import": true, "from": true,
		"try": true, "except": true, "finally": true, "with": true, "as": true,
		"pass": true, "break": true, "continue": true, "raise": true, "yield": true,
		"lambda": true, "and": true, "or": true, "not": true, "in": true, "is": true,
		"None": true, "True": true, "False": true, "print": true, "len": true,
		"range": true, "assert": true, "self": true,
	}
	return keywords[s]
}

func isJSKeyword(s string) bool {
	keywords := map[string]bool{
		"if": true, "else": true, "for": true, "while": true, "do": true,
		"function": true, "return": true, "var": true, "let": true, "const": true,
		"class": true, "new": true, "this": true, "super": true, "import": true,
		"export": true, "default": true, "try": true, "catch": true, "finally": true,
		"throw": true, "async": true, "await": true, "typeof": true, "instanceof": true,
		"null": true, "undefined": true, "true": true, "false": true, "describe": true,
		"it": true, "test": true, "expect": true, "beforeEach": true, "afterEach": true,
	}
	return keywords[s]
}

// ValidateMinimumAssertions checks if test has minimum required assertions
func (a *Analyzer) ValidateMinimumAssertions(analysis *AssertionAnalysis, minRequired int) (bool, string) {
	if analysis.TotalAssertions < minRequired {
		return false, "Test has insufficient assertions: " +
			string(rune(analysis.TotalAssertions)) + " found, " +
			string(rune(minRequired)) + " required"
	}

	// Check each test function has at least one assertion
	for testName, count := range analysis.AssertionsByTest {
		if count == 0 {
			return false, "Test function '" + testName + "' has no assertions"
		}
	}

	return true, ""
}

// HasCriticalIssues checks if there are blocking issues
func (a *Analyzer) HasCriticalIssues(analysis *AssertionAnalysis) (bool, []string) {
	var criticalIssues []string

	for _, issue := range analysis.Issues {
		switch issue.Type {
		case IssueNoAssertions:
			criticalIssues = append(criticalIssues, issue.Description)
		case IssueTargetNotCalled:
			criticalIssues = append(criticalIssues, issue.Description)
		}
	}

	// Too many trivial assertions is also critical
	if analysis.TrivialAssertions > 0 && analysis.TrivialAssertions >= analysis.TotalAssertions/2 {
		criticalIssues = append(criticalIssues, "More than half of assertions are trivial")
	}

	return len(criticalIssues) > 0, criticalIssues
}
