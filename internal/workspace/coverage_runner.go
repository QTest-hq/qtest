package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/QTest-hq/qtest/internal/codecov"
	"github.com/QTest-hq/qtest/internal/emitter"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/specgen"
	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/rs/zerolog/log"
)

// CoverageRunner generates tests based on actual code coverage gaps
type CoverageRunner struct {
	ws        *Workspace
	llmRouter *llm.Router
	emitters  *emitter.Registry
	cfg       *CoverageRunConfig

	// Coverage state
	collector *codecov.Collector
	analyzer  *codecov.Analyzer
	report    *codecov.CoverageReport

	// Callbacks
	OnProgress func(phase string, current, total int, message string)
	OnComplete func(testFile string, testsCount int)
	OnCoverage func(before, after float64)
}

// CoverageRunConfig configures coverage-guided generation
type CoverageRunConfig struct {
	Tier           llm.Tier
	TargetCoverage float64 // Target coverage percentage (default: 80%)
	MaxIterations  int     // Max generation iterations (default: 5)
	MaxTestsPerRun int     // Max tests per iteration (default: 10)
	TestDir        string  // Output directory for tests
	RunTests       bool    // Run tests after generation
	CommitEach     bool    // Commit after each iteration
	FocusCritical  bool    // Focus on critical gaps first
}

// DefaultCoverageRunConfig returns sensible defaults
func DefaultCoverageRunConfig() *CoverageRunConfig {
	return &CoverageRunConfig{
		Tier:           llm.Tier2,
		TargetCoverage: 80.0,
		MaxIterations:  5,
		MaxTestsPerRun: 10,
		TestDir:        "tests",
		RunTests:       true,
		CommitEach:     false,
		FocusCritical:  true,
	}
}

// NewCoverageRunner creates a coverage-guided runner
func NewCoverageRunner(ws *Workspace, llmRouter *llm.Router, cfg *CoverageRunConfig) *CoverageRunner {
	if cfg == nil {
		cfg = DefaultCoverageRunConfig()
	}

	return &CoverageRunner{
		ws:        ws,
		llmRouter: llmRouter,
		emitters:  emitter.NewRegistry(),
		cfg:       cfg,
		collector: codecov.NewCollector(ws.RepoPath, ws.Language),
	}
}

// Run executes coverage-guided test generation
func (r *CoverageRunner) Run(ctx context.Context) error {
	r.ws.SetPhase(PhaseGenerating)
	now := time.Now()
	r.ws.State.StartedAt = &now

	log.Info().
		Float64("target", r.cfg.TargetCoverage).
		Int("maxIterations", r.cfg.MaxIterations).
		Msg("starting coverage-guided generation")

	for iteration := 1; iteration <= r.cfg.MaxIterations; iteration++ {
		select {
		case <-ctx.Done():
			r.ws.SetPhase(PhasePaused)
			return r.ws.Save()
		default:
		}

		r.reportProgress("collecting", iteration, r.cfg.MaxIterations, "Collecting code coverage...")

		// Collect current coverage
		report, err := r.collector.Collect(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("failed to collect coverage, continuing")
			// Continue with empty report for first iteration
			if iteration == 1 {
				report = &codecov.CoverageReport{
					Language: r.ws.Language,
					Files:    make([]codecov.FileCoverage, 0),
				}
			} else {
				continue
			}
		}
		r.report = report

		log.Info().
			Float64("coverage", report.Percentage).
			Int("iteration", iteration).
			Msg("current coverage")

		// Check if target reached
		if report.Percentage >= r.cfg.TargetCoverage {
			log.Info().Float64("coverage", report.Percentage).Msg("target coverage reached!")
			break
		}

		// Analyze gaps
		r.reportProgress("analyzing", iteration, r.cfg.MaxIterations, "Analyzing coverage gaps...")

		r.analyzer = codecov.NewAnalyzer(report, nil)
		result := r.analyzer.Analyze(r.cfg.TargetCoverage)

		if len(result.Gaps) == 0 {
			log.Info().Msg("no coverage gaps found")
			break
		}

		log.Info().
			Int("gaps", len(result.Gaps)).
			Int("critical", result.CriticalGaps).
			Msg("found coverage gaps")

		// Generate test intents from gaps
		intents := r.analyzer.GenerateTestIntents(result.Gaps)

		// Limit tests per iteration
		if len(intents) > r.cfg.MaxTestsPerRun {
			intents = intents[:r.cfg.MaxTestsPerRun]
		}

		// Sort by priority if focusing on critical
		if r.cfg.FocusCritical {
			intents = r.sortIntentsByPriority(intents, result.Gaps)
		}

		// Generate tests for these intents
		r.reportProgress("generating", iteration, r.cfg.MaxIterations,
			fmt.Sprintf("Generating %d tests for coverage gaps...", len(intents)))

		beforeCoverage := report.Percentage
		specs, err := r.generateSpecs(ctx, intents)
		if err != nil {
			log.Warn().Err(err).Msg("spec generation failed")
			continue
		}

		if len(specs) == 0 {
			log.Warn().Msg("no specs generated")
			continue
		}

		// Emit tests
		testFile, err := r.emitTests(specs)
		if err != nil {
			log.Warn().Err(err).Msg("failed to emit tests")
			continue
		}

		if r.OnComplete != nil {
			r.OnComplete(testFile, len(specs))
		}

		// Run tests to verify and update coverage
		if r.cfg.RunTests {
			r.reportProgress("testing", iteration, r.cfg.MaxIterations, "Running tests...")

			// Re-collect coverage after tests
			newReport, err := r.collector.Collect(ctx)
			if err == nil {
				r.report = newReport
				if r.OnCoverage != nil {
					r.OnCoverage(beforeCoverage, newReport.Percentage)
				}

				log.Info().
					Float64("before", beforeCoverage).
					Float64("after", newReport.Percentage).
					Msg("coverage changed")
			}
		}

		// Update workspace state
		r.ws.State.Completed += len(specs)
		r.ws.Save()
	}

	// Final status
	r.ws.SetPhase(PhaseCompleted)

	if r.report != nil {
		log.Info().
			Float64("finalCoverage", r.report.Percentage).
			Int("testsGenerated", r.ws.State.Completed).
			Msg("coverage-guided generation complete")
	}

	return r.ws.Save()
}

