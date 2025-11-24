package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/internal/llm"
)

// LLMEnhancer uses LLM to enhance E2E test specifications.
type LLMEnhancer struct {
	router *llm.Router
	config *EnhancerConfig
}

// EnhancerConfig configures the LLM enhancer.
type EnhancerConfig struct {
	// MaxSuggestionsPerTest limits suggestions per test case
	MaxSuggestionsPerTest int `json:"maxSuggestionsPerTest"`
	// GenerateNegativeTests enables negative test case generation
	GenerateNegativeTests bool `json:"generateNegativeTests"`
	// GenerateEdgeCases enables edge case test generation
	GenerateEdgeCases bool `json:"generateEdgeCases"`
	// ImproveDescriptions enables description enhancement
	ImproveDescriptions bool `json:"improveDescriptions"`
	// AddAccessibilityTests enables accessibility test suggestions
	AddAccessibilityTests bool `json:"addAccessibilityTests"`
}

// DefaultEnhancerConfig returns the default enhancer configuration.
func DefaultEnhancerConfig() *EnhancerConfig {
	return &EnhancerConfig{
		MaxSuggestionsPerTest: 5,
		GenerateNegativeTests: true,
		GenerateEdgeCases:     true,
		ImproveDescriptions:   true,
		AddAccessibilityTests: false,
	}
}

// NewLLMEnhancer creates a new LLM enhancer.
func NewLLMEnhancer(router *llm.Router, config *EnhancerConfig) *LLMEnhancer {
	if config == nil {
		config = DefaultEnhancerConfig()
	}
	return &LLMEnhancer{
		router: router,
		config: config,
	}
}

// EnhancementResult contains the result of LLM enhancement.
type EnhancementResult struct {
	// EnhancedSpec is the enhanced test specification
	EnhancedSpec *E2ETestSpec `json:"enhancedSpec"`
	// SuggestedTests are additional test cases suggested by LLM
	SuggestedTests []TestCase `json:"suggestedTests,omitempty"`
	// Improvements lists improvements made to existing tests
	Improvements []Improvement `json:"improvements,omitempty"`
	// Warnings contains any warnings during enhancement
	Warnings []string `json:"warnings,omitempty"`
}

// Improvement describes an improvement made to a test.
type Improvement struct {
	TestID      string `json:"testId"`
	Type        string `json:"type"` // assertion, description, data, selector
	Description string `json:"description"`
	Before      string `json:"before,omitempty"`
	After       string `json:"after,omitempty"`
}

// Enhance enhances an E2E test specification using LLM.
func (e *LLMEnhancer) Enhance(ctx context.Context, spec *E2ETestSpec) (*EnhancementResult, error) {
	result := &EnhancementResult{
		EnhancedSpec:   spec,
		SuggestedTests: make([]TestCase, 0),
		Improvements:   make([]Improvement, 0),
	}

	// Enhance descriptions
	if e.config.ImproveDescriptions {
		improvements, err := e.improveDescriptions(ctx, spec)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to improve descriptions: %v", err))
		} else {
			result.Improvements = append(result.Improvements, improvements...)
		}
	}

	// Suggest additional assertions
	assertionImprovements, err := e.suggestAssertions(ctx, spec)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to suggest assertions: %v", err))
	} else {
		result.Improvements = append(result.Improvements, assertionImprovements...)
	}

	// Generate negative tests
	if e.config.GenerateNegativeTests {
		negativeTests, err := e.generateNegativeTests(ctx, spec)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to generate negative tests: %v", err))
		} else {
			result.SuggestedTests = append(result.SuggestedTests, negativeTests...)
		}
	}

	// Generate edge case tests
	if e.config.GenerateEdgeCases {
		edgeCases, err := e.generateEdgeCaseTests(ctx, spec)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to generate edge cases: %v", err))
		} else {
			result.SuggestedTests = append(result.SuggestedTests, edgeCases...)
		}
	}

	return result, nil
}

