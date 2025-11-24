package e2e

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/api"
	"github.com/QTest-hq/qtest/internal/flow"
)

func TestDefaultTestConfig(t *testing.T) {
	config := DefaultTestConfig()

	if config.Framework != FrameworkPlaywright {
		t.Errorf("Framework = %v, want %v", config.Framework, FrameworkPlaywright)
	}
	if config.Language != LanguageTypeScript {
		t.Errorf("Language = %v, want %v", config.Language, LanguageTypeScript)
	}
	if !config.Headless {
		t.Error("Headless should be true by default")
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", config.Timeout)
	}
	if config.Screenshots != "only-on-failure" {
		t.Errorf("Screenshots = %q, want %q", config.Screenshots, "only-on-failure")
	}
}

func TestDefaultGenerationConfig(t *testing.T) {
	config := DefaultGenerationConfig()

	if config.Framework != FrameworkPlaywright {
		t.Errorf("Framework = %v, want %v", config.Framework, FrameworkPlaywright)
	}
	if config.Language != LanguageTypeScript {
		t.Errorf("Language = %v, want %v", config.Language, LanguageTypeScript)
	}
	if config.OutputDir != "tests" {
		t.Errorf("OutputDir = %q, want %q", config.OutputDir, "tests")
	}
	if !config.GroupByFeature {
		t.Error("GroupByFeature should be true by default")
	}
	if !config.IncludeComments {
		t.Error("IncludeComments should be true by default")
	}
	if config.MaxStepsPerTest != 20 {
		t.Errorf("MaxStepsPerTest = %d, want 20", config.MaxStepsPerTest)
	}
}

func TestTestFrameworks(t *testing.T) {
	tests := []struct {
		framework TestFramework
		expected  string
	}{
		{FrameworkPlaywright, "playwright"},
		{FrameworkCypress, "cypress"},
		{FrameworkPuppeteer, "puppeteer"},
	}

	for _, tt := range tests {
		if string(tt.framework) != tt.expected {
			t.Errorf("TestFramework %v = %q, want %q", tt.framework, string(tt.framework), tt.expected)
		}
	}
}

func TestTestLanguages(t *testing.T) {
	tests := []struct {
		lang     TestLanguage
		expected string
	}{
		{LanguageTypeScript, "typescript"},
		{LanguageJavaScript, "javascript"},
		{LanguagePython, "python"},
	}

	for _, tt := range tests {
		if string(tt.lang) != tt.expected {
			t.Errorf("TestLanguage %v = %q, want %q", tt.lang, string(tt.lang), tt.expected)
		}
	}
}

func TestAssertionTypes(t *testing.T) {
	tests := []struct {
		at       AssertionType
		expected string
	}{
		{AssertVisible, "visible"},
		{AssertHidden, "hidden"},
		{AssertText, "text"},
		{AssertValue, "value"},
		{AssertEnabled, "enabled"},
		{AssertDisabled, "disabled"},
		{AssertURL, "url"},
		{AssertTitle, "title"},
		{AssertCount, "count"},
		{AssertContains, "contains"},
		{AssertStatusCode, "status_code"},
	}

	for _, tt := range tests {
		if string(tt.at) != tt.expected {
			t.Errorf("AssertionType %v = %q, want %q", tt.at, string(tt.at), tt.expected)
		}
	}
}