// generateSpecs generates test specs from coverage intents
func (r *CoverageRunner) generateSpecs(ctx context.Context, intents []model.TestIntent) ([]model.TestSpec, error) {
	specGen := specgen.NewGenerator(r.llmRouter, r.cfg.Tier)

	// Create minimal system model for spec generation
	sysModel := &model.SystemModel{
		ID:         r.ws.Name + "-coverage",
		Repository: r.ws.Name,
		Functions:  make([]model.Function, 0),
	}

	// Build functions from coverage data if available
	if r.report != nil {
		for _, file := range r.report.Files {
			fn := model.Function{
				ID:   file.Path,
				Name: filepath.Base(file.Path),
				File: file.Path,
			}
			sysModel.Functions = append(sysModel.Functions, fn)
		}
	}

	var specs []model.TestSpec
	for _, intent := range intents {
		spec, err := specGen.GenerateSpec(ctx, intent, sysModel)
		if err != nil {
			log.Warn().Err(err).Str("intent", intent.ID).Msg("failed to generate spec")
			continue
		}
		specs = append(specs, *spec)
	}

	return specs, nil
}

// emitTests generates test code from specs
func (r *CoverageRunner) emitTests(specs []model.TestSpec) (string, error) {
	if len(specs) == 0 {
		return "", nil
	}

	// Choose emitter based on language
	var em emitter.Emitter
	var err error

	switch r.ws.Language {
	case "javascript", "typescript":
		em, err = r.emitters.Get("supertest")
	case "python":
		em, err = r.emitters.Get("pytest")
	case "go":
		em, err = r.emitters.Get("go-http")
	default:
		em, err = r.emitters.Get("supertest")
	}

	if err != nil {
		return "", err
	}

	// Generate code
	code, err := em.Emit(specs)
	if err != nil {
		return "", err
	}

	// Write to file
	testDir := filepath.Join(r.ws.RepoPath, r.cfg.TestDir)
	testFile := filepath.Join(testDir, "coverage_generated"+em.FileExtension())

	if err := writeOrAppendTest(testFile, code, r.ws.Language); err != nil {
		return "", err
	}

	return testFile, nil
}

// sortIntentsByPriority sorts intents to prioritize critical gaps
func (r *CoverageRunner) sortIntentsByPriority(intents []model.TestIntent, gaps []codecov.CoverageGap) []model.TestIntent {
	// Build priority map from gaps
	priorityMap := make(map[string]int)
	for _, gap := range gaps {
		priority := 0
		switch gap.Priority {
		case "critical":
			priority = 4
		case "high":
			priority = 3
		case "medium":
			priority = 2
		case "low":
			priority = 1
		}
		priorityMap[gap.File] = priority
	}

	// Simple bubble sort (small lists)
	for i := 0; i < len(intents)-1; i++ {
		for j := i + 1; j < len(intents); j++ {
			pi := priorityMap[intents[i].TargetID]
			pj := priorityMap[intents[j].TargetID]
			if pj > pi {
				intents[i], intents[j] = intents[j], intents[i]
			}
		}
	}

	return intents
}

func (r *CoverageRunner) reportProgress(phase string, current, total int, message string) {
	if r.OnProgress != nil {
		r.OnProgress(phase, current, total, message)
	}
}

// GetCoverageReport returns the latest coverage report
func (r *CoverageRunner) GetCoverageReport() *codecov.CoverageReport {
	return r.report
}

// writeOrAppendTest writes or appends to a test file
func writeOrAppendTest(testFile, code, language string) error {
	// Create directory if needed
	if err := ensureDir(filepath.Dir(testFile)); err != nil {
		return err
	}

	// Check if file exists
	existing, err := readFileIfExists(testFile)
	if err != nil {
		return err
	}

	if existing == "" {
		// New file
		return writeFile(testFile, code)
	}

	// Append to existing file
	newCode := extractTestBlocks(code, language)
	combined := existing + "\n// === Coverage-guided tests ===\n\n" + newCode

	return writeFile(testFile, combined)
}

// ensureDir creates directory if it doesn't exist
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// readFileIfExists reads a file if it exists, returns empty string if not
func readFileIfExists(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeFile writes content to a file
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
