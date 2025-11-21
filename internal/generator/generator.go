package generator

import (
	"context"
	"fmt"
	"os"

	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/QTest-hq/qtest/pkg/dsl"
	"github.com/rs/zerolog/log"
)

// Generator generates tests from parsed code
type Generator struct {
	llmRouter *llm.Router
	parser    *parser.Parser
}

// NewGenerator creates a new test generator
func NewGenerator(router *llm.Router) *Generator {
	return &Generator{
		llmRouter: router,
		parser:    parser.NewParser(),
	}
}

// GenerateOptions holds options for test generation
type GenerateOptions struct {
	Tier       llm.Tier
	TestType   dsl.TestType
	Framework  string
	MaxTests   int
	TargetFile string // Optional: specific file to target
}

// GeneratedTest represents a generated test with metadata
type GeneratedTest struct {
	DSL       *dsl.TestDSL
	RawYAML   string
	Function  *parser.Function
	FileName  string
}

// GenerateForFile generates tests for all functions in a file
func (g *Generator) GenerateForFile(ctx context.Context, filePath string, opts GenerateOptions) ([]GeneratedTest, error) {
	// Parse the file
	parsed, err := g.parser.ParseFile(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	log.Info().
		Str("file", filePath).
		Int("functions", len(parsed.Functions)).
		Str("language", string(parsed.Language)).
		Msg("parsed file")

	// Generate tests for each function
	tests := make([]GeneratedTest, 0)
	for i, fn := range parsed.Functions {
		if opts.MaxTests > 0 && len(tests) >= opts.MaxTests {
			break
		}

		// Skip private functions for unit tests
		if !fn.Exported && opts.TestType == dsl.TestTypeUnit {
			log.Debug().Str("function", fn.Name).Msg("skipping private function")
			continue
		}

		log.Info().
			Str("function", fn.Name).
			Int("index", i+1).
			Int("total", len(parsed.Functions)).
			Msg("generating test")

		test, err := g.generateTestForFunction(ctx, &fn, parsed, opts)
		if err != nil {
			log.Warn().Err(err).Str("function", fn.Name).Msg("failed to generate test")
			continue
		}

		tests = append(tests, *test)
	}

	return tests, nil
}

// generateTestForFunction generates a single test for a function
func (g *Generator) generateTestForFunction(ctx context.Context, fn *parser.Function, file *parser.ParsedFile, opts GenerateOptions) (*GeneratedTest, error) {
	// Read the file content to get the function body
	content, err := os.ReadFile(file.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Extract function body from content
	lines := splitLines(string(content))
	functionCode := extractLines(lines, fn.StartLine, fn.EndLine)

	// Build context from related functions
	context := g.buildContext(file, fn)

	// Create the prompt
	prompt := llm.TestGenerationPrompt(
		functionCode,
		fn.Name,
		file.Path,
		string(file.Language),
		context,
	)

	// Call LLM
	resp, err := g.llmRouter.Complete(ctx, &llm.Request{
		Tier:   opts.Tier,
		System: llm.SystemPromptTestGeneration,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3, // Lower temperature for more deterministic output
		MaxTokens:   2000,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse the response
	yamlContent := llm.ParseDSLOutput(resp.Content)

	// Convert LLM output to DSL (handles multiple formats)
	testDSL, err := ConvertToDSL(yamlContent, fn.Name, file.Path, string(file.Language))
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSL: %w (content: %s)", err, yamlContent[:min(200, len(yamlContent))])
	}

	// Fill in defaults
	if testDSL.Version == "" {
		testDSL.Version = "1.0"
	}
	if testDSL.Type == "" {
		testDSL.Type = opts.TestType
	}
	testDSL.Target = dsl.TestTarget{
		File:     file.Path,
		Function: fn.Name,
	}

	return &GeneratedTest{
		DSL:      testDSL,
		RawYAML:  yamlContent,
		Function: fn,
		FileName: file.Path,
	}, nil
}

// buildContext builds context from related functions
func (g *Generator) buildContext(file *parser.ParsedFile, targetFn *parser.Function) string {
	// For now, just list other function names in the file
	var related []string
	for _, fn := range file.Functions {
		if fn.Name != targetFn.Name {
			related = append(related, fn.Name)
		}
	}

	if len(related) == 0 {
		return ""
	}

	return fmt.Sprintf("Related functions in this file: %v", related)
}

// splitLines splits content into lines
func splitLines(content string) []string {
	lines := make([]string, 0)
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, content[start:i])
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, content[start:])
	}
	return lines
}

// extractLines extracts lines from startLine to endLine (1-indexed)
func extractLines(lines []string, startLine, endLine int) string {
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	result := ""
	for i := startLine - 1; i < endLine; i++ {
		result += lines[i] + "\n"
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
