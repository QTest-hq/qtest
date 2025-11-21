package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QTest-hq/qtest/internal/emitter"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/QTest-hq/qtest/internal/specgen"
	"github.com/QTest-hq/qtest/internal/supplements"
	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/rs/zerolog/log"
)

// RunnerV2 uses the new SystemModel-based pipeline
type RunnerV2 struct {
	ws        *Workspace
	git       *GitManager
	parser    *parser.Parser
	llmRouter *llm.Router
	emitters  *emitter.Registry
	cfg       *RunConfig

	// Pipeline state
	sysModel *model.SystemModel
	testPlan *model.TestPlan
	specSet  *model.TestSpecSet

	// Callbacks
	OnProgress func(phase string, current, total int, message string)
	OnComplete func(testFile string, specsCount int)
	OnError    func(err error)
}

// NewRunnerV2 creates a new v2 runner with SystemModel pipeline
func NewRunnerV2(ws *Workspace, llmRouter *llm.Router, gitToken string, cfg *RunConfig) *RunnerV2 {
	if cfg == nil {
		cfg = DefaultRunConfig()
	}

	return &RunnerV2{
		ws:        ws,
		git:       NewGitManager(ws, gitToken),
		parser:    parser.NewParser(),
		llmRouter: llmRouter,
		emitters:  emitter.NewRegistry(),
		cfg:       cfg,
	}
}

// Initialize builds the SystemModel and TestPlan
func (r *RunnerV2) Initialize(ctx context.Context) error {
	// Handle repository setup
	if _, err := os.Stat(r.ws.RepoPath); os.IsNotExist(err) {
		// Check if RepoURL is a local path or remote URL
		if isLocalPath(r.ws.RepoURL) {
			r.reportProgress("copying", 0, 1, "Copying local repository...")
			if err := copyDir(r.ws.RepoURL, r.ws.RepoPath); err != nil {
				return fmt.Errorf("copy failed: %w", err)
			}
		} else {
			r.reportProgress("cloning", 0, 1, "Cloning repository...")
			if err := r.git.Clone(ctx); err != nil {
				return fmt.Errorf("clone failed: %w", err)
			}
		}
	}

	// Build SystemModel
	r.reportProgress("modeling", 0, 3, "Building system model...")
	if err := r.buildSystemModel(ctx); err != nil {
		return fmt.Errorf("modeling failed: %w", err)
	}

	// Generate TestPlan
	r.reportProgress("planning", 1, 3, "Generating test plan...")
	if err := r.buildTestPlan(); err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	// Update workspace state
	r.ws.State.TotalTargets = len(r.testPlan.Intents)
	r.ws.SetPhase(PhasePlanning)

	// Save model and plan as artifacts
	r.reportProgress("saving", 2, 3, "Saving artifacts...")
	if err := r.saveArtifacts(); err != nil {
		log.Warn().Err(err).Msg("failed to save artifacts")
	}

	r.reportProgress("ready", 3, 3, fmt.Sprintf("Ready to generate %d tests", r.testPlan.TotalTests))

	return r.ws.Save()
}

