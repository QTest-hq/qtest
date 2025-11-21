package llm

import (
	"strings"
	"testing"
)

func TestTestGenerationPrompt(t *testing.T) {
	code := `func Add(a, b int) int {
	return a + b
}`
	funcName := "Add"
	fileName := "math.go"
	language := "go"
	context := "This is a simple math utility"

	prompt := TestGenerationPrompt(code, funcName, fileName, language, context)

	// Should contain all inputs
	if !strings.Contains(prompt, funcName) {
		t.Error("prompt should contain function name")
	}
	if !strings.Contains(prompt, fileName) {
		t.Error("prompt should contain file name")
	}
	if !strings.Contains(prompt, "```go") {
		t.Error("prompt should contain go code block")
	}
	if !strings.Contains(prompt, code) {
		t.Error("prompt should contain function code")
	}
	if !strings.Contains(prompt, context) {
		t.Error("prompt should contain context")
	}
	if !strings.Contains(prompt, "YAML") {
		t.Error("prompt should mention YAML output format")
	}
}

func TestTestGenerationPrompt_DifferentLanguages(t *testing.T) {
	tests := []struct {
		language string
		expected string
	}{
		{"go", "```go"},
		{"python", "```python"},
		{"javascript", "```javascript"},
		{"typescript", "```typescript"},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			prompt := TestGenerationPrompt("code", "func", "file", tt.language, "")
			if !strings.Contains(prompt, tt.expected) {
				t.Errorf("prompt should contain %s code block", tt.expected)
			}
		})
	}
}

func TestIntegrationTestPrompt(t *testing.T) {
	endpoints := []string{"/api/users", "/api/posts"}
	dependencies := []string{"database", "redis"}
	context := "User management API"

	prompt := IntegrationTestPrompt(endpoints, dependencies, context)

	// Should contain all inputs
	if !strings.Contains(prompt, "/api/users") {
		t.Error("prompt should contain endpoints")
	}
	if !strings.Contains(prompt, "/api/posts") {
		t.Error("prompt should contain all endpoints")
	}
	if !strings.Contains(prompt, "database") {
		t.Error("prompt should contain dependencies")
	}
	if !strings.Contains(prompt, context) {
		t.Error("prompt should contain context")
	}
	if !strings.Contains(prompt, "YAML") {
		t.Error("prompt should mention YAML output format")
	}
}

func TestIntegrationTestPrompt_EmptyInputs(t *testing.T) {
	// Should not panic with empty inputs
	prompt := IntegrationTestPrompt([]string{}, []string{}, "")
	if prompt == "" {
		t.Error("prompt should not be empty")
	}
}

func TestParseDSLOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain yaml",
			input:    "name: test\nsteps:\n  - action: call",
			expected: "name: test\nsteps:\n  - action: call",
		},
		{
			name:     "with yaml code block",
			input:    "```yaml\nname: test\n```",
			expected: "name: test",
		},
		{
			name:     "with generic code block",
			input:    "```\nname: test\n```",
			expected: "name: test",
		},
		{
			name:     "with whitespace",
			input:    "  \n```yaml\nname: test\n```  \n",
			expected: "name: test",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \n\t  ",
			expected: "",
		},
		{
			name:     "code block without closing",
			input:    "```yaml\nname: test",
			expected: "name: test",
		},
		{
			name:     "multiple code blocks - first one",
			input:    "```yaml\nfirst\n```\nsome text\n```yaml\nsecond\n```",
			expected: "first\n```\nsome text\n```yaml\nsecond",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDSLOutput(tt.input)
			if result != tt.expected {
				t.Errorf("ParseDSLOutput(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSystemPromptConstants(t *testing.T) {
	// Verify system prompts are not empty
	if SystemPromptTestGeneration == "" {
		t.Error("SystemPromptTestGeneration should not be empty")
	}
	if SystemPromptCritic == "" {
		t.Error("SystemPromptCritic should not be empty")
	}

	// Verify they contain expected content
	if !strings.Contains(SystemPromptTestGeneration, "test") {
		t.Error("SystemPromptTestGeneration should mention 'test'")
	}
	if !strings.Contains(SystemPromptCritic, "quality") {
		t.Error("SystemPromptCritic should mention 'quality'")
	}

	// Verify JSON format instruction in critic prompt
	if !strings.Contains(SystemPromptCritic, "JSON") {
		t.Error("SystemPromptCritic should mention JSON output format")
	}
}
