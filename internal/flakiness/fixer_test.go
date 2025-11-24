package flakiness

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/QTest-hq/qtest/internal/llm"
)

func TestDefaultFixerConfig(t *testing.T) {
	cfg := DefaultFixerConfig()
	if cfg.Tier != llm.Tier2 {
		t.Errorf("expected Tier2, got %v", cfg.Tier)
	}
	if cfg.MaxTokens != 2000 {
		t.Errorf("expected MaxTokens 2000, got %d", cfg.MaxTokens)
	}
	if !cfg.IncludeCodeFix {
		t.Error("expected IncludeCodeFix to be true")
	}
}

func TestNewFixSuggester(t *testing.T) {
	suggester := NewFixSuggester(nil, nil)
	if suggester == nil {
		t.Fatal("expected non-nil suggester")
	}
	if suggester.config.Tier != llm.Tier2 {
		t.Error("expected default config to be applied")
	}
}

func TestParseCategory(t *testing.T) {
	tests := []struct {
		input    string
		expected FlakyCategory
	}{
		{"timing", CategoryTiming},
		{"TIMING", CategoryTiming},
		{"concurrency", CategoryConcurrency},
		{"resource", CategoryResource},
		{"network", CategoryNetwork},
		{"state", CategoryState},
		{"random", CategoryRandom},
		{"environment", CategoryEnvironment},
		{"unknown", CategoryUnknown},
		{"something_else", CategoryUnknown},
		{"", CategoryUnknown},
	}

	for _, tc := range tests {
		result := parseCategory(tc.input)
		if result != tc.expected {
			t.Errorf("parseCategory(%q) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
}

func TestParsePriority(t *testing.T) {
	tests := []struct {
		input    string
		expected FixPriority
	}{
		{"critical", PriorityCritical},
		{"CRITICAL", PriorityCritical},
		{"high", PriorityHigh},
		{"medium", PriorityMedium},
		{"low", PriorityLow},
		{"unknown", PriorityMedium},
		{"", PriorityMedium},
	}

	for _, tc := range tests {
		result := parsePriority(tc.input)
		if result != tc.expected {
			t.Errorf("parsePriority(%q) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain json",
			input:    `{"category": "timing"}`,
			expected: `{"category": "timing"}`,
		},
		{
			name:     "markdown code block",
			input:    "```json\n{\"category\": \"timing\"}\n```",
			expected: `{"category": "timing"}`,
		},
		{
			name:     "markdown code block without json tag",
			input:    "```\n{\"category\": \"timing\"}\n```",
			expected: `{"category": "timing"}`,
		},
		{
			name:     "json embedded in text",
			input:    "Here is the result: {\"category\": \"timing\"} and more text",
			expected: `{"category": "timing"}`,
		},
		{
			name:     "with whitespace",
			input:    "  {\"category\": \"timing\"}  ",
			expected: `{"category": "timing"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractJSON(tc.input)
			if result != tc.expected {
				t.Errorf("extractJSON() = %q, expected %q", result, tc.expected)
			}
		})
	}
}

func TestDetectCategoryFromPatterns(t *testing.T) {
	tests := []struct {
		failures []string
		expected FlakyCategory
	}{
		{[]string{"test timed out after 30s"}, CategoryTiming},
		{[]string{"deadline exceeded"}, CategoryTiming},
		{[]string{"race condition detected"}, CategoryConcurrency},
		{[]string{"deadlock detected"}, CategoryConcurrency},
		{[]string{"goroutine leak"}, CategoryConcurrency},
		{[]string{"file not found"}, CategoryResource},
		{[]string{"connection refused"}, CategoryResource},
		{[]string{"network unreachable"}, CategoryNetwork},
		{[]string{"network unreachable: dial tcp"}, CategoryNetwork},
		{[]string{"state not initialized"}, CategoryState},
		{[]string{"cleanup failed"}, CategoryState},
		{[]string{"random value mismatch"}, CategoryRandom},
		{[]string{"no recognizable pattern"}, CategoryUnknown},
	}

	for _, tc := range tests {
		result := detectCategoryFromPatterns(tc.failures)
		if result != tc.expected {
			t.Errorf("detectCategoryFromPatterns(%v) = %v, expected %v",
				tc.failures, result, tc.expected)
		}
	}
}

func TestGetSuggestionsForCategory(t *testing.T) {
	categories := []FlakyCategory{
		CategoryTiming,
		CategoryConcurrency,
		CategoryResource,
		CategoryNetwork,
		CategoryState,
		CategoryRandom,
		CategoryEnvironment,
		CategoryUnknown,
	}

	for _, cat := range categories {
		suggestions := getSuggestionsForCategory(cat)
		if len(suggestions) == 0 {
			t.Errorf("getSuggestionsForCategory(%v) returned empty suggestions", cat)
		}
	}
}

func TestExtractTestFunction(t *testing.T) {
	code := `package main

func TestAdd(t *testing.T) {
	result := Add(1, 2)
	if result != 3 {
		t.Error("expected 3")
	}
}

func TestSubtract(t *testing.T) {
	result := Subtract(5, 3)
	if result != 2 {
		t.Error("expected 2")
	}
}
`
	extracted := ExtractTestFunction(code, "TestAdd")
	if !contains(extracted, "func TestAdd") {
		t.Error("expected extracted code to contain TestAdd")
	}
	if contains(extracted, "TestSubtract") {
		t.Error("extracted code should not contain TestSubtract")
	}
}

func TestExtractTestFunctionNotFound(t *testing.T) {
	code := `package main

func TestAdd(t *testing.T) {
	// test code
}
`
	// Should return full code when function not found
	extracted := ExtractTestFunction(code, "TestNotExist")
	if extracted != code {
		t.Error("expected full code when function not found")
	}
}

func TestGenerateReport(t *testing.T) {
	suggestions := []*FixSuggestion{
		{TestID: "1", Category: CategoryTiming, Priority: PriorityHigh},
		{TestID: "2", Category: CategoryTiming, Priority: PriorityCritical},
		{TestID: "3", Category: CategoryConcurrency, Priority: PriorityMedium},
	}

	report := GenerateReport(suggestions)

	if report.TotalTests != 3 {
		t.Errorf("expected 3 total tests, got %d", report.TotalTests)
	}
	if report.CategoryBreakdown[CategoryTiming] != 2 {
		t.Errorf("expected 2 timing issues, got %d", report.CategoryBreakdown[CategoryTiming])
	}
	if report.CategoryBreakdown[CategoryConcurrency] != 1 {
		t.Errorf("expected 1 concurrency issue, got %d", report.CategoryBreakdown[CategoryConcurrency])
	}
	if report.PriorityBreakdown[PriorityHigh] != 1 {
		t.Errorf("expected 1 high priority, got %d", report.PriorityBreakdown[PriorityHigh])
	}
}

// MockLLMClient for testing
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.Response{Content: m.response}, nil
}

func (m *mockLLMClient) Name() llm.Provider {
	return "mock"
}

func (m *mockLLMClient) Available() bool {
	return true
}

func TestSuggestFix(t *testing.T) {
	mockResp := map[string]interface{}{
		"category":    "timing",
		"root_cause":  "Test uses fixed sleep",
		"suggestions": []string{"Use polling instead of sleep"},
		"code_fix":    "// Use time.Eventually",
		"confidence":  0.85,
		"priority":    "high",
	}
	respJSON, _ := json.Marshal(mockResp)

	mock := &mockLLMClient{response: string(respJSON)}
	suggester := NewFixSuggester(mock, nil)

	analysis := &AnalysisContext{
		TestCode:     "func TestSlow(t *testing.T) { time.Sleep(5*time.Second) }",
		TestFilePath: "/path/to/test.go",
		FlakinessScore: &FlakinessScore{
			TestID:         "test-1",
			TestName:       "TestSlow",
			Score:          0.35,
			Classification: "highly_flaky",
			FailureRate:    0.4,
			TransitionRate: 0.3,
		},
		FailureHistory: []string{"test timed out"},
	}

	suggestion, err := suggester.SuggestFix(context.Background(), analysis)
	if err != nil {
		t.Fatalf("SuggestFix failed: %v", err)
	}

	if suggestion.Category != CategoryTiming {
		t.Errorf("expected CategoryTiming, got %v", suggestion.Category)
	}
	if suggestion.Priority != PriorityHigh {
		t.Errorf("expected PriorityHigh, got %v", suggestion.Priority)
	}
	if suggestion.RootCause != "Test uses fixed sleep" {
		t.Errorf("unexpected root cause: %s", suggestion.RootCause)
	}
	if len(suggestion.Suggestions) == 0 {
		t.Error("expected suggestions")
	}
}

func TestSuggestFixNoClient(t *testing.T) {
	suggester := NewFixSuggester(nil, nil)

	analysis := &AnalysisContext{
		FlakinessScore: &FlakinessScore{TestID: "test-1"},
	}

	_, err := suggester.SuggestFix(context.Background(), analysis)
	if err == nil {
		t.Error("expected error when no client configured")
	}
}

func TestSuggestFixBatch(t *testing.T) {
	mockResp := map[string]interface{}{
		"category":    "timing",
		"root_cause":  "Test uses fixed sleep",
		"suggestions": []string{"Fix it"},
		"confidence":  0.8,
		"priority":    "medium",
	}
	respJSON, _ := json.Marshal(mockResp)

	mock := &mockLLMClient{response: string(respJSON)}
	suggester := NewFixSuggester(mock, nil)

	analyses := []*AnalysisContext{
		{
			FlakinessScore: &FlakinessScore{TestID: "test-1", TestName: "Test1"},
			FailureHistory: []string{"timeout"},
		},
		{
			FlakinessScore: &FlakinessScore{TestID: "test-2", TestName: "Test2"},
			FailureHistory: []string{"race"},
		},
	}

	suggestions, err := suggester.SuggestFixBatch(context.Background(), analyses)
	if err != nil {
		t.Fatalf("SuggestFixBatch failed: %v", err)
	}

	if len(suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d", len(suggestions))
	}
}

func TestFallbackSuggestion(t *testing.T) {
	suggester := NewFixSuggester(nil, nil)

	analysis := &AnalysisContext{
		FlakinessScore: &FlakinessScore{
			TestID:   "test-1",
			TestName: "TestFlaky",
		},
		FailureHistory: []string{"test timed out after 30s"},
	}

	suggestion, err := suggester.fallbackSuggestion(analysis, nil)
	if err != nil {
		t.Fatalf("fallbackSuggestion failed: %v", err)
	}

	// Should detect timing category from error message
	if suggestion.Category != CategoryTiming {
		t.Errorf("expected CategoryTiming from fallback, got %v", suggestion.Category)
	}
	if len(suggestion.Suggestions) == 0 {
		t.Error("expected fallback suggestions")
	}
	if suggestion.Confidence > 0.5 {
		t.Error("expected low confidence for fallback")
	}
}

func TestBuildAnalysisPrompt(t *testing.T) {
	suggester := NewFixSuggester(nil, &FixerConfig{IncludeCodeFix: true})

	analysis := &AnalysisContext{
		TestCode:     "func TestExample(t *testing.T) { }",
		TestFilePath: "/path/to/test.go",
		FlakinessScore: &FlakinessScore{
			TestID:         "test-1",
			TestName:       "TestExample",
			Score:          0.25,
			Classification: "flaky",
			FailureRate:    0.2,
			TransitionRate: 0.3,
		},
		FailureHistory: []string{"assertion failed", "timeout"},
	}

	prompt := suggester.buildAnalysisPrompt(analysis)

	// Check that key information is included
	if !contains(prompt, "TestExample") {
		t.Error("prompt should contain test name")
	}
	if !contains(prompt, "/path/to/test.go") {
		t.Error("prompt should contain file path")
	}
	if !contains(prompt, "flaky") {
		t.Error("prompt should contain classification")
	}
	if !contains(prompt, "assertion failed") {
		t.Error("prompt should contain failure messages")
	}
	if !contains(prompt, "func TestExample") {
		t.Error("prompt should contain test code")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