// buildSystemModel creates the SystemModel from the repository
func (r *RunnerV2) buildSystemModel(ctx context.Context) error {
	adapter := model.NewParserAdapter(r.ws.Name, r.ws.BaseBranch, r.ws.CommitSHA)

	// Register supplements
	registry := supplements.NewRegistry()
	for _, supp := range registry.GetAll() {
		adapter.RegisterSupplement(supp)
	}

	// Walk and parse files
	fileCount := 0
	err := filepath.Walk(r.ws.RepoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories and common ignores
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip test files
		base := filepath.Base(path)
		if strings.Contains(base, "_test.") || strings.Contains(base, ".test.") || strings.HasPrefix(base, "test_") {
			return nil
		}

		// Only parse supported files
		ext := strings.ToLower(filepath.Ext(path))
		if !isSupportedExt(ext) {
			return nil
		}

		// Parse file
		parsed, err := r.parser.ParseFile(ctx, path)
		if err != nil {
			return nil
		}

		// Convert to model format
		pf := convertToModelParsedFile(parsed)
		adapter.AddFile(pf)
		fileCount++

		// Set workspace language from first file
		if r.ws.Language == "" {
			r.ws.Language = string(parsed.Language)
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Info().Int("files", fileCount).Msg("parsed repository")

	// Build model (runs supplements)
	sysModel, err := adapter.Build()
	if err != nil {
		return err
	}

	r.sysModel = sysModel

	log.Info().
		Int("functions", len(sysModel.Functions)).
		Int("endpoints", len(sysModel.Endpoints)).
		Msg("built system model")

	return nil
}

// buildTestPlan creates prioritized TestIntents
func (r *RunnerV2) buildTestPlan() error {
	cfg := model.DefaultPlannerConfig()
	planner := model.NewPlanner(cfg)

	plan, err := planner.Plan(r.sysModel)
	if err != nil {
		return err
	}

	r.testPlan = plan

	// Update workspace targets from plan
	for _, intent := range plan.Intents {
		r.ws.State.Targets[intent.ID] = &TargetState{
			ID:     intent.ID,
			Name:   intent.Reason,
			Type:   string(intent.Level),
			Status: StatusPending,
		}
	}

	log.Info().
		Int("total", plan.TotalTests).
		Int("api", plan.APITests).
		Int("unit", plan.UnitTests).
		Msg("generated test plan")

	return nil
}

// Run executes incremental test generation
func (r *RunnerV2) Run(ctx context.Context) error {
	r.ws.SetPhase(PhaseGenerating)
	now := time.Now()
	r.ws.State.StartedAt = &now

	// Load artifacts if resuming
	if r.sysModel == nil || r.testPlan == nil {
		if err := r.loadArtifacts(); err != nil {
			return fmt.Errorf("failed to load artifacts: %w", err)
		}
	}

	// Create spec generator
	specGen := specgen.NewGenerator(r.llmRouter, r.cfg.Tier)

	// Initialize spec set if needed
	if r.specSet == nil {
		r.specSet = &model.TestSpecSet{
			ModelID:    r.sysModel.ID,
			Repository: r.ws.Name,
			Specs:      make([]model.TestSpec, 0),
		}
	}

	// Track new specs for incremental emission
	var newAPISpecs []model.TestSpec
	var newUnitSpecs []model.TestSpec

	// Track generated count for MaxTests limit
	generatedCount := 0

	// Process intents incrementally
	total := len(r.testPlan.Intents)
	processed := 0

	// Group intents by level for batching
	apiIntents := filterIntentsByLevel(r.testPlan.Intents, model.LevelAPI)
	unitIntents := filterIntentsByLevel(r.testPlan.Intents, model.LevelUnit)

	// Process API tests first
	if len(apiIntents) > 0 {
		r.reportProgress("generating", processed, total, "Generating API test specs...")

		for _, intent := range apiIntents {
			// Check MaxTests limit
			if r.cfg.MaxTests > 0 && generatedCount >= r.cfg.MaxTests {
				log.Info().Int("max", r.cfg.MaxTests).Msg("reached max tests limit")
				break
			}

			select {
			case <-ctx.Done():
				r.ws.SetPhase(PhasePaused)
				return r.ws.Save()
			default:
			}

			// Skip if already covered (has spec generated)
			if target, ok := r.ws.State.Targets[intent.ID]; ok && target.Covered {
				processed++
				continue
			}

			processed++
			r.reportProgress("generating", processed, total, fmt.Sprintf("API: %s", intent.Reason))

			// Generate spec
			spec, err := specGen.GenerateSpec(ctx, intent, r.sysModel)
			if err != nil {
				r.ws.UpdateTarget(intent.ID, StatusFailed, "", err)
				log.Warn().Err(err).Str("intent", intent.ID).Msg("spec generation failed")
				continue
			}

			// Add to full spec set and new specs
			r.specSet.Specs = append(r.specSet.Specs, *spec)
			newAPISpecs = append(newAPISpecs, *spec)
			generatedCount++

			// Mark target as covered with spec ID
			r.ws.UpdateTargetCovered(intent.ID, spec.ID)
			r.ws.State.Completed++

			// Save progress periodically
			if processed%5 == 0 {
				r.ws.Save()
			}
		}

		// Emit only NEW API tests
		if len(newAPISpecs) > 0 {
			if err := r.emitNewTests(newAPISpecs, model.LevelAPI); err != nil {
				log.Warn().Err(err).Msg("failed to emit API tests")
			}
		} else {
			log.Info().Msg("no new API tests to emit")
		}
	}

	// Process unit tests (if not already at MaxTests limit)
	if len(unitIntents) > 0 && (r.cfg.MaxTests == 0 || generatedCount < r.cfg.MaxTests) {
		r.reportProgress("generating", processed, total, "Generating unit test specs...")

		for _, intent := range unitIntents {
			// Check MaxTests limit
			if r.cfg.MaxTests > 0 && generatedCount >= r.cfg.MaxTests {
				log.Info().Int("max", r.cfg.MaxTests).Msg("reached max tests limit")
				break
			}

			select {
			case <-ctx.Done():
				r.ws.SetPhase(PhasePaused)
				return r.ws.Save()
			default:
			}

			// Skip if already covered
			if target, ok := r.ws.State.Targets[intent.ID]; ok && target.Covered {
				processed++
				continue
			}

			processed++
			r.reportProgress("generating", processed, total, fmt.Sprintf("Unit: %s", intent.Reason))

			// Generate spec
			spec, err := specGen.GenerateSpec(ctx, intent, r.sysModel)
			if err != nil {
				r.ws.UpdateTarget(intent.ID, StatusFailed, "", err)
				continue
			}

			r.specSet.Specs = append(r.specSet.Specs, *spec)
			newUnitSpecs = append(newUnitSpecs, *spec)
			generatedCount++

			r.ws.UpdateTargetCovered(intent.ID, spec.ID)
			r.ws.State.Completed++
		}

		// Emit only NEW unit tests
		if len(newUnitSpecs) > 0 {
			if err := r.emitNewTests(newUnitSpecs, model.LevelUnit); err != nil {
				log.Warn().Err(err).Msg("failed to emit unit tests")
			}
		} else {
			log.Info().Msg("no new unit tests to emit")
		}
	}

	r.ws.SetPhase(PhaseCompleted)
	r.reportProgress("complete", total, total, fmt.Sprintf("Generated %d tests", len(r.specSet.Specs)))

	// Save final artifacts
	r.saveArtifacts()

	return r.ws.Save()
}

// emitTests generates test code from specs
func (r *RunnerV2) emitTests(level model.TestLevel) error {
	specs := r.specSet.FilterByLevel(level)
	if len(specs) == 0 {
		return nil
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
		em, err = r.emitters.Get("supertest") // Default
	}

	if err != nil {
		return err
	}

	// Generate code
	code, err := em.Emit(specs)
	if err != nil {
		return err
	}

	// Determine output path
	testDir := filepath.Join(r.ws.RepoPath, "tests")
	if r.cfg.TestDir != "" {
		testDir = filepath.Join(r.ws.RepoPath, r.cfg.TestDir)
	}

	if err := os.MkdirAll(testDir, 0755); err != nil {
		return err
	}

	filename := string(level) + em.FileExtension()
	testFile := filepath.Join(testDir, filename)

	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		return err
	}

	log.Info().
		Str("file", testFile).
		Int("tests", len(specs)).
		Msg("emitted tests")

	// Commit if configured
	if r.cfg.CommitEach && !r.cfg.DryRun {
		if _, err := r.git.CommitTest(testFile, fmt.Sprintf("%s tests", level)); err != nil {
			log.Warn().Err(err).Msg("failed to commit tests")
		}
	}

	if r.OnComplete != nil {
		r.OnComplete(testFile, len(specs))
	}

	return nil
}

// emitNewTests appends new test specs to existing test file
func (r *RunnerV2) emitNewTests(specs []model.TestSpec, level model.TestLevel) error {
	if len(specs) == 0 {
		return nil
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
		return err
	}

	// Generate code for new tests
	code, err := em.Emit(specs)
	if err != nil {
		return err
	}

	// Determine output path
	testDir := filepath.Join(r.ws.RepoPath, "tests")
	if r.cfg.TestDir != "" {
		testDir = filepath.Join(r.ws.RepoPath, r.cfg.TestDir)
	}

	if err := os.MkdirAll(testDir, 0755); err != nil {
		return err
	}

	filename := string(level) + em.FileExtension()
	testFile := filepath.Join(testDir, filename)

	// Check if file exists - if so, append; otherwise create
	if _, err := os.Stat(testFile); err == nil {
		// File exists - append new tests
		// Read existing content
		existing, err := os.ReadFile(testFile)
		if err != nil {
			return err
		}

		// For JS/TS, we need to append test blocks without the imports
		// For Python, append test functions
		newCode := extractTestBlocks(code, r.ws.Language)

		// Append to file
		combined := string(existing) + "\n// === New tests added incrementally ===\n\n" + newCode
		if err := os.WriteFile(testFile, []byte(combined), 0644); err != nil {
			return err
		}

		log.Info().
			Str("file", testFile).
			Int("new_tests", len(specs)).
			Msg("appended new tests")
	} else {
		// File doesn't exist - create with full content
		if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
			return err
		}

		log.Info().
			Str("file", testFile).
			Int("tests", len(specs)).
			Msg("created test file")
	}

	// Commit if configured
	if r.cfg.CommitEach && !r.cfg.DryRun {
		if _, err := r.git.CommitTest(testFile, fmt.Sprintf("add %d new %s tests", len(specs), level)); err != nil {
			log.Warn().Err(err).Msg("failed to commit tests")
		}
	}

	if r.OnComplete != nil {
		r.OnComplete(testFile, len(specs))
	}

	return nil
}