func TestPlaywrightGenerator_Generate(t *testing.T) {
	gen := NewPlaywrightGenerator(nil)

	spec := &E2ETestSpec{
		ID:      "test-spec",
		Name:    "Login Flow Tests",
		BaseURL: "https://example.com",
		TestCases: []TestCase{
			{
				ID:   "tc-1",
				Name: "Successful login",
				Steps: []TestStep{
					{
						ID:    "step-1",
						Name:  "Navigate to login",
						Order: 1,
						Action: TestAction{
							Type: flow.ActionTypeNavigate,
							URL:  "/login",
						},
					},
					{
						ID:    "step-2",
						Name:  "Fill email",
						Order: 2,
						Action: TestAction{
							Type:     flow.ActionTypeFill,
							Selector: "#email",
							Value:    "test@example.com",
						},
					},
					{
						ID:    "step-3",
						Name:  "Fill password",
						Order: 3,
						Action: TestAction{
							Type:     flow.ActionTypeFill,
							Selector: "#password",
							Value:    "password123",
						},
					},
					{
						ID:    "step-4",
						Name:  "Click submit",
						Order: 4,
						Action: TestAction{
							Type:     flow.ActionTypeClick,
							Selector: "button[type='submit']",
						},
					},
				},
				Expected: []Assertion{
					{
						Type:     AssertURL,
						Expected: "https://example.com/dashboard",
					},
				},
			},
		},
	}

	result, err := gen.Generate(spec)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if len(result.Files) == 0 {
		t.Fatal("Expected at least one generated file")
	}

	if result.TestCount != 1 {
		t.Errorf("TestCount = %d, want 1", result.TestCount)
	}

	if result.StepCount != 4 {
		t.Errorf("StepCount = %d, want 4", result.StepCount)
	}

	// Check generated content
	content := result.Files[0].Content
	if !strings.Contains(content, "test.describe('Login Flow Tests'") {
		t.Error("Generated code should contain test.describe")
	}
	if !strings.Contains(content, "test('Successful login'") {
		t.Error("Generated code should contain test case")
	}
	if !strings.Contains(content, "page.goto") {
		t.Error("Generated code should contain page.goto")
	}
	if !strings.Contains(content, "page.fill") {
		t.Error("Generated code should contain page.fill")
	}
	if !strings.Contains(content, "page.click") {
		t.Error("Generated code should contain page.click")
	}
}

func TestPlaywrightGenerator_GenerateAction(t *testing.T) {
	gen := NewPlaywrightGenerator(nil)

	tests := []struct {
		action   TestAction
		expected string
	}{
		{
			action:   TestAction{Type: flow.ActionTypeClick, Selector: "#btn"},
			expected: "await page.click('#btn');",
		},
		{
			action:   TestAction{Type: flow.ActionTypeFill, Selector: "#input", Value: "test"},
			expected: "await page.fill('#input', 'test');",
		},
		{
			action:   TestAction{Type: flow.ActionTypeNavigate, URL: "https://example.com"},
			expected: "await page.goto('https://example.com');",
		},
		{
			action:   TestAction{Type: flow.ActionTypeSelect, Selector: "#dropdown", Value: "option1"},
			expected: "await page.selectOption('#dropdown', 'option1');",
		},
		{
			action:   TestAction{Type: flow.ActionTypeCheck, Selector: "#checkbox"},
			expected: "await page.check('#checkbox');",
		},
		{
			action:   TestAction{Type: flow.ActionTypeHover, Selector: "#element"},
			expected: "await page.hover('#element');",
		},
		{
			action:   TestAction{Type: flow.ActionTypeKeypress, Key: "Enter"},
			expected: "await page.keyboard.press('Enter');",
		},
		{
			action:   TestAction{Type: flow.ActionTypeKeypress, Key: "a", Modifiers: []string{"Control"}},
			expected: "await page.keyboard.press('Control+a');",
		},
	}

	for _, tt := range tests {
		result := gen.generateAction(&tt.action)
		if result != tt.expected {
			t.Errorf("generateAction(%v) = %q, want %q", tt.action.Type, result, tt.expected)
		}
	}
}

func TestPlaywrightGenerator_GenerateAssertion(t *testing.T) {
	gen := NewPlaywrightGenerator(nil)

	tests := []struct {
		assertion Assertion
		expected  string
	}{
		{
			assertion: Assertion{Type: AssertVisible, Selector: "#element"},
			expected:  "await expect(page.locator('#element')).toBeVisible();",
		},
		{
			assertion: Assertion{Type: AssertHidden, Selector: "#element"},
			expected:  "await expect(page.locator('#element')).toBeHidden();",
		},
		{
			assertion: Assertion{Type: AssertText, Selector: "#title", Expected: "Hello"},
			expected:  "await expect(page.locator('#title')).toHaveText('Hello');",
		},
		{
			assertion: Assertion{Type: AssertURL, Expected: "https://example.com"},
			expected:  "await expect(page).toHaveURL('https://example.com');",
		},
		{
			assertion: Assertion{Type: AssertTitle, Expected: "Page Title"},
			expected:  "await expect(page).toHaveTitle('Page Title');",
		},
		{
			assertion: Assertion{Type: AssertCount, Selector: ".item", Expected: 5},
			expected:  "await expect(page.locator('.item')).toHaveCount(5);",
		},
	}

	for _, tt := range tests {
		result := gen.generateAssertion(&tt.assertion)
		if result != tt.expected {
			t.Errorf("generateAssertion(%v) = %q, want %q", tt.assertion.Type, result, tt.expected)
		}
	}
}

