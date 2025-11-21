package validator

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/rs/zerolog/log"
)

// Fixer uses LLM to fix failing tests
type Fixer struct {
	router    *llm.Router
	tier      llm.Tier
	maxRetries int
}

// NewFixer creates a test fixer
func NewFixer(router *llm.Router, tier llm.Tier) *Fixer {
	return &Fixer{
		router:    router,
		tier:      tier,
		maxRetries: 3,
	}
}

// FixResult holds the result of a fix attempt
type FixResult struct {
	Fixed       bool   `json:"fixed"`
	NewCode     string `json:"new_code"`
	Explanation string `json:"explanation"`
	Attempts    int    `json:"attempts"`
}

// FixTest attempts to fix a failing test
func (f *Fixer) FixTest(ctx context.Context, testFile string, result *TestResult, validator *Validator) (*FixResult, error) {
	// Read current test code
	code, err := os.ReadFile(testFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	currentCode := string(code)
	fixResult := &FixResult{Attempts: 0}

	for attempt := 1; attempt <= f.maxRetries; attempt++ {
		fixResult.Attempts = attempt

		log.Info().Int("attempt", attempt).Str("file", testFile).Msg("attempting to fix test")

		// Generate fix using LLM
		fixedCode, explanation, err := f.generateFix(ctx, currentCode, result)
		if err != nil {
			log.Warn().Err(err).Int("attempt", attempt).Msg("fix generation failed")
			continue
		}

		// Write fixed code
		if err := os.WriteFile(testFile, []byte(fixedCode), 0644); err != nil {
			return nil, fmt.Errorf("failed to write fixed test: %w", err)
		}

		// Validate the fix
		newResult, err := validator.RunTests(ctx, testFile)
		if err != nil {
			log.Warn().Err(err).Msg("failed to run fixed tests")
			continue
		}

		if newResult.Passed {
			fixResult.Fixed = true
			fixResult.NewCode = fixedCode
			fixResult.Explanation = explanation
			log.Info().Int("attempts", attempt).Msg("test fixed successfully")
			return fixResult, nil
		}

		// Update for next attempt
		currentCode = fixedCode
		result = newResult
	}

	// Restore original if all attempts failed
	os.WriteFile(testFile, code, 0644)
	fixResult.Fixed = false
	fixResult.Explanation = "Failed to fix after maximum attempts"

	return fixResult, nil
}

// generateFix uses LLM to generate fixed test code
func (f *Fixer) generateFix(ctx context.Context, code string, result *TestResult) (string, string, error) {
	prompt := f.buildFixPrompt(code, result)

	req := &llm.Request{
		Tier: f.tier,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 4096,
	}

	response, err := f.router.Complete(ctx, req)
	if err != nil {
		return "", "", err
	}

	// Extract code and explanation from response
	fixedCode, explanation := parseFixResponse(response.Content)
	if fixedCode == "" {
		return "", "", fmt.Errorf("no code found in LLM response")
	}

	return fixedCode, explanation, nil
}

// buildFixPrompt creates the prompt for the LLM
func (f *Fixer) buildFixPrompt(code string, result *TestResult) string {
	var sb strings.Builder

	sb.WriteString("You are a test fixing assistant. A test is failing and needs to be fixed.\n\n")

	sb.WriteString("## Current Test Code\n```\n")
	sb.WriteString(code)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Test Failures\n")
	for _, err := range result.Errors {
		sb.WriteString(fmt.Sprintf("- Test: %s\n", err.TestName))
		if err.Message != "" {
			sb.WriteString(fmt.Sprintf("  Error: %s\n", err.Message))
		}
		if err.Expected != "" {
			sb.WriteString(fmt.Sprintf("  Expected: %s\n", err.Expected))
		}
		if err.Actual != "" {
			sb.WriteString(fmt.Sprintf("  Actual: %s\n", err.Actual))
		}
	}

	sb.WriteString("\n## Raw Output\n```\n")
	output := result.Output
	if len(output) > 1500 {
		output = output[:1500] + "...[truncated]"
	}
	sb.WriteString(output)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Instructions\n")
	sb.WriteString("1. Analyze the test failures\n")
	sb.WriteString("2. Fix the test code to make it pass\n")
	sb.WriteString("3. Keep the test intent the same - only fix issues like:\n")
	sb.WriteString("   - Wrong expected values\n")
	sb.WriteString("   - Incorrect selectors/paths\n")
	sb.WriteString("   - Missing setup/teardown\n")
	sb.WriteString("   - Async/await issues\n")
	sb.WriteString("   - Type mismatches\n")
	sb.WriteString("4. Do NOT remove tests, only fix them\n\n")

	sb.WriteString("## Response Format\n")
	sb.WriteString("EXPLANATION:\n[Brief explanation of what you fixed]\n\n")
	sb.WriteString("CODE:\n```\n[Complete fixed test file]\n```\n")

	return sb.String()
}

// parseFixResponse extracts code and explanation from LLM response
func parseFixResponse(response string) (string, string) {
	var code, explanation string

	// Extract explanation
	if idx := strings.Index(response, "EXPLANATION:"); idx != -1 {
		endIdx := strings.Index(response, "CODE:")
		if endIdx == -1 {
			endIdx = len(response)
		}
		explanation = strings.TrimSpace(response[idx+12 : endIdx])
	}

	// Extract code block
	codeStart := strings.Index(response, "```")
	if codeStart != -1 {
		codeStart = strings.Index(response[codeStart+3:], "\n") + codeStart + 4
		codeEnd := strings.LastIndex(response, "```")
		if codeEnd > codeStart {
			code = strings.TrimSpace(response[codeStart:codeEnd])
		}
	}

	// If no code block, try to find code after CODE:
	if code == "" {
		if idx := strings.Index(response, "CODE:"); idx != -1 {
			code = strings.TrimSpace(response[idx+5:])
			// Remove markdown markers
			code = strings.TrimPrefix(code, "```javascript")
			code = strings.TrimPrefix(code, "```python")
			code = strings.TrimPrefix(code, "```go")
			code = strings.TrimPrefix(code, "```")
			code = strings.TrimSuffix(code, "```")
			code = strings.TrimSpace(code)
		}
	}

	return code, explanation
}