// SuggestTestCases suggests additional test cases based on existing ones.
func (e *LLMEnhancer) SuggestTestCases(ctx context.Context, spec *E2ETestSpec) ([]TestCase, error) {
	prompt := e.buildSuggestionPrompt(spec)

	response, err := e.router.Complete(ctx, &llm.Request{
		System:      testEnhancementSystemPrompt,
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.3,
		MaxTokens:   2000,
		Tier:        llm.Tier2,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM suggestion failed: %w", err)
	}

	return e.parseTestCaseSuggestions(response.Content)
}

func (e *LLMEnhancer) improveDescriptions(ctx context.Context, spec *E2ETestSpec) ([]Improvement, error) {
	var improvements []Improvement

	for i := range spec.TestCases {
		tc := &spec.TestCases[i]
		if tc.Description == "" || len(tc.Description) < 20 {
			improved, err := e.improveTestDescription(ctx, tc)
			if err != nil {
				continue
			}
			if improved != tc.Description {
				improvements = append(improvements, Improvement{
					TestID:      tc.ID,
					Type:        "description",
					Description: "Improved test description for clarity",
					Before:      tc.Description,
					After:       improved,
				})
				tc.Description = improved
			}
		}
	}

	return improvements, nil
}

func (e *LLMEnhancer) improveTestDescription(ctx context.Context, tc *TestCase) (string, error) {
	prompt := fmt.Sprintf(`Improve this test description to be more clear and descriptive.

Test Name: %s
Current Description: %s
Steps: %d steps including actions like %s

Provide only the improved description, nothing else.`,
		tc.Name,
		tc.Description,
		len(tc.Steps),
		e.summarizeActions(tc.Steps))

	response, err := e.router.Complete(ctx, &llm.Request{
		System:      testEnhancementSystemPrompt,
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.2,
		MaxTokens:   200,
		Tier:        llm.Tier1,
	})
	if err != nil {
		return tc.Description, err
	}

	return strings.TrimSpace(response.Content), nil
}

func (e *LLMEnhancer) suggestAssertions(ctx context.Context, spec *E2ETestSpec) ([]Improvement, error) {
	var improvements []Improvement

	for i := range spec.TestCases {
		tc := &spec.TestCases[i]

		// Only suggest for tests with few assertions
		totalAssertions := len(tc.Expected)
		for _, step := range tc.Steps {
			totalAssertions += len(step.Assertions)
		}

		if totalAssertions < 3 {
			suggestions, err := e.suggestTestAssertions(ctx, tc)
			if err != nil {
				continue
			}

			for _, assertion := range suggestions {
				if len(tc.Expected) < e.config.MaxSuggestionsPerTest {
					tc.Expected = append(tc.Expected, assertion)
					improvements = append(improvements, Improvement{
						TestID:      tc.ID,
						Type:        "assertion",
						Description: fmt.Sprintf("Added %s assertion", assertion.Type),
					})
				}
			}
		}
	}

	return improvements, nil
}

func (e *LLMEnhancer) suggestTestAssertions(ctx context.Context, tc *TestCase) ([]Assertion, error) {
	prompt := fmt.Sprintf(`Suggest additional assertions for this E2E test.

Test Name: %s
Description: %s
Current Assertions: %d

Suggest 1-3 additional assertions that would make this test more robust.
Return JSON array of assertions:
[{"type": "visible|text|value|url|title", "selector": "CSS selector", "expected": "expected value"}]`,
		tc.Name,
		tc.Description,
		len(tc.Expected))

	response, err := e.router.Complete(ctx, &llm.Request{
		System:      testEnhancementSystemPrompt,
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.3,
		MaxTokens:   500,
		Tier:        llm.Tier1,
	})
	if err != nil {
		return nil, err
	}

	return e.parseAssertionSuggestions(response.Content)
}

func (e *LLMEnhancer) generateNegativeTests(ctx context.Context, spec *E2ETestSpec) ([]TestCase, error) {
	if len(spec.TestCases) == 0 {
		return nil, nil
	}

	prompt := e.buildNegativeTestPrompt(spec)

	response, err := e.router.Complete(ctx, &llm.Request{
		System:      testEnhancementSystemPrompt,
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.4,
		MaxTokens:   1500,
		Tier:        llm.Tier2,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate negative tests: %w", err)
	}

	return e.parseTestCaseSuggestions(response.Content)
}

func (e *LLMEnhancer) generateEdgeCaseTests(ctx context.Context, spec *E2ETestSpec) ([]TestCase, error) {
	if len(spec.TestCases) == 0 {
		return nil, nil
	}

	prompt := e.buildEdgeCasePrompt(spec)

	response, err := e.router.Complete(ctx, &llm.Request{
		System:      testEnhancementSystemPrompt,
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.4,
		MaxTokens:   1500,
		Tier:        llm.Tier2,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate edge case tests: %w", err)
	}

	return e.parseTestCaseSuggestions(response.Content)
}

func (e *LLMEnhancer) buildSuggestionPrompt(spec *E2ETestSpec) string {
	var sb strings.Builder

	sb.WriteString("Based on these E2E tests, suggest additional test cases:\n\n")
	sb.WriteString(fmt.Sprintf("Test Suite: %s\n", spec.Name))
	sb.WriteString(fmt.Sprintf("Base URL: %s\n\n", spec.BaseURL))

	sb.WriteString("Existing Tests:\n")
	for _, tc := range spec.TestCases {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", tc.Name, tc.Description))
	}

	sb.WriteString("\nSuggest 2-3 additional test cases that would improve coverage.\n")
	sb.WriteString("Return JSON array of test cases with name, description, and tags.\n")

	return sb.String()
}

func (e *LLMEnhancer) buildNegativeTestPrompt(spec *E2ETestSpec) string {
	var sb strings.Builder

	sb.WriteString("Generate negative test cases for these E2E tests:\n\n")
	sb.WriteString(fmt.Sprintf("Test Suite: %s\n", spec.Name))

	sb.WriteString("\nExisting Tests:\n")
	for _, tc := range spec.TestCases {
		sb.WriteString(fmt.Sprintf("- %s\n", tc.Name))
		for _, step := range tc.Steps {
			sb.WriteString(fmt.Sprintf("  Step: %s (action: %s)\n", step.Name, step.Action.Type))
		}
	}

	sb.WriteString("\nGenerate 1-2 negative test cases that test error handling and validation.\n")
	sb.WriteString("Return JSON array: [{\"name\": \"...\", \"description\": \"...\", \"tags\": [\"negative\"]}]\n")

	return sb.String()
}

func (e *LLMEnhancer) buildEdgeCasePrompt(spec *E2ETestSpec) string {
	var sb strings.Builder

	sb.WriteString("Generate edge case tests for these E2E tests:\n\n")
	sb.WriteString(fmt.Sprintf("Test Suite: %s\n", spec.Name))

	sb.WriteString("\nExisting Tests:\n")
	for _, tc := range spec.TestCases {
		sb.WriteString(fmt.Sprintf("- %s\n", tc.Name))
	}

	sb.WriteString("\nGenerate 1-2 edge case tests for boundary conditions, empty states, etc.\n")
	sb.WriteString("Return JSON array: [{\"name\": \"...\", \"description\": \"...\", \"tags\": [\"edge-case\"]}]\n")

	return sb.String()
}

func (e *LLMEnhancer) summarizeActions(steps []TestStep) string {
	actions := make([]string, 0, len(steps))
	for _, step := range steps {
		actions = append(actions, string(step.Action.Type))
	}
	if len(actions) > 3 {
		return strings.Join(actions[:3], ", ") + "..."
	}
	return strings.Join(actions, ", ")
}

func (e *LLMEnhancer) parseTestCaseSuggestions(response string) ([]TestCase, error) {
	jsonStr := extractJSONArray(response)
	if jsonStr == "" {
		return nil, nil
	}

	var raw []struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, err
	}

	var testCases []TestCase
	for _, r := range raw {
		tc := TestCase{
			ID:          generateID(),
			Name:        r.Name,
			Description: r.Description,
			Tags:        r.Tags,
			Steps:       []TestStep{},
		}
		testCases = append(testCases, tc)
	}

	return testCases, nil
}

func (e *LLMEnhancer) parseAssertionSuggestions(response string) ([]Assertion, error) {
	jsonStr := extractJSONArray(response)
	if jsonStr == "" {
		return nil, nil
	}

	var raw []struct {
		Type     string      `json:"type"`
		Selector string      `json:"selector"`
		Expected interface{} `json:"expected"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, err
	}

	var assertions []Assertion
	for _, r := range raw {
		assertion := Assertion{
			Type:     AssertionType(r.Type),
			Selector: r.Selector,
			Expected: r.Expected,
		}
		assertions = append(assertions, assertion)
	}

	return assertions, nil
}

func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	if start == -1 {
		return ""
	}

	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '[' {
			depth++
		} else if s[i] == ']' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
}

func generateID() string {
	return fmt.Sprintf("test-%d", randomInt())
}

func randomInt() int {
	// Simple pseudo-random for IDs
	return int(^uint(0)>>1) & 0xFFFFFF
}

const testEnhancementSystemPrompt = `You are an E2E test enhancement assistant. Your task is to improve test quality by:

1. Suggesting additional test cases for better coverage
2. Adding meaningful assertions to existing tests
3. Generating negative test cases for error handling
4. Creating edge case tests for boundary conditions
5. Improving test descriptions for clarity

Always return valid JSON when requested. Focus on practical, executable test improvements.
Consider:
- User experience scenarios
- Error states and validation
- Accessibility concerns
- Performance implications
- Security testing opportunities`