// extractTestBlocks extracts test blocks without imports/setup
func extractTestBlocks(code string, language string) string {
	lines := strings.Split(code, "\n")
	var result []string
	inTestBlock := false
	braceCount := 0

	switch language {
	case "javascript", "typescript":
		// Extract describe/test blocks
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Skip top-level imports and requires (but keep code inside test blocks)
			if !inTestBlock {
				if strings.HasPrefix(trimmed, "const request") ||
					strings.HasPrefix(trimmed, "const app") ||
					strings.HasPrefix(trimmed, "import ") ||
					strings.HasPrefix(trimmed, "require(") {
					continue
				}
			}

			// Track describe blocks
			if strings.HasPrefix(trimmed, "describe(") {
				inTestBlock = true
			}

			if inTestBlock {
				result = append(result, line)
				braceCount += strings.Count(line, "{") - strings.Count(line, "}")
				if braceCount == 0 && len(result) > 1 {
					result = append(result, "")
					inTestBlock = false
				}
			}
		}
	case "python":
		// Extract def test_ functions
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Skip imports
			if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") {
				continue
			}

			// Skip client setup
			if strings.HasPrefix(trimmed, "client = ") {
				continue
			}

			result = append(result, line)
		}
	default:
		// Return as-is for other languages
		return code
	}

	return strings.Join(result, "\n")
}

