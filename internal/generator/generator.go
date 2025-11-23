package generator

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/QTest-hq/qtest/pkg/dsl"
	"github.com/QTest-hq/qtest/pkg/model"
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
	UseIRSpec  bool   // Use IRSpec JSON mode for structured output
}

// GeneratedTest represents a generated test with metadata
type GeneratedTest struct {
	DSL       *dsl.TestDSL
	TestSpecs []model.TestSpec // New: rich test specs with proper assertions
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
			Bool("irspec", opts.UseIRSpec).
			Msg("generating test")

		var test *GeneratedTest
		var err error
		if opts.UseIRSpec {
			test, err = g.GenerateWithIRSpec(ctx, &fn, parsed, opts)
		} else {
			test, err = g.generateTestForFunction(ctx, &fn, parsed, opts)
		}
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

	// DEBUG: Log raw YAML for troubleshooting
	log.Debug().
		Str("function", fn.Name).
		Str("raw_yaml", yamlContent).
		Msg("LLM YAML response")

	// Convert LLM output to DSL (handles multiple formats)
	testDSL, err := ConvertToDSL(yamlContent, fn.Name, file.Path, string(file.Language))
	if err != nil {
		// Log full YAML content at debug level for troubleshooting
		log.Debug().
			Str("function", fn.Name).
			Str("yaml_content", yamlContent).
			Msg("full LLM response for failed parse")

		// Provide helpful error message with more context
		contentPreview := yamlContent
		if len(contentPreview) > 500 {
			contentPreview = contentPreview[:500] + "... (truncated, see debug logs for full content)"
		}
		return nil, fmt.Errorf("failed to parse LLM response as test DSL: %w\n\nLLM Output:\n%s", err, contentPreview)
	}

	// DEBUG: Log parsed DSL
	log.Debug().
		Str("function", fn.Name).
		Int("steps", len(testDSL.Steps)).
		Msg("converted DSL")
	for i, step := range testDSL.Steps {
		hasExpected := step.Expected != nil
		var expectedVal interface{}
		if hasExpected && step.Expected.Value != nil {
			expectedVal = step.Expected.Value
		}
		log.Debug().
			Int("step", i).
			Str("desc", step.Description).
			Interface("args", step.Action.Args).
			Bool("has_expected", hasExpected).
			Interface("expected_value", expectedVal).
			Msg("DSL step")
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

	// Also convert to TestSpec with proper Assertions
	var testSpecs []model.TestSpec
	specs, specErr := ConvertToTestSpec(yamlContent, fn.Name, file.Path, string(file.Language))
	if specErr != nil {
		log.Debug().Err(specErr).Str("function", fn.Name).Msg("failed to convert to TestSpec, using DSL only")
	} else {
		testSpecs = specs
		log.Debug().
			Str("function", fn.Name).
			Int("specs", len(testSpecs)).
			Msg("converted to TestSpecs")
		for i, spec := range testSpecs {
			log.Debug().
				Int("spec", i).
				Str("desc", spec.Description).
				Int("assertions", len(spec.Assertions)).
				Msg("TestSpec")
		}
	}

	return &GeneratedTest{
		DSL:       testDSL,
		TestSpecs: testSpecs,
		RawYAML:   yamlContent,
		Function:  fn,
		FileName:  file.Path,
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

// GenerateWithIRSpec generates tests using the new IRSpec JSON format
// This uses Ollama's JSON mode for structured, parseable output
func (g *Generator) GenerateWithIRSpec(ctx context.Context, fn *parser.Function, file *parser.ParsedFile, opts GenerateOptions) (*GeneratedTest, error) {
	// Read the file content to get the function body
	content, err := os.ReadFile(file.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Extract function body from content
	lines := splitLines(string(content))
	functionCode := extractLines(lines, fn.StartLine, fn.EndLine)

	// Create the IRSpec prompt
	prompt := llm.IRSpecGenerationPrompt(
		functionCode,
		fn.Name,
		file.Path,
		string(file.Language),
	)

	// Call LLM with JSON mode enabled
	resp, err := g.llmRouter.Complete(ctx, &llm.Request{
		Tier:     opts.Tier,
		System:   llm.SystemPromptIRSpec,
		Messages: []llm.Message{{Role: "user", Content: prompt}},
		JSONMode: true, // Force JSON output
		Temperature: 0.2,
		MaxTokens:   3000,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	log.Debug().
		Str("function", fn.Name).
		Str("raw_json", resp.Content).
		Msg("LLM IRSpec JSON response")

	// Parse and convert IRSpec to TestSpecs
	converter := NewIRSpecConverter()
	testSpecs, err := converter.ParseAndConvert(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse IRSpec: %w\n\nLLM Output:\n%s", err, resp.Content)
	}

	log.Info().
		Str("function", fn.Name).
		Int("test_specs", len(testSpecs)).
		Msg("generated tests via IRSpec")

	// Convert TestSpecs to DSL for backward compatibility
	testDSL := convertTestSpecsToDSL(testSpecs, fn.Name, file.Path, opts.TestType)

	return &GeneratedTest{
		DSL:       testDSL,
		TestSpecs: testSpecs,
		RawYAML:   resp.Content, // Store JSON in RawYAML field for now
		Function:  fn,
		FileName:  file.Path,
	}, nil
}

// convertTestSpecsToDSL converts TestSpecs back to DSL for backward compatibility
func convertTestSpecsToDSL(specs []model.TestSpec, functionName, filePath string, testType dsl.TestType) *dsl.TestDSL {
	testDSL := &dsl.TestDSL{
		Version: "1.0",
		Type:    testType,
		Target: dsl.TestTarget{
			File:     filePath,
			Function: functionName,
		},
		Steps: make([]dsl.TestStep, 0, len(specs)),
	}

	for _, spec := range specs {
		// Convert inputs to args slice
		args := make([]interface{}, 0)
		for _, v := range spec.Inputs {
			args = append(args, v)
		}

		step := dsl.TestStep{
			Description: spec.Description,
			Input:       spec.Inputs,
			Action: dsl.StepAction{
				Type:   dsl.ActionCall,
				Target: functionName,
				Args:   args,
			},
		}

		// Convert assertions to expected
		if len(spec.Assertions) > 0 {
			a := spec.Assertions[0]
			step.Expected = &dsl.Expected{
				Value: a.Expected,
			}
		}

		testDSL.Steps = append(testDSL.Steps, step)
	}

	return testDSL
}

// BatchOptions configures batch test generation
type BatchOptions struct {
	GenerateOptions
	Concurrency int      // Number of concurrent generations (default: 4)
	Files       []string // Files to process
	OnProgress  func(completed, total int, current string) // Progress callback
}

// BatchResult contains results from batch generation
type BatchResult struct {
	Tests      []GeneratedTest     // Successfully generated tests
	Errors     []BatchError        // Errors encountered
	TotalFiles int                 // Total files processed
	TotalFuncs int                 // Total functions found
	Generated  int                 // Successfully generated tests
	Failed     int                 // Failed generations
}

// BatchError represents an error during batch generation
type BatchError struct {
	File     string
	Function string
	Error    error
}

// GenerateBatch generates tests for multiple files concurrently
func (g *Generator) GenerateBatch(ctx context.Context, opts BatchOptions) (*BatchResult, error) {
	if len(opts.Files) == 0 {
		return nil, fmt.Errorf("no files specified for batch generation")
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}

	result := &BatchResult{
		Tests:      make([]GeneratedTest, 0),
		Errors:     make([]BatchError, 0),
		TotalFiles: len(opts.Files),
	}

	// Parse all files first to get function count
	type parsedFile struct {
		file   *parser.ParsedFile
		path   string
		err    error
	}
	parsedFiles := make([]parsedFile, 0, len(opts.Files))
	for _, filePath := range opts.Files {
		parsed, err := g.parser.ParseFile(ctx, filePath)
		if err != nil {
			result.Errors = append(result.Errors, BatchError{
				File:  filePath,
				Error: fmt.Errorf("parse error: %w", err),
			})
			result.Failed++
			continue
		}
		parsedFiles = append(parsedFiles, parsedFile{file: parsed, path: filePath})
		result.TotalFuncs += len(parsed.Functions)
	}

	log.Info().
		Int("files", len(parsedFiles)).
		Int("functions", result.TotalFuncs).
		Int("concurrency", concurrency).
		Msg("starting batch generation")

	// Create work items for all functions
	type workItem struct {
		file *parser.ParsedFile
		fn   *parser.Function
	}
	work := make([]workItem, 0)
	for _, pf := range parsedFiles {
		for i := range pf.file.Functions {
			fn := &pf.file.Functions[i]
			// Skip private functions for unit tests
			if !fn.Exported && opts.TestType == dsl.TestTypeUnit {
				continue
			}
			work = append(work, workItem{file: pf.file, fn: fn})
		}
	}

	// Apply MaxTests limit
	if opts.MaxTests > 0 && len(work) > opts.MaxTests {
		work = work[:opts.MaxTests]
	}

	// Create channels for work distribution
	workCh := make(chan workItem, len(work))
	resultCh := make(chan *GeneratedTest, len(work))
	errorCh := make(chan BatchError, len(work))
	doneCh := make(chan struct{})

	// Track progress
	var completedMu sync.Mutex
	completed := 0

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for item := range workCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				log.Debug().
					Int("worker", workerID).
					Str("function", item.fn.Name).
					Msg("generating test")

				var test *GeneratedTest
				var err error
				if opts.UseIRSpec {
					test, err = g.GenerateWithIRSpec(ctx, item.fn, item.file, opts.GenerateOptions)
				} else {
					test, err = g.generateTestForFunction(ctx, item.fn, item.file, opts.GenerateOptions)
				}

				if err != nil {
					errorCh <- BatchError{
						File:     item.file.Path,
						Function: item.fn.Name,
						Error:    err,
					}
				} else {
					resultCh <- test
				}

				// Update progress
				completedMu.Lock()
				completed++
				if opts.OnProgress != nil {
					opts.OnProgress(completed, len(work), item.fn.Name)
				}
				completedMu.Unlock()
			}
		}(i)
	}

	// Send work to workers
	go func() {
		for _, item := range work {
			workCh <- item
		}
		close(workCh)
	}()

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	// Collect results
	collecting := true
	for collecting {
		select {
		case test := <-resultCh:
			result.Tests = append(result.Tests, *test)
			result.Generated++
		case batchErr := <-errorCh:
			result.Errors = append(result.Errors, batchErr)
			result.Failed++
		case <-doneCh:
			// Drain remaining results
			for {
				select {
				case test := <-resultCh:
					result.Tests = append(result.Tests, *test)
					result.Generated++
				case batchErr := <-errorCh:
					result.Errors = append(result.Errors, batchErr)
					result.Failed++
				default:
					collecting = false
				}
				if !collecting {
					break
				}
			}
		case <-ctx.Done():
			collecting = false
		}
	}

	log.Info().
		Int("generated", result.Generated).
		Int("failed", result.Failed).
		Int("total_tests", len(result.Tests)).
		Msg("batch generation complete")

	return result, nil
}

// GenerateForFiles generates tests for multiple files sequentially
// Use GenerateBatch for concurrent generation
func (g *Generator) GenerateForFiles(ctx context.Context, files []string, opts GenerateOptions) (*BatchResult, error) {
	result := &BatchResult{
		Tests:      make([]GeneratedTest, 0),
		Errors:     make([]BatchError, 0),
		TotalFiles: len(files),
	}

	for _, filePath := range files {
		if opts.MaxTests > 0 && result.Generated >= opts.MaxTests {
			break
		}

		tests, err := g.GenerateForFile(ctx, filePath, opts)
		if err != nil {
			result.Errors = append(result.Errors, BatchError{
				File:  filePath,
				Error: err,
			})
			result.Failed++
			continue
		}

		result.Tests = append(result.Tests, tests...)
		result.Generated += len(tests)
	}

	return result, nil
}
