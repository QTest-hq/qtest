package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/e2e"
	"github.com/QTest-hq/qtest/internal/flow"
	"github.com/QTest-hq/qtest/internal/jobs"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/sidecar/playwright"
)

// E2EDiscoveryWorker discovers user flows from websites
type E2EDiscoveryWorker struct {
	*BaseWorker
	cfg       *config.Config
	llmRouter *llm.Router
}

func NewE2EDiscoveryWorker(base *BaseWorker, cfg *config.Config, llmRouter *llm.Router) *E2EDiscoveryWorker {
	w := &E2EDiscoveryWorker{BaseWorker: base, cfg: cfg, llmRouter: llmRouter}
	base.handler = w.handleJob
	return w
}

func (w *E2EDiscoveryWorker) Name() string { return "e2e_discovery" }

func (w *E2EDiscoveryWorker) handleJob(ctx context.Context, job *jobs.Job) error {
	var payload jobs.E2EDiscoveryPayload
	if err := job.GetPayload(&payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	log.Info().
		Str("url", payload.URL).
		Int("max_pages", payload.MaxPages).
		Msg("starting E2E flow discovery")

	// Set defaults
	if payload.MaxPages == 0 {
		payload.MaxPages = 5
	}
	if payload.PlaywrightURL == "" {
		payload.PlaywrightURL = "http://localhost:3000"
	}

	// Create playwright client
	pwClient, err := playwright.NewClient(payload.PlaywrightURL)
	if err != nil {
		return fmt.Errorf("failed to connect to playwright sidecar: %w", err)
	}

	// Create flow discovery
	discovery := flow.NewDiscovery(pwClient, w.llmRouter, nil)

	// Discover flows
	discoveryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	flows, err := discovery.ExploreAndDiscover(discoveryCtx, payload.URL, payload.MaxPages)
	if err != nil {
		return fmt.Errorf("flow discovery failed: %w", err)
	}

	log.Info().Int("flows", len(flows)).Msg("discovered flows")

	// Serialize flows to JSON
	flowsJSON, err := json.Marshal(flows)
	if err != nil {
		return fmt.Errorf("failed to marshal flows: %w", err)
	}

	result := jobs.E2EDiscoveryResult{
		FlowsDiscovered: len(flows),
		Flows:           flowsJSON,
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	return nil
}

// E2EGenerateWorker generates E2E tests from flows
type E2EGenerateWorker struct {
	*BaseWorker
	cfg       *config.Config
	llmRouter *llm.Router
}

func NewE2EGenerateWorker(base *BaseWorker, cfg *config.Config, llmRouter *llm.Router) *E2EGenerateWorker {
	w := &E2EGenerateWorker{BaseWorker: base, cfg: cfg, llmRouter: llmRouter}
	base.handler = w.handleJob
	return w
}

func (w *E2EGenerateWorker) Name() string { return "e2e_generation" }

func (w *E2EGenerateWorker) handleJob(ctx context.Context, job *jobs.Job) error {
	var payload jobs.E2EGeneratePayload
	if err := job.GetPayload(&payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	log.Info().
		Str("framework", payload.Framework).
		Str("language", payload.Language).
		Bool("enhance", payload.Enhance).
		Msg("generating E2E tests")

	// Set defaults
	if payload.Framework == "" {
		payload.Framework = "playwright"
	}
	if payload.Language == "" {
		payload.Language = "typescript"
	}
	if payload.OutputDir == "" {
		payload.OutputDir = filepath.Join(os.TempDir(), "qtest-e2e", job.ID.String())
	}

	// Parse flows from payload
	var flows []flow.Flow
	if len(payload.Flows) > 0 {
		if err := json.Unmarshal(payload.Flows, &flows); err != nil {
			return fmt.Errorf("failed to parse flows: %w", err)
		}
	}

	if len(flows) == 0 {
		return fmt.Errorf("no flows provided for test generation")
	}

	// Create output directory
	if err := os.MkdirAll(payload.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create generation config
	genConfig := &e2e.GenerationConfig{
		Framework:       e2e.TestFramework(payload.Framework),
		Language:        e2e.TestLanguage(payload.Language),
		OutputDir:       payload.OutputDir,
		IncludeComments: true,
		GenerateHelpers: true,
	}

	generator := e2e.NewPlaywrightGenerator(genConfig)

	var totalTests int
	var totalSteps int
	var generatedFiles []string

	for _, f := range flows {
		// Set base URL if provided
		if payload.BaseURL != "" {
			f.StartURL = payload.BaseURL
		}

		// Convert flow to E2E spec
		spec := e2e.FlowToSpec(&f, nil)

		// Enhance with LLM if requested
		if payload.Enhance && w.llmRouter != nil {
			enhancer := e2e.NewLLMEnhancer(w.llmRouter, nil)
			enhanced, err := enhancer.Enhance(ctx, spec)
			if err == nil {
				spec = enhanced.EnhancedSpec
				spec.TestCases = append(spec.TestCases, enhanced.SuggestedTests...)
			}
		}

		// Generate tests
		result, err := generator.Generate(spec)
		if err != nil {
			log.Warn().Err(err).Str("flow", f.Name).Msg("failed to generate tests for flow")
			continue
		}

		// Write generated files
		for _, file := range result.Files {
			filePath := file.Path
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(payload.OutputDir, filepath.Base(file.Path))
			}

			dir := filepath.Dir(filePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Warn().Err(err).Str("dir", dir).Msg("failed to create directory")
				continue
			}

			if err := os.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
				log.Warn().Err(err).Str("file", filePath).Msg("failed to write file")
				continue
			}

			generatedFiles = append(generatedFiles, filePath)
			log.Info().Str("file", filePath).Int("tests", file.TestCount).Msg("generated test file")
		}

		totalTests += result.TestCount
		totalSteps += result.StepCount
	}

	result := jobs.E2EGenerateResult{
		TestsGenerated: totalTests,
		StepsCount:     totalSteps,
		OutputDir:      payload.OutputDir,
		Files:          generatedFiles,
	}

	log.Info().
		Int("tests", totalTests).
		Int("steps", totalSteps).
		Int("files", len(generatedFiles)).
		Msg("E2E test generation completed")

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	return nil
}

// E2ERunWorker runs E2E tests
type E2ERunWorker struct {
	*BaseWorker
	cfg *config.Config
}

func NewE2ERunWorker(base *BaseWorker, cfg *config.Config) *E2ERunWorker {
	w := &E2ERunWorker{BaseWorker: base, cfg: cfg}
	base.handler = w.handleJob
	return w
}

func (w *E2ERunWorker) Name() string { return "e2e_run" }

func (w *E2ERunWorker) handleJob(ctx context.Context, job *jobs.Job) error {
	var payload jobs.E2ERunPayload
	if err := job.GetPayload(&payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	log.Info().
		Str("test_dir", payload.TestDir).
		Str("browser", payload.Browser).
		Bool("headless", payload.Headless).
		Msg("running E2E tests")

	// Set defaults
	if payload.Browser == "" {
		payload.Browser = "chromium"
	}
	if payload.Workers == 0 {
		payload.Workers = 4
	}
	if payload.Retries == 0 {
		payload.Retries = 2
	}

	// Validate test directory exists
	if _, err := os.Stat(payload.TestDir); os.IsNotExist(err) {
		return fmt.Errorf("test directory not found: %s", payload.TestDir)
	}

	// Create runner config
	runnerConfig := &e2e.RunnerConfig{
		WorkDir:   payload.TestDir,
		TestDir:   payload.TestDir,
		OutputDir: filepath.Join(payload.TestDir, "test-results"),
		Headless:  payload.Headless,
		Timeout:   10 * time.Minute,
		Retries:   payload.Retries,
		Workers:   payload.Workers,
		Reporter:  "json",
		Browser:   payload.Browser,
		BaseURL:   payload.BaseURL,
	}

	runner := e2e.NewTestRunner(runnerConfig)

	// Run tests with timeout
	runCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	runResult, err := runner.RunTests(runCtx, payload.Pattern)
	if err != nil {
		return fmt.Errorf("test execution failed: %w", err)
	}

	result := jobs.E2ERunResult{
		TotalTests: runResult.TotalTests,
		Passed:     runResult.Passed,
		Failed:     runResult.Failed,
		Skipped:    runResult.Skipped,
		Duration:   runResult.Duration.String(),
	}

	log.Info().
		Int("total", result.TotalTests).
		Int("passed", result.Passed).
		Int("failed", result.Failed).
		Int("skipped", result.Skipped).
		Str("duration", result.Duration).
		Msg("E2E test run completed")

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	return nil
}