// saveArtifacts saves model, plan, and specs to workspace
func (r *RunnerV2) saveArtifacts() error {
	artifactsDir := filepath.Join(r.ws.Path(), "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return err
	}

	if r.sysModel != nil {
		data, _ := json.MarshalIndent(r.sysModel, "", "  ")
		os.WriteFile(filepath.Join(artifactsDir, "model.json"), data, 0644)
	}

	if r.testPlan != nil {
		data, _ := json.MarshalIndent(r.testPlan, "", "  ")
		os.WriteFile(filepath.Join(artifactsDir, "plan.json"), data, 0644)
	}

	if r.specSet != nil {
		data, _ := json.MarshalIndent(r.specSet, "", "  ")
		os.WriteFile(filepath.Join(artifactsDir, "specs.json"), data, 0644)
	}

	return nil
}

// loadArtifacts loads saved artifacts for resuming
func (r *RunnerV2) loadArtifacts() error {
	artifactsDir := filepath.Join(r.ws.Path(), "artifacts")

	// Load model
	if data, err := os.ReadFile(filepath.Join(artifactsDir, "model.json")); err == nil {
		r.sysModel = &model.SystemModel{}
		json.Unmarshal(data, r.sysModel)
	}

	// Load plan
	if data, err := os.ReadFile(filepath.Join(artifactsDir, "plan.json")); err == nil {
		r.testPlan = &model.TestPlan{}
		json.Unmarshal(data, r.testPlan)
	}

	// Load specs
	if data, err := os.ReadFile(filepath.Join(artifactsDir, "specs.json")); err == nil {
		r.specSet = &model.TestSpecSet{}
		json.Unmarshal(data, r.specSet)
	}

	if r.sysModel == nil || r.testPlan == nil {
		return fmt.Errorf("artifacts not found, run Initialize first")
	}

	return nil
}

