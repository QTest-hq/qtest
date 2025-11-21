package llm

import (
	"fmt"
	"strings"
)

// Prompt templates for test generation

const SystemPromptTestGeneration = `You are an expert software engineer specializing in test-driven development.
Your task is to generate high-quality, meaningful tests that:
1. Test actual behavior, not implementation details
2. Cover edge cases and error conditions
3. Are maintainable and readable
4. Follow the conventions of the target testing framework

IMPORTANT:
- Use realistic, reasonable test values (avoid extreme values that cause overflow)
- Generate 4-6 test cases maximum - quality over quantity
- For numeric types, use values within safe ranges (e.g., -1000000 to 1000000)
- Each test should have a clear expected result assertion

Output tests in the specified DSL format (YAML). Be precise and thorough.`

const SystemPromptCritic = `You are a test quality expert. Analyze the given test and determine if it:
1. Tests meaningful behavior (not trivial)
2. Has proper assertions
3. Handles edge cases appropriately
4. Would catch real bugs via mutation testing

Respond with JSON: {"quality": "high"|"medium"|"low", "issues": [...], "suggestions": [...]}`

// TestGenerationPrompt creates a prompt for generating a unit test
func TestGenerationPrompt(functionCode, functionName, fileName, language string, context string) string {
	codeBlock := "```" + language + "\n" + functionCode + "\n```"
	return fmt.Sprintf("Generate a unit test for the following %s function:\n\n"+
		"File: %s\n"+
		"Function: %s\n\n"+
		"%s\n\n"+
		"%s\n\n"+
		"Generate a test in DSL format (YAML) with:\n"+
		"- Meaningful test name describing the behavior\n"+
		"- Setup if needed\n"+
		"- Clear action (function call)\n"+
		"- Assertions on expected output\n"+
		"- Edge case handling\n\n"+
		"Output ONLY the YAML, no explanation.", language, fileName, functionName, codeBlock, context)
}

// IntegrationTestPrompt creates a prompt for generating an integration test
func IntegrationTestPrompt(endpoints []string, dependencies []string, context string) string {
	return fmt.Sprintf(`Generate an integration test for the following API endpoints:

Endpoints:
%s

Dependencies:
%s

Context:
%s

Generate a test in DSL format (YAML) that:
- Tests the endpoint behavior
- Sets up necessary dependencies
- Verifies response format and content
- Handles error cases

Output ONLY the YAML, no explanation.`,
		strings.Join(endpoints, "\n"),
		strings.Join(dependencies, ", "),
		context)
}

// ParseDSLOutput extracts YAML content from LLM response
func ParseDSLOutput(response string) string {
	// Remove markdown code blocks if present
	response = strings.TrimSpace(response)

	// Remove ```yaml and ``` markers
	if strings.HasPrefix(response, "```yaml") {
		response = strings.TrimPrefix(response, "```yaml")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}

	if strings.HasSuffix(response, "```") {
		response = strings.TrimSuffix(response, "```")
	}

	return strings.TrimSpace(response)
}
