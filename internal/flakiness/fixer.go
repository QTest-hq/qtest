// Package flakiness provides test flakiness detection and AI-powered fix suggestions.
package flakiness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/QTest-hq/qtest/internal/llm"
)

// FixSuggester uses LLM to suggest fixes for flaky tests.
type FixSuggester struct {
	client llm.Client
	config *FixerConfig
}

// FixerConfig configures the fix suggester.
type FixerConfig struct {
	// Tier specifies which LLM tier to use
	Tier llm.Tier
	// MaxTokens limits the response size
	MaxTokens int
	// IncludeCodeFix enables code fix generation
	IncludeCodeFix bool
}

// DefaultFixerConfig returns default configuration.
func DefaultFixerConfig() *FixerConfig {
	return &FixerConfig{
		Tier:           llm.Tier2,
		MaxTokens:      2000,
		IncludeCodeFix: true,
	}
}

// NewFixSuggester creates a new fix suggester.
func NewFixSuggester(client llm.Client, config *FixerConfig) *FixSuggester {
	if config == nil {
		config = DefaultFixerConfig()
	}
	return &FixSuggester{
		client: client,
		config: config,
	}
}

// FixSuggestion contains a suggested fix for a flaky test.
type FixSuggestion struct {
	// TestID is the test identifier
	TestID string `json:"test_id"`
	// TestName is the test name
	TestName string `json:"test_name"`
	// Category is the type of flakiness detected
	Category FlakyCategory `json:"category"`
	// RootCause is the identified root cause
	RootCause string `json:"root_cause"`
	// Suggestions are the recommended fixes
	Suggestions []string `json:"suggestions"`
	// CodeFix is the suggested code changes (if enabled)
	CodeFix string `json:"code_fix,omitempty"`
	// Confidence is the confidence level (0-1)
	Confidence float64 `json:"confidence"`
	// Priority indicates how urgent the fix is
	Priority FixPriority `json:"priority"`
}

// FlakyCategory represents the type of flakiness.
type FlakyCategory string

const (
	CategoryTiming      FlakyCategory = "timing"
	CategoryConcurrency FlakyCategory = "concurrency"
	CategoryResource    FlakyCategory = "resource"
	CategoryNetwork     FlakyCategory = "network"
	CategoryState       FlakyCategory = "state"
	CategoryRandom      FlakyCategory = "random"
	CategoryEnvironment FlakyCategory = "environment"
	CategoryUnknown     FlakyCategory = "unknown"
)

// FixPriority indicates the urgency of a fix.
type FixPriority string

const (
	PriorityCritical FixPriority = "critical"
	PriorityHigh     FixPriority = "high"
	PriorityMedium   FixPriority = "medium"
	PriorityLow      FixPriority = "low"
)

// AnalysisContext provides context for analyzing flakiness.
type AnalysisContext struct {
	// TestCode is the test source code
	TestCode string
	// TestFilePath is the path to the test file
	TestFilePath string
	// FailureHistory contains recent failure messages
	FailureHistory []string
	// FlakinessScore is the calculated score
	FlakinessScore *FlakinessScore
	// TestHistory is the full test history
	TestHistory *TestHistory
}

// SuggestFix analyzes a flaky test and suggests fixes.
func (f *FixSuggester) SuggestFix(ctx context.Context, analysis *AnalysisContext) (*FixSuggestion, error) {
	if f.client == nil {
		return nil, fmt.Errorf("LLM client not configured")
	}

	// Build the analysis prompt
	prompt := f.buildAnalysisPrompt(analysis)

	req := &llm.Request{
		Tier: f.config.Tier,
		System: `You are an expert at analyzing and fixing flaky tests. You identify root causes and provide specific, actionable fixes.

Respond with valid JSON in this exact format:
{
  "category": "timing|concurrency|resource|network|state|random|environment|unknown",
  "root_cause": "Brief description of the root cause",
  "suggestions": ["First suggestion", "Second suggestion"],
  "code_fix": "Specific code changes if applicable",
  "confidence": 0.85,
  "priority": "critical|high|medium|low"
}`,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   f.config.MaxTokens,
		Temperature: 0.2, // Low temperature for consistent analysis
		JSONMode:    true,
	}

	resp, err := f.client.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	// Parse the response
	suggestion, err := f.parseResponse(resp.Content, analysis)
	if err != nil {
		// Try to provide a basic suggestion even if parsing fails
		return f.fallbackSuggestion(analysis, err)
	}

	return suggestion, nil
}