func (r *RunnerV2) reportProgress(phase string, current, total int, message string) {
	if r.OnProgress != nil {
		r.OnProgress(phase, current, total, message)
	}
}

// Pause pauses the run
func (r *RunnerV2) Pause() {
	r.ws.SetPhase(PhasePaused)
	now := time.Now()
	r.ws.State.PausedAt = &now
	r.ws.Save()
}

// Helper functions

func isSupportedExt(ext string) bool {
	switch ext {
	case ".go", ".py", ".js", ".jsx", ".ts", ".tsx":
		return true
	}
	return false
}

func filterIntentsByLevel(intents []model.TestIntent, level model.TestLevel) []model.TestIntent {
	var filtered []model.TestIntent
	for _, i := range intents {
		if i.Level == level {
			filtered = append(filtered, i)
		}
	}
	return filtered
}

func isLocalPath(path string) bool {
	// Check if it's a URL
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@") || strings.HasPrefix(path, "git://") {
		return false
	}
	// Check if it's an existing local path
	if _, err := os.Stat(path); err == nil {
		return true
	}
	// Check if it looks like a file path
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}

func copyDir(src, dst string) error {
	// Get absolute path of source
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}

	return filepath.Walk(srcAbs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(srcAbs, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		// Skip hidden files and common ignores
		if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == "node_modules" || info.Name() == "__pycache__" || info.Name() == "vendor" {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

func convertToModelParsedFile(pf *parser.ParsedFile) *model.ParsedFile {
	result := &model.ParsedFile{
		Path:     pf.Path,
		Language: string(pf.Language),
	}

	for _, fn := range pf.Functions {
		params := make([]model.ParserParameter, len(fn.Parameters))
		for i, p := range fn.Parameters {
			params[i] = model.ParserParameter{
				Name:     p.Name,
				Type:     p.Type,
				Default:  p.Default,
				Optional: p.Optional,
			}
		}

		result.Functions = append(result.Functions, model.ParserFunction{
			ID:         fn.ID,
			Name:       fn.Name,
			StartLine:  fn.StartLine,
			EndLine:    fn.EndLine,
			Parameters: params,
			ReturnType: fn.ReturnType,
			Body:       fn.Body,
			Comments:   fn.Comments,
			Exported:   fn.Exported,
			Async:      fn.Async,
			Class:      fn.Class,
		})
	}

	return result
}