func TestPlaywrightGenerator_GenerateFileName(t *testing.T) {
	gen := NewPlaywrightGenerator(nil)

	tests := []struct {
		name     string
		expected string
	}{
		{"Login Flow", "login-flow.spec.ts"},
		{"User Registration", "user-registration.spec.ts"},
		{"API Tests", "api-tests.spec.ts"},
		{"Test_With_Underscores", "test-with-underscores.spec.ts"},
	}

	for _, tt := range tests {
		result := gen.generateFileName(tt.name)
		if result != tt.expected {
			t.Errorf("generateFileName(%q) = %q, want %q", tt.name, result, tt.expected)
		}
	}
}

func TestPlaywrightGenerator_JavaScript(t *testing.T) {
	config := DefaultGenerationConfig()
	config.Language = LanguageJavaScript
	gen := NewPlaywrightGenerator(config)

	spec := &E2ETestSpec{
		ID:      "test",
		Name:    "Test",
		BaseURL: "https://example.com",
		TestCases: []TestCase{
			{
				ID:    "tc-1",
				Name:  "Test case",
				Steps: []TestStep{{ID: "s1", Order: 1, Action: TestAction{Type: flow.ActionTypeClick, Selector: "#btn"}}},
			},
		},
	}

	result, err := gen.Generate(spec)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	content := result.Files[0].Content
	if !strings.Contains(content, "require('@playwright/test')") {
		t.Error("JavaScript should use require for imports")
	}
	if strings.Contains(content, ": Page") {
		t.Error("JavaScript should not have TypeScript type annotations")
	}
}

func TestFlowToSpec(t *testing.T) {
	f := &flow.Flow{
		ID:       "flow-1",
		Name:     "Login Flow",
		Type:     flow.FlowTypeLogin,
		StartURL: "https://example.com/login",
		Steps: []flow.Step{
			{
				ID:    "step-1",
				Name:  "Fill email",
				Order: 1,
				Action: flow.Action{
					Type:     flow.ActionTypeFill,
					Selector: &flow.Selector{Primary: "#email"},
					Value:    "${username}",
				},
			},
			{
				ID:    "step-2",
				Name:  "Click submit",
				Order: 2,
				Action: flow.Action{
					Type:     flow.ActionTypeClick,
					Selector: &flow.Selector{Primary: "button[type='submit']"},
				},
			},
		},
	}

	credentials := map[string]string{
		"username": "test@example.com",
	}

	spec := FlowToSpec(f, credentials)

	if spec.Name != "Login Flow" {
		t.Errorf("Name = %q, want %q", spec.Name, "Login Flow")
	}

	if spec.BaseURL != "https://example.com/login" {
		t.Errorf("BaseURL = %q, want %q", spec.BaseURL, "https://example.com/login")
	}

	if len(spec.TestCases) != 1 {
		t.Fatalf("TestCases count = %d, want 1", len(spec.TestCases))
	}

	tc := spec.TestCases[0]
	if len(tc.Steps) != 2 {
		t.Errorf("Steps count = %d, want 2", len(tc.Steps))
	}

	// Check credential substitution
	if tc.Steps[0].Action.Value != "test@example.com" {
		t.Errorf("Credential substitution failed: got %q, want %q", tc.Steps[0].Action.Value, "test@example.com")
	}
}

func TestAPITestGenerator_GenerateFromSpec(t *testing.T) {
	gen := NewAPITestGenerator(nil)

	apiSpec := &api.APISpec{
		ID:      "api-1",
		Name:    "Users API",
		BaseURL: "https://api.example.com",
		Endpoints: []api.Endpoint{
			{
				ID:      "ep-1",
				Path:    "/users",
				Method:  api.MethodGET,
				Summary: "List users",
				Responses: []api.Response{
					{StatusCode: 200, Description: "Success"},
				},
			},
			{
				ID:      "ep-2",
				Path:    "/users",
				Method:  api.MethodPOST,
				Summary: "Create user",
				RequestBody: &api.RequestBody{
					ContentType: api.ContentTypeJSON,
					Schema: &api.Schema{
						Type: api.DataTypeObject,
						Properties: map[string]*api.Schema{
							"name":  {Type: api.DataTypeString},
							"email": {Type: api.DataTypeString, Format: "email"},
						},
					},
				},
				Responses: []api.Response{
					{StatusCode: 201, Description: "Created"},
				},
			},
		},
	}

	result, err := gen.GenerateFromSpec(apiSpec)
	if err != nil {
		t.Fatalf("GenerateFromSpec() error: %v", err)
	}

	if len(result.Files) == 0 {
		t.Fatal("Expected at least one generated file")
	}

	if result.TestCount == 0 {
		t.Error("Expected at least one test case")
	}

	content := result.Files[0].Content
	if !strings.Contains(content, "BASE_URL") {
		t.Error("Generated code should contain BASE_URL constant")
	}
	if !strings.Contains(content, "request.get") {
		t.Error("Generated code should contain GET request")
	}
	if !strings.Contains(content, "request.post") {
		t.Error("Generated code should contain POST request")
	}
}