// SuggestFixBatch analyzes multiple flaky tests.
func (f *FixSuggester) SuggestFixBatch(ctx context.Context, analyses []*AnalysisContext) ([]*FixSuggestion, error) {
	suggestions := make([]*FixSuggestion, 0, len(analyses))

	for _, analysis := range analyses {
		suggestion, err := f.SuggestFix(ctx, analysis)
		if err != nil {
			// Include error as a suggestion with low confidence
			suggestions = append(suggestions, &FixSuggestion{
				TestID:     analysis.FlakinessScore.TestID,
				TestName:   analysis.FlakinessScore.TestName,
				Category:   CategoryUnknown,
				RootCause:  fmt.Sprintf("Analysis failed: %v", err),
				Confidence: 0.0,
				Priority:   PriorityMedium,
			})
			continue
		}
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

// buildAnalysisPrompt creates the analysis prompt.
func (f *FixSuggester) buildAnalysisPrompt(analysis *AnalysisContext) string {
	var sb strings.Builder

	sb.WriteString("Analyze this flaky test and suggest fixes:\n\n")

	// Test info
	sb.WriteString(fmt.Sprintf("Test: %s\n", analysis.FlakinessScore.TestName))
	sb.WriteString(fmt.Sprintf("File: %s\n", analysis.TestFilePath))
	sb.WriteString(fmt.Sprintf("Flakiness Score: %.2f (%s)\n",
		analysis.FlakinessScore.Score,
		analysis.FlakinessScore.Classification))
	sb.WriteString(fmt.Sprintf("Failure Rate: %.1f%%\n", analysis.FlakinessScore.FailureRate*100))
	sb.WriteString(fmt.Sprintf("State Transitions: %.1f%%\n", analysis.FlakinessScore.TransitionRate*100))
	sb.WriteString("\n")

	// Failure patterns
	if len(analysis.FailureHistory) > 0 {
		sb.WriteString("Recent failure messages:\n")
		for i, msg := range analysis.FailureHistory {
			if i >= 5 { // Limit to 5 most recent
				break
			}
			// Truncate long messages
			if len(msg) > 500 {
				msg = msg[:500] + "..."
			}
			sb.WriteString(fmt.Sprintf("- %s\n", msg))
		}
		sb.WriteString("\n")
	}

	// Test code (if available)
	if analysis.TestCode != "" {
		code := analysis.TestCode
		if len(code) > 3000 {
			code = code[:3000] + "\n// ... truncated"
		}
		sb.WriteString("Test code:\n```\n")
		sb.WriteString(code)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("Based on this information, identify:\n")
	sb.WriteString("1. The category of flakiness\n")
	sb.WriteString("2. The root cause\n")
	sb.WriteString("3. Specific fixes to make the test reliable\n")
	if f.config.IncludeCodeFix {
		sb.WriteString("4. Code changes if applicable\n")
	}

	return sb.String()
}

// parseResponse parses the LLM response into a suggestion.
func (f *FixSuggester) parseResponse(content string, analysis *AnalysisContext) (*FixSuggestion, error) {
	// Extract JSON from the response (handle markdown code blocks)
	content = extractJSON(content)

	var parsed struct {
		Category    string   `json:"category"`
		RootCause   string   `json:"root_cause"`
		Suggestions []string `json:"suggestions"`
		CodeFix     string   `json:"code_fix"`
		Confidence  float64  `json:"confidence"`
		Priority    string   `json:"priority"`
	}

	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	suggestion := &FixSuggestion{
		TestID:      analysis.FlakinessScore.TestID,
		TestName:    analysis.FlakinessScore.TestName,
		Category:    parseCategory(parsed.Category),
		RootCause:   parsed.RootCause,
		Suggestions: parsed.Suggestions,
		Confidence:  parsed.Confidence,
		Priority:    parsePriority(parsed.Priority),
	}

	if f.config.IncludeCodeFix {
		suggestion.CodeFix = parsed.CodeFix
	}

	return suggestion, nil
}

// fallbackSuggestion creates a basic suggestion when LLM analysis fails.
func (f *FixSuggester) fallbackSuggestion(analysis *AnalysisContext, err error) (*FixSuggestion, error) {
	// Try to identify category from error patterns
	category := detectCategoryFromPatterns(analysis.FailureHistory)

	suggestion := &FixSuggestion{
		TestID:     analysis.FlakinessScore.TestID,
		TestName:   analysis.FlakinessScore.TestName,
		Category:   category,
		RootCause:  "Unable to determine exact root cause",
		Confidence: 0.3,
		Priority:   PriorityMedium,
	}

	// Add generic suggestions based on category
	suggestion.Suggestions = getSuggestionsForCategory(category)

	return suggestion, nil
}

// extractJSON extracts JSON from a string that might contain markdown.
func extractJSON(content string) string {
	content = strings.TrimSpace(content)

	// If it starts with {, assume it's already JSON
	if strings.HasPrefix(content, "{") {
		return content
	}

	// Try to extract from markdown code block
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find JSON object in the content
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start != -1 && end > start {
		return content[start : end+1]
	}

	return content
}

// parseCategory converts a string to FlakyCategory.
func parseCategory(s string) FlakyCategory {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "timing":
		return CategoryTiming
	case "concurrency":
		return CategoryConcurrency
	case "resource":
		return CategoryResource
	case "network":
		return CategoryNetwork
	case "state":
		return CategoryState
	case "random":
		return CategoryRandom
	case "environment":
		return CategoryEnvironment
	default:
		return CategoryUnknown
	}
}

// parsePriority converts a string to FixPriority.
func parsePriority(s string) FixPriority {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return PriorityCritical
	case "high":
		return PriorityHigh
	case "medium":
		return PriorityMedium
	case "low":
		return PriorityLow
	default:
		return PriorityMedium
	}
}

// detectCategoryFromPatterns tries to identify flakiness category from error messages.
func detectCategoryFromPatterns(failures []string) FlakyCategory {
	combinedErrors := strings.ToLower(strings.Join(failures, " "))

	// Timing patterns
	timingPatterns := []string{"timeout", "deadline", "too slow", "timed out", "sleep", "wait"}
	for _, p := range timingPatterns {
		if strings.Contains(combinedErrors, p) {
			return CategoryTiming
		}
	}

	// Concurrency patterns
	concurrencyPatterns := []string{"race", "deadlock", "concurrent", "goroutine", "mutex", "channel"}
	for _, p := range concurrencyPatterns {
		if strings.Contains(combinedErrors, p) {
			return CategoryConcurrency
		}
	}

	// Resource patterns
	resourcePatterns := []string{"file", "disk", "memory", "port", "permission", "connection refused"}
	for _, p := range resourcePatterns {
		if strings.Contains(combinedErrors, p) {
			return CategoryResource
		}
	}

	// Network patterns
	networkPatterns := []string{"network", "http", "socket", "dns", "dial", "unreachable"}
	for _, p := range networkPatterns {
		if strings.Contains(combinedErrors, p) {
			return CategoryNetwork
		}
	}

	// State patterns
	statePatterns := []string{"state", "order", "previous", "cleanup", "setup", "initialized"}
	for _, p := range statePatterns {
		if strings.Contains(combinedErrors, p) {
			return CategoryState
		}
	}

	// Random patterns
	randomPatterns := []string{"random", "rand", "uuid", "generated"}
	for _, p := range randomPatterns {
		if strings.Contains(combinedErrors, p) {
			return CategoryRandom
		}
	}

	return CategoryUnknown
}

// getSuggestionsForCategory returns generic suggestions for a category.
func getSuggestionsForCategory(category FlakyCategory) []string {
	switch category {
	case CategoryTiming:
		return []string{
			"Increase timeout values or use retry mechanisms",
			"Replace fixed sleeps with polling/conditions",
			"Use Eventually() patterns for async operations",
			"Consider using mock clocks for time-dependent code",
		}
	case CategoryConcurrency:
		return []string{
			"Add proper synchronization (mutexes, channels)",
			"Use sync.WaitGroup for goroutine coordination",
			"Run with -race flag to detect data races",
			"Consider using atomic operations for counters",
		}
	case CategoryResource:
		return []string{
			"Use unique resource names per test (temp files, ports)",
			"Add proper cleanup in test teardown",
			"Check resource availability before tests",
			"Use t.TempDir() for temporary files",
		}
	case CategoryNetwork:
		return []string{
			"Mock external network calls",
			"Add retry logic with exponential backoff",
			"Use test containers for network dependencies",
			"Implement circuit breaker patterns",
		}
	case CategoryState:
		return []string{
			"Ensure proper test isolation",
			"Reset shared state in setup/teardown",
			"Use subtests for better isolation",
			"Avoid global variables in tests",
		}
	case CategoryRandom:
		return []string{
			"Use deterministic values in tests",
			"Seed random generators consistently",
			"Mock random/UUID generation",
			"Use snapshot testing where appropriate",
		}
	case CategoryEnvironment:
		return []string{
			"Mock environment dependencies",
			"Use environment variables in setup",
			"Document required environment setup",
			"Consider containerized test environments",
		}
	default:
		return []string{
			"Increase test isolation",
			"Add explicit waits instead of fixed sleeps",
			"Review shared resources and state",
			"Consider running tests multiple times to identify pattern",
		}
	}
}

// LoadTestCode loads test code from a file path.
func LoadTestCode(testFilePath string) (string, error) {
	data, err := os.ReadFile(testFilePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ExtractTestFunction extracts a specific test function from source code.
func ExtractTestFunction(code, testName string) string {
	// Find the function definition
	pattern := fmt.Sprintf(`(?s)func\s+%s\s*\([^)]*\)\s*\{`, regexp.QuoteMeta(testName))
	re := regexp.MustCompile(pattern)

	loc := re.FindStringIndex(code)
	if loc == nil {
		return code // Return full code if function not found
	}

	start := loc[0]
	// Find matching closing brace
	braceCount := 0
	inString := false
	stringChar := byte(0)
	end := start

	for i := loc[1] - 1; i < len(code); i++ {
		c := code[i]

		// Track string literals to avoid counting braces inside strings
		if (c == '"' || c == '`') && !inString {
			inString = true
			stringChar = c
		} else if c == stringChar && inString && (i == 0 || code[i-1] != '\\') {
			inString = false
		}

		if !inString {
			if c == '{' {
				braceCount++
			} else if c == '}' {
				braceCount--
				if braceCount == 0 {
					end = i + 1
					break
				}
			}
		}
	}

	return code[start:end]
}

// FixReport contains fix suggestions for multiple tests.
type FixReport struct {
	// TotalTests is the number of tests analyzed
	TotalTests int `json:"total_tests"`
	// Suggestions are the fix suggestions
	Suggestions []*FixSuggestion `json:"suggestions"`
	// CategoryBreakdown shows count by category
	CategoryBreakdown map[FlakyCategory]int `json:"category_breakdown"`
	// PriorityBreakdown shows count by priority
	PriorityBreakdown map[FixPriority]int `json:"priority_breakdown"`
}

// GenerateReport creates a report from fix suggestions.
func GenerateReport(suggestions []*FixSuggestion) *FixReport {
	report := &FixReport{
		TotalTests:        len(suggestions),
		Suggestions:       suggestions,
		CategoryBreakdown: make(map[FlakyCategory]int),
		PriorityBreakdown: make(map[FixPriority]int),
	}

	for _, s := range suggestions {
		report.CategoryBreakdown[s.Category]++
		report.PriorityBreakdown[s.Priority]++
	}

	return report
}