func TestAPITestGenerator_GenerateSampleValue(t *testing.T) {
	gen := NewAPITestGenerator(nil)

	tests := []struct {
		schema   *api.Schema
		expected interface{}
	}{
		{&api.Schema{Type: api.DataTypeString}, "string"},
		{&api.Schema{Type: api.DataTypeString, Format: "email"}, "test@example.com"},
		{&api.Schema{Type: api.DataTypeInteger}, 1},
		{&api.Schema{Type: api.DataTypeNumber}, 1.0},
		{&api.Schema{Type: api.DataTypeBoolean}, true},
		{&api.Schema{Type: api.DataTypeString, Example: "custom"}, "custom"},
	}

	for _, tt := range tests {
		result := gen.generateSampleValue(tt.schema)
		if result != tt.expected {
			t.Errorf("generateSampleValue(%v) = %v, want %v", tt.schema.Type, result, tt.expected)
		}
	}
}

func TestEscapeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"it's", "it\\'s"},
		{"path\\to\\file", "path\\\\to\\\\file"},
		{"test's \"value\"", "test\\'s \"value\""},
	}

	for _, tt := range tests {
		result := escapeString(tt.input)
		if result != tt.expected {
			t.Errorf("escapeString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestEscapeSelector(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"#id", "#id"},
		{".class", ".class"},
		{"[data-test='value']", "[data-test=\\'value\\']"},
		{"button:contains('Submit')", "button:contains(\\'Submit\\')"},
	}

	for _, tt := range tests {
		result := escapeSelector(tt.input)
		if result != tt.expected {
			t.Errorf("escapeSelector(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMapAssertionType(t *testing.T) {
	tests := []struct {
		flowType string
		expected AssertionType
	}{
		{"visible", AssertVisible},
		{"hidden", AssertHidden},
		{"text", AssertText},
		{"value", AssertValue},
		{"url", AssertURL},
		{"title", AssertTitle},
	}

	for _, tt := range tests {
		result := mapAssertionType(tt.flowType)
		if result != tt.expected {
			t.Errorf("mapAssertionType(%v) = %v, want %v", tt.flowType, result, tt.expected)
		}
	}
}

// LLM Enhancer Tests

func TestDefaultEnhancerConfig(t *testing.T) {
	config := DefaultEnhancerConfig()

	if config.MaxSuggestionsPerTest != 5 {
		t.Errorf("MaxSuggestionsPerTest = %d, want 5", config.MaxSuggestionsPerTest)
	}
	if !config.GenerateNegativeTests {
		t.Error("GenerateNegativeTests should be true by default")
	}
	if !config.GenerateEdgeCases {
		t.Error("GenerateEdgeCases should be true by default")
	}
	if !config.ImproveDescriptions {
		t.Error("ImproveDescriptions should be true by default")
	}
	if config.AddAccessibilityTests {
		t.Error("AddAccessibilityTests should be false by default")
	}
}

func TestNewLLMEnhancer(t *testing.T) {
	// Test with nil config
	enhancer := NewLLMEnhancer(nil, nil)
	if enhancer == nil {
		t.Fatal("NewLLMEnhancer returned nil")
	}
	if enhancer.config == nil {
		t.Error("config should not be nil")
	}

	// Test with custom config
	customConfig := &EnhancerConfig{
		MaxSuggestionsPerTest: 10,
		GenerateNegativeTests: false,
	}
	enhancer = NewLLMEnhancer(nil, customConfig)
	if enhancer.config.MaxSuggestionsPerTest != 10 {
		t.Errorf("MaxSuggestionsPerTest = %d, want 10", enhancer.config.MaxSuggestionsPerTest)
	}
}

func TestExtractJSONArray(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `Here is the result: [{"name": "test"}]`,
			expected: `[{"name": "test"}]`,
		},
		{
			input:    `[{"a": 1}, {"b": 2}]`,
			expected: `[{"a": 1}, {"b": 2}]`,
		},
		{
			input:    `No array here`,
			expected: "",
		},
		{
			input:    `Nested [[1, 2], [3, 4]]`,
			expected: `[[1, 2], [3, 4]]`,
		},
	}

	for _, tt := range tests {
		result := extractJSONArray(tt.input)
		if result != tt.expected {
			t.Errorf("extractJSONArray(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestLLMEnhancer_ParseTestCaseSuggestions(t *testing.T) {
	enhancer := NewLLMEnhancer(nil, nil)

	response := `Here are suggested tests:
[
  {"name": "Test Login", "description": "Test user login", "tags": ["auth"]},
  {"name": "Test Logout", "description": "Test user logout", "tags": ["auth"]}
]`

	tests, err := enhancer.parseTestCaseSuggestions(response)
	if err != nil {
		t.Fatalf("parseTestCaseSuggestions error: %v", err)
	}

	if len(tests) != 2 {
		t.Fatalf("len(tests) = %d, want 2", len(tests))
	}

	if tests[0].Name != "Test Login" {
		t.Errorf("tests[0].Name = %q, want %q", tests[0].Name, "Test Login")
	}
	if tests[0].Description != "Test user login" {
		t.Errorf("tests[0].Description = %q, want %q", tests[0].Description, "Test user login")
	}
	if len(tests[0].Tags) != 1 || tests[0].Tags[0] != "auth" {
		t.Errorf("tests[0].Tags = %v, want [auth]", tests[0].Tags)
	}
}

func TestLLMEnhancer_ParseAssertionSuggestions(t *testing.T) {
	enhancer := NewLLMEnhancer(nil, nil)

	response := `[
  {"type": "visible", "selector": "#submit", "expected": null},
  {"type": "text", "selector": ".message", "expected": "Success"}
]`

	assertions, err := enhancer.parseAssertionSuggestions(response)
	if err != nil {
		t.Fatalf("parseAssertionSuggestions error: %v", err)
	}

	if len(assertions) != 2 {
		t.Fatalf("len(assertions) = %d, want 2", len(assertions))
	}

	if assertions[0].Type != AssertVisible {
		t.Errorf("assertions[0].Type = %q, want %q", assertions[0].Type, AssertVisible)
	}
	if assertions[0].Selector != "#submit" {
		t.Errorf("assertions[0].Selector = %q, want %q", assertions[0].Selector, "#submit")
	}

	if assertions[1].Type != AssertText {
		t.Errorf("assertions[1].Type = %q, want %q", assertions[1].Type, AssertText)
	}
	if assertions[1].Expected != "Success" {
		t.Errorf("assertions[1].Expected = %v, want %q", assertions[1].Expected, "Success")
	}
}

func TestLLMEnhancer_BuildPrompts(t *testing.T) {
	enhancer := NewLLMEnhancer(nil, nil)

	spec := &E2ETestSpec{
		Name:    "Login Tests",
		BaseURL: "https://example.com",
		TestCases: []TestCase{
			{
				ID:          "test-1",
				Name:        "Test Login",
				Description: "Test user login flow",
			},
		},
	}

	// Test suggestion prompt
	suggestionPrompt := enhancer.buildSuggestionPrompt(spec)
	if !strings.Contains(suggestionPrompt, "Login Tests") {
		t.Error("Suggestion prompt should contain test suite name")
	}
	if !strings.Contains(suggestionPrompt, "Test Login") {
		t.Error("Suggestion prompt should contain existing test name")
	}

	// Test negative test prompt
	negativePrompt := enhancer.buildNegativeTestPrompt(spec)
	if !strings.Contains(negativePrompt, "negative") {
		t.Error("Negative test prompt should mention 'negative'")
	}

	// Test edge case prompt
	edgeCasePrompt := enhancer.buildEdgeCasePrompt(spec)
	if !strings.Contains(edgeCasePrompt, "edge case") {
		t.Error("Edge case prompt should mention 'edge case'")
	}
}

// Test Runner Tests

func TestDefaultRunnerConfig(t *testing.T) {
	config := DefaultRunnerConfig()

	if config.WorkDir != "." {
		t.Errorf("WorkDir = %q, want %q", config.WorkDir, ".")
	}
	if config.TestDir != "tests" {
		t.Errorf("TestDir = %q, want %q", config.TestDir, "tests")
	}
	if config.OutputDir != "test-results" {
		t.Errorf("OutputDir = %q, want %q", config.OutputDir, "test-results")
	}
	if !config.Headless {
		t.Error("Headless should be true by default")
	}
	if config.Retries != 2 {
		t.Errorf("Retries = %d, want 2", config.Retries)
	}
	if config.Workers != 4 {
		t.Errorf("Workers = %d, want 4", config.Workers)
	}
	if config.Reporter != "json" {
		t.Errorf("Reporter = %q, want %q", config.Reporter, "json")
	}
	if config.Browser != "chromium" {
		t.Errorf("Browser = %q, want %q", config.Browser, "chromium")
	}
}

func TestNewTestRunner(t *testing.T) {
	// Test with nil config
	runner := NewTestRunner(nil)
	if runner == nil {
		t.Fatal("NewTestRunner returned nil")
	}
	if runner.config == nil {
		t.Error("config should not be nil")
	}

	// Test with custom config
	customConfig := &RunnerConfig{
		WorkDir:  "/custom",
		Headless: false,
		Workers:  8,
	}
	runner = NewTestRunner(customConfig)
	if runner.config.WorkDir != "/custom" {
		t.Errorf("WorkDir = %q, want %q", runner.config.WorkDir, "/custom")
	}
	if runner.config.Workers != 8 {
		t.Errorf("Workers = %d, want 8", runner.config.Workers)
	}
}

func TestTestRunner_BuildPlaywrightArgs(t *testing.T) {
	runner := NewTestRunner(&RunnerConfig{
		OutputDir: "results",
		Headless:  true,
		Retries:   3,
		Workers:   2,
		Browser:   "firefox",
		BaseURL:   "http://localhost:3000",
	})

	args := runner.buildPlaywrightArgs("")
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "playwright test") {
		t.Error("Should contain 'playwright test'")
	}
	if !strings.Contains(argsStr, "--output results") {
		t.Error("Should contain output directory")
	}
	if !strings.Contains(argsStr, "--retries 3") {
		t.Error("Should contain retries")
	}
	if !strings.Contains(argsStr, "--workers 2") {
		t.Error("Should contain workers")
	}
	if !strings.Contains(argsStr, "--browser firefox") {
		t.Error("Should contain browser")
	}
	if !strings.Contains(argsStr, "--base-url http://localhost:3000") {
		t.Error("Should contain base URL")
	}
}

func TestTestRunner_BuildPlaywrightArgs_WithPattern(t *testing.T) {
	runner := NewTestRunner(DefaultRunnerConfig())

	// Test with file pattern
	args := runner.buildPlaywrightArgs("login.spec.ts")
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "login.spec.ts") {
		t.Error("Should contain file pattern")
	}

	// Test with grep pattern
	args = runner.buildPlaywrightArgs("login test")
	argsStr = strings.Join(args, " ")
	if !strings.Contains(argsStr, "--grep") {
		t.Error("Should contain --grep for non-file patterns")
	}
}

func TestCategorizeFailure(t *testing.T) {
	tests := []struct {
		errorMsg string
		expected string
	}{
		{"Timeout of 30000ms exceeded", "timeout"},
		{"waiting for selector timed out", "timeout"},
		{"expect(received).toBe(expected)", "assertion"},
		{"locator.click: Target closed", "selector"},
		{"selector not found", "selector"},
		{"network request failed", "network"},
		{"connection refused", "network"},
		{"some unknown error", "unknown"},
	}

	for _, tt := range tests {
		result := categorizeFailure(tt.errorMsg)
		if result != tt.expected {
			t.Errorf("categorizeFailure(%q) = %q, want %q", tt.errorMsg, result, tt.expected)
		}
	}
}

func TestTestRunner_ValidateResults(t *testing.T) {
	runner := NewTestRunner(nil)

	result := &RunResult{
		TotalTests: 10,
		Passed:     7,
		Failed:     2,
		Skipped:    1,
		Duration:   30 * time.Second,
		Tests: []TestResult{
			{Name: "Test 1", Status: "passed"},
			{Name: "Test 2", Status: "failed", Error: "Timeout exceeded"},
			{Name: "Test 3", Status: "failed", Error: "expect(1).toBe(2)"},
		},
	}

	summary := runner.ValidateResults(result)

	if summary.TotalTests != 10 {
		t.Errorf("TotalTests = %d, want 10", summary.TotalTests)
	}
	if summary.PassedTests != 7 {
		t.Errorf("PassedTests = %d, want 7", summary.PassedTests)
	}
	if summary.PassRate != 70 {
		t.Errorf("PassRate = %f, want 70", summary.PassRate)
	}
	if len(summary.Failures) != 2 {
		t.Errorf("len(Failures) = %d, want 2", len(summary.Failures))
	}
	if summary.Failures[0].Category != "timeout" {
		t.Errorf("Failures[0].Category = %q, want %q", summary.Failures[0].Category, "timeout")
	}
	if summary.Failures[1].Category != "assertion" {
		t.Errorf("Failures[1].Category = %q, want %q", summary.Failures[1].Category, "assertion")
	}
}

func TestTestRunner_GenerateMarkdownReport(t *testing.T) {
	runner := NewTestRunner(nil)

	result := &RunResult{
		Success:    true,
		TotalTests: 3,
		Passed:     3,
		Failed:     0,
		Duration:   5 * time.Second,
		Tests: []TestResult{
			{Name: "Test 1", Status: "passed", Duration: time.Second},
			{Name: "Test 2", Status: "passed", Duration: 2 * time.Second},
			{Name: "Test 3", Status: "passed", Duration: 2 * time.Second},
		},
	}

	report, err := runner.GenerateReport(result, "markdown")
	if err != nil {
		t.Fatalf("GenerateReport error: %v", err)
	}

	if !strings.Contains(report, "# E2E Test Report") {
		t.Error("Report should contain title")
	}
	if !strings.Contains(report, "**Total Tests:** 3") {
		t.Error("Report should contain total tests")
	}
	if !strings.Contains(report, "All tests passed") {
		t.Error("Report should indicate success")
	}
	if !strings.Contains(report, "Test 1") {
		t.Error("Report should contain test names")
	}
}

func TestTestRunner_GenerateJSONReport(t *testing.T) {
	runner := NewTestRunner(nil)

	result := &RunResult{
		Success:    false,
		TotalTests: 2,
		Passed:     1,
		Failed:     1,
		Tests: []TestResult{
			{Name: "Test 1", Status: "passed"},
			{Name: "Test 2", Status: "failed", Error: "error"},
		},
	}

	report, err := runner.GenerateReport(result, "json")
	if err != nil {
		t.Fatalf("GenerateReport error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(report), &parsed); err != nil {
		t.Fatalf("Report is not valid JSON: %v", err)
	}

	if parsed["totalTests"].(float64) != 2 {
		t.Error("JSON should contain totalTests")
	}
}

func TestTestRunner_GenerateHTMLReport(t *testing.T) {
	runner := NewTestRunner(nil)

	result := &RunResult{
		TotalTests: 1,
		Passed:     1,
		Tests: []TestResult{
			{Name: "Test 1", Status: "passed"},
		},
	}

	report, err := runner.GenerateReport(result, "html")
	if err != nil {
		t.Fatalf("GenerateReport error: %v", err)
	}

	if !strings.Contains(report, "<!DOCTYPE html>") {
		t.Error("Report should be valid HTML")
	}
	if !strings.Contains(report, "E2E Test Report") {
		t.Error("Report should contain title")
	}
	if !strings.Contains(report, "<table>") {
		t.Error("Report should contain table")
	}
}

func TestTestRunner_ParseStdoutResults(t *testing.T) {
	runner := NewTestRunner(nil)
	result := &RunResult{}

	stdout := `
  Running 5 tests using 2 workers

  ✓ login.spec.ts:5:1 › should login successfully (2.5s)
  ✓ login.spec.ts:15:1 › should show error for invalid credentials (1.2s)
  × logout.spec.ts:5:1 › should logout (500ms)
  ⊘ admin.spec.ts:5:1 › should access admin panel

  3 passed
  1 failed
  1 skipped
`

	runner.parseStdoutResults(result, stdout)

	if result.Passed != 3 {
		t.Errorf("Passed = %d, want 3", result.Passed)
	}
	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
	if result.TotalTests != 5 {
		t.Errorf("TotalTests = %d, want 5", result.TotalTests)
	}
}
