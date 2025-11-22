package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/adapters"
	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/db"
	"github.com/QTest-hq/qtest/internal/generator"
	"github.com/QTest-hq/qtest/internal/jobs"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/mutation"
	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/QTest-hq/qtest/pkg/dsl"
	"github.com/QTest-hq/qtest/pkg/model"
)

// IngestionWorker handles repository cloning and initial processing
type IngestionWorker struct {
	*BaseWorker
	store *db.Store
}

func NewIngestionWorker(base *BaseWorker, store *db.Store) *IngestionWorker {
	w := &IngestionWorker{BaseWorker: base, store: store}
	base.handler = w.handleJob
	return w
}

func (w *IngestionWorker) Name() string { return "ingestion" }

func (w *IngestionWorker) handleJob(ctx context.Context, job *jobs.Job) error {
	var payload jobs.IngestionPayload
	if err := job.GetPayload(&payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	log.Info().
		Str("repo_url", payload.RepositoryURL).
		Str("branch", payload.Branch).
		Msg("ingesting repository")

	// Check if repository already exists in database
	var repo *db.Repository
	var err error
	if w.store != nil {
		repo, err = w.store.GetRepositoryByURL(ctx, payload.RepositoryURL)
		if err != nil {
			log.Warn().Err(err).Msg("failed to check existing repository")
		}
	}

	// Extract repository name and owner from URL
	repoName, repoOwner := extractRepoInfo(payload.RepositoryURL)
	branch := payload.Branch
	if branch == "" {
		branch = "main"
	}

	// Create repository record if it doesn't exist
	if repo == nil && w.store != nil {
		repo = &db.Repository{
			URL:           payload.RepositoryURL,
			Name:          repoName,
			Owner:         repoOwner,
			DefaultBranch: branch,
			Status:        "cloning",
		}
		if err := w.store.CreateRepository(ctx, repo); err != nil {
			log.Warn().Err(err).Msg("failed to create repository record")
			// Continue without DB record - use generated UUID
			repo = &db.Repository{ID: uuid.New()}
		}
	} else if repo == nil {
		// No store available, use generated UUID
		repo = &db.Repository{ID: uuid.New()}
	}

	// Clone repository to workspace
	workspacePath := filepath.Join(os.TempDir(), "qtest", job.ID.String())
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		w.updateRepoStatus(ctx, repo.ID, "failed", nil)
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Git clone
	args := []string{"clone", "--depth", "1"}
	if payload.Branch != "" {
		args = append(args, "-b", payload.Branch)
	}
	args = append(args, payload.RepositoryURL, workspacePath)

	cmd := exec.CommandContext(ctx, "git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		w.updateRepoStatus(ctx, repo.ID, "failed", nil)
		return fmt.Errorf("git clone failed: %s: %w", string(output), err)
	}

	// Get commit SHA
	commitSHA := getCommitSHA(ctx, workspacePath)

	// Detect language and count files
	var fileCount int
	var language string
	filepath.Walk(workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		switch ext {
		case ".go":
			language = "go"
			fileCount++
		case ".py":
			if language == "" {
				language = "python"
			}
			fileCount++
		case ".ts", ".js":
			if language == "" {
				language = "typescript"
			}
			fileCount++
		}
		return nil
	})

	// Update repository status to ready
	w.updateRepoStatus(ctx, repo.ID, "ready", &commitSHA)

	// Create result
	result := jobs.IngestionResult{
		RepositoryID:  repo.ID,
		WorkspacePath: workspacePath,
		FileCount:     fileCount,
		Language:      language,
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	// Chain to modeling job with pipeline options
	if w.Pipeline() != nil {
		opts := jobs.PipelineJobOptions{
			MaxTests:    payload.MaxTests,
			LLMTier:     payload.LLMTier,
			RunMutation: payload.RunMutation,
			CreatePR:    payload.CreatePR,
		}
		_, err := w.Pipeline().CreateModelingJob(ctx, job.ID, result.RepositoryID, workspacePath, opts)
		if err != nil {
			log.Warn().Err(err).Msg("failed to create modeling job")
		}
	}

	return nil
}

// updateRepoStatus updates the repository status if store is available
func (w *IngestionWorker) updateRepoStatus(ctx context.Context, repoID uuid.UUID, status string, commitSHA *string) {
	if w.store != nil {
		if err := w.store.UpdateRepositoryStatus(ctx, repoID, status, commitSHA); err != nil {
			log.Warn().Err(err).Str("repo_id", repoID.String()).Msg("failed to update repository status")
		}
	}
}

// extractRepoInfo extracts repository name and owner from URL
func extractRepoInfo(url string) (name, owner string) {
	// Handle SSH URLs: git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@") {
		parts := strings.Split(url, ":")
		if len(parts) == 2 {
			path := strings.TrimSuffix(parts[1], ".git")
			pathParts := strings.Split(path, "/")
			if len(pathParts) >= 2 {
				return pathParts[len(pathParts)-1], pathParts[len(pathParts)-2]
			}
		}
	}

	// Handle HTTP URLs: https://github.com/owner/repo.git
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1], parts[len(parts)-2]
	}

	return "unknown", "unknown"
}

// getCommitSHA gets the current commit SHA from the repository
func getCommitSHA(ctx context.Context, workspacePath string) string {
	cmd := exec.CommandContext(ctx, "git", "-C", workspacePath, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// ModelingWorker builds system models from parsed code
type ModelingWorker struct {
	*BaseWorker
	store *db.Store
}

func NewModelingWorker(base *BaseWorker, store *db.Store) *ModelingWorker {
	w := &ModelingWorker{BaseWorker: base, store: store}
	base.handler = w.handleJob
	return w
}

func (w *ModelingWorker) Name() string { return "modeling" }

func (w *ModelingWorker) handleJob(ctx context.Context, job *jobs.Job) error {
	var payload jobs.ModelingPayload
	if err := job.GetPayload(&payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	log.Info().
		Str("workspace", payload.WorkspacePath).
		Str("repo_id", payload.RepositoryID.String()).
		Msg("modeling repository")

	// Parse all source files and build rich SystemModel
	p := parser.NewParser()

	// Get repository info
	repoName := filepath.Base(payload.WorkspacePath)
	commitSHA := getCommitSHA(ctx, payload.WorkspacePath)

	// Build rich SystemModel using pkg/model
	sysModel, err := model.BuildSystemModelFromParser(ctx, p, payload.WorkspacePath, repoName, "main", commitSHA)
	if err != nil {
		log.Warn().Err(err).Msg("failed to build system model, falling back to basic parsing")
		// Fall back to basic parsing if model building fails
		return w.handleJobLegacy(ctx, job, payload)
	}

	stats := sysModel.Stats()
	log.Info().
		Int("modules", stats["modules"]).
		Int("functions", stats["functions"]).
		Int("types", stats["types"]).
		Int("endpoints", stats["endpoints"]).
		Int("test_targets", stats["test_targets"]).
		Strs("languages", sysModel.Languages).
		Msg("built system model")

	// Serialize the rich model to JSON
	modelJSON, err := json.Marshal(sysModel)
	if err != nil {
		return fmt.Errorf("failed to serialize model: %w", err)
	}

	// Create system model in database
	modelID := uuid.New()
	if w.store != nil {
		if err := w.store.CreateSystemModel(ctx, &db.SystemModel{
			ID:           modelID,
			RepositoryID: payload.RepositoryID,
			CommitSHA:    commitSHA,
			ModelData:    modelJSON,
		}); err != nil {
			log.Warn().Err(err).Msg("failed to persist system model")
			// Continue without DB persistence
		}
	}

	result := jobs.ModelingResult{
		ModelID:       modelID,
		FileCount:     stats["modules"],
		FunctionCount: stats["functions"],
		EndpointCount: stats["endpoints"],
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	// Chain to planning job with pipeline options
	if w.Pipeline() != nil {
		opts := jobs.PipelineJobOptions{
			MaxTests:    payload.MaxTests,
			LLMTier:     payload.LLMTier,
			RunMutation: payload.RunMutation,
			CreatePR:    payload.CreatePR,
		}
		_, err := w.Pipeline().CreatePlanningJob(ctx, job.ID, payload.RepositoryID, result.ModelID, opts)
		if err != nil {
			log.Warn().Err(err).Msg("failed to create planning job")
		}
	}

	return nil
}

// handleJobLegacy is the fallback handler for basic parsing
func (w *ModelingWorker) handleJobLegacy(ctx context.Context, job *jobs.Job, payload jobs.ModelingPayload) error {
	p := parser.NewParser()
	var functionCount, fileCount int
	var functions []modelFunction
	var endpoints []modelEndpoint

	err := filepath.Walk(payload.WorkspacePath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}

		// Skip excluded paths
		for _, excl := range payload.ExcludePaths {
			if strings.Contains(path, excl) {
				return nil
			}
		}

		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".py" && ext != ".ts" && ext != ".js" {
			return nil
		}

		fileCount++
		result, parseErr := p.ParseFile(ctx, path)
		if parseErr != nil {
			log.Warn().Err(parseErr).Str("file", path).Msg("failed to parse file")
			return nil
		}

		// Collect function information
		relPath, _ := filepath.Rel(payload.WorkspacePath, path)
		for _, fn := range result.Functions {
			functionCount++
			functions = append(functions, modelFunction{
				Name:      fn.Name,
				File:      relPath,
				StartLine: fn.StartLine,
				EndLine:   fn.EndLine,
				Exported:  fn.Exported,
			})

			// Detect HTTP handler functions (basic heuristic)
			if isHTTPHandler(fn.Name) {
				endpoints = append(endpoints, modelEndpoint{
					Function: fn.Name,
					File:     relPath,
				})
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk workspace: %w", err)
	}

	// Build model data
	modelData := systemModelData{
		Functions: functions,
		Endpoints: endpoints,
		Stats: modelStats{
			FileCount:     fileCount,
			FunctionCount: functionCount,
			EndpointCount: len(endpoints),
		},
	}

	modelJSON, err := json.Marshal(modelData)
	if err != nil {
		return fmt.Errorf("failed to serialize model: %w", err)
	}

	// Get commit SHA for the model
	commitSHA := getCommitSHA(ctx, payload.WorkspacePath)

	// Create system model in database
	modelID := uuid.New()
	if w.store != nil {
		if err := w.store.CreateSystemModel(ctx, &db.SystemModel{
			ID:           modelID,
			RepositoryID: payload.RepositoryID,
			CommitSHA:    commitSHA,
			ModelData:    modelJSON,
		}); err != nil {
			log.Warn().Err(err).Msg("failed to persist system model")
		}
	}

	result := jobs.ModelingResult{
		ModelID:       modelID,
		FileCount:     fileCount,
		FunctionCount: functionCount,
		EndpointCount: len(endpoints),
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	// Chain to planning job with pipeline options
	if w.Pipeline() != nil {
		opts := jobs.PipelineJobOptions{
			MaxTests:    payload.MaxTests,
			LLMTier:     payload.LLMTier,
			RunMutation: payload.RunMutation,
			CreatePR:    payload.CreatePR,
		}
		_, err := w.Pipeline().CreatePlanningJob(ctx, job.ID, payload.RepositoryID, result.ModelID, opts)
		if err != nil {
			log.Warn().Err(err).Msg("failed to create planning job")
		}
	}

	return nil
}

// Model data structures for JSON serialization
type systemModelData struct {
	Functions []modelFunction `json:"functions"`
	Endpoints []modelEndpoint `json:"endpoints"`
	Stats     modelStats      `json:"stats"`
}

type modelFunction struct {
	Name      string `json:"name"`
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Exported  bool   `json:"exported"`
}

type modelEndpoint struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Method   string `json:"method,omitempty"`
	Path     string `json:"path,omitempty"`
}

type modelStats struct {
	FileCount     int `json:"file_count"`
	FunctionCount int `json:"function_count"`
	EndpointCount int `json:"endpoint_count"`
}

// isHTTPHandler checks if a function name looks like an HTTP handler
func isHTTPHandler(name string) bool {
	lowerName := strings.ToLower(name)
	return strings.HasPrefix(lowerName, "handle") ||
		strings.HasSuffix(lowerName, "handler") ||
		strings.HasPrefix(lowerName, "serve") ||
		strings.Contains(lowerName, "endpoint")
}

// PlanningWorker creates test generation plans
type PlanningWorker struct {
	*BaseWorker
	store *db.Store
}

func NewPlanningWorker(base *BaseWorker, store *db.Store) *PlanningWorker {
	w := &PlanningWorker{BaseWorker: base, store: store}
	base.handler = w.handleJob
	return w
}

func (w *PlanningWorker) Name() string { return "planning" }

func (w *PlanningWorker) handleJob(ctx context.Context, job *jobs.Job) error {
	var payload jobs.PlanningPayload
	if err := job.GetPayload(&payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	log.Info().
		Str("model_id", payload.ModelID.String()).
		Int("max_tests", payload.MaxTests).
		Msg("creating test plan")

	// Try to load rich SystemModel and use proper planner
	var testPlan *model.TestPlan
	var sysModel *model.SystemModel

	if w.store != nil {
		dbModel, err := w.store.GetSystemModel(ctx, payload.ModelID)
		if err != nil {
			log.Warn().Err(err).Msg("failed to load system model")
		} else if dbModel != nil {
			// Try to unmarshal as rich SystemModel first
			if err := json.Unmarshal(dbModel.ModelData, &sysModel); err == nil && sysModel != nil && len(sysModel.Functions) > 0 {
				// Use the real planner!
				plannerConfig := model.DefaultPlannerConfig()
				if payload.MaxTests > 0 {
					plannerConfig.MaxIntents = payload.MaxTests
				}
				planner := model.NewPlanner(plannerConfig)

				testPlan, err = planner.Plan(sysModel)
				if err != nil {
					log.Warn().Err(err).Msg("failed to create test plan with planner")
					testPlan = nil
				} else {
					stats := testPlan.Stats()
					log.Info().
						Int("total", stats["total"]).
						Int("unit", stats["unit"]).
						Int("api", stats["api"]).
						Int("e2e", stats["e2e"]).
						Int("high_priority", stats["high"]).
						Int("medium_priority", stats["medium"]).
						Int("low_priority", stats["low"]).
						Msg("created test plan with planner")
				}
			}
		}
	}

	// Fall back to simple calculation if planner didn't work
	var result jobs.PlanningResult
	if testPlan != nil {
		result = jobs.PlanningResult{
			PlanID:     uuid.New(),
			TotalTests: testPlan.TotalTests,
			UnitTests:  testPlan.UnitTests,
			APITests:   testPlan.APITests,
			E2ETests:   testPlan.E2ETests,
		}
	} else {
		// Fallback: simple percentage split
		maxTests := payload.MaxTests
		if maxTests == 0 {
			maxTests = 10
		}
		result = jobs.PlanningResult{
			PlanID:     uuid.New(),
			TotalTests: maxTests,
			UnitTests:  int(float64(maxTests) * 0.6),
			APITests:   int(float64(maxTests) * 0.3),
			E2ETests:   int(float64(maxTests) * 0.1),
		}
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	// Chain to generation job with pipeline options
	if w.Pipeline() != nil {
		runID := uuid.New()
		tier := payload.LLMTier
		if tier == 0 {
			tier = 1 // Default to fast tier
		}

		// Create generation_runs record in database (required for foreign key constraint)
		if w.store != nil {
			genRun := &db.GenerationRun{
				ID:           runID,
				RepositoryID: payload.RepositoryID,
				Status:       "pending",
				Config:       []byte(`{}`),
			}
			if err := w.store.CreateGenerationRun(ctx, genRun); err != nil {
				log.Warn().Err(err).Msg("failed to create generation run record")
			} else {
				log.Info().Str("run_id", runID.String()).Msg("created generation run record")
			}
		} else {
			log.Warn().Msg("planning worker has no store, cannot create generation run record")
		}

		opts := jobs.GenerationJobOptions{
			MaxTests:    payload.MaxTests,
			LLMTier:     tier,
			RunMutation: payload.RunMutation,
			CreatePR:    payload.CreatePR,
		}
		_, err := w.Pipeline().CreateGenerationJob(ctx, job.ID, payload.RepositoryID, runID, result.PlanID, opts)
		if err != nil {
			log.Warn().Err(err).Msg("failed to create generation job")
		}
	}

	return nil
}

// GenerationWorker generates tests using LLM
type GenerationWorker struct {
	*BaseWorker
	cfg   *config.Config
	store *db.Store
	gen   *generator.Generator
}

func NewGenerationWorker(base *BaseWorker, cfg *config.Config, store *db.Store, llmRouter *llm.Router) *GenerationWorker {
	var gen *generator.Generator
	if llmRouter != nil {
		gen = generator.NewGenerator(llmRouter)
	}
	w := &GenerationWorker{BaseWorker: base, cfg: cfg, store: store, gen: gen}
	base.handler = w.handleJob
	return w
}

func (w *GenerationWorker) Name() string { return "generation" }

func (w *GenerationWorker) handleJob(ctx context.Context, job *jobs.Job) error {
	var payload jobs.GenerationPayload
	if err := job.GetPayload(&payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	log.Info().
		Str("plan_id", payload.PlanID.String()).
		Str("run_id", payload.GenerationRunID.String()).
		Int("tier", payload.LLMTier).
		Msg("generating tests")

	// Update generation run status to running
	if w.store != nil {
		if err := w.store.UpdateGenerationRunStatus(ctx, payload.GenerationRunID, "running"); err != nil {
			log.Warn().Err(err).Msg("failed to update run status")
		}
	}

	// Check if generator is available
	if w.gen == nil {
		log.Warn().Msg("LLM generator not configured, skipping test generation")
		result := jobs.GenerationResult{
			TestsGenerated: 0,
			TestFilePaths:  []string{},
			FailedIntents:  []string{"LLM not configured"},
		}
		if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
			return fmt.Errorf("failed to complete job: %w", err)
		}
		return nil
	}

	// Get workspace path from parent job chain
	workspacePath := w.getWorkspacePath(ctx, job)
	if workspacePath == "" {
		return fmt.Errorf("could not determine workspace path")
	}

	// Determine LLM tier
	tier := llm.Tier(payload.LLMTier)
	if tier == 0 {
		tier = llm.Tier1 // Default to fast tier
	}

	// Generate tests for source files in workspace
	var testFilePaths []string
	var failedIntents []string
	testsGenerated := 0

	err := filepath.Walk(workspacePath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".py" && ext != ".ts" && ext != ".js" {
			return nil
		}

		// Skip test files
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_test.py") ||
			strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, ".test.js") ||
			strings.HasSuffix(path, ".spec.ts") || strings.HasSuffix(path, ".spec.js") {
			return nil
		}

		log.Debug().Str("file", path).Msg("generating tests for file")

		// Generate tests for this file using IRSpec (structured JSON output)
		tests, genErr := w.gen.GenerateForFile(ctx, path, generator.GenerateOptions{
			Tier:      tier,
			TestType:  dsl.TestTypeUnit,
			MaxTests:  5,        // Limit per file
			UseIRSpec: true,     // Use IRSpec for structured output
		})
		if genErr != nil {
			log.Warn().Err(genErr).Str("file", path).Msg("failed to generate tests")
			failedIntents = append(failedIntents, path)
			return nil
		}

		// Convert generated tests to code and write to files
		for _, test := range tests {
			testPath, writeErr := w.writeTestFile(path, test, workspacePath)
			if writeErr != nil {
				log.Warn().Err(writeErr).Msg("failed to write test file")
				failedIntents = append(failedIntents, test.Function.Name)
				continue
			}
			testFilePaths = append(testFilePaths, testPath)
			testsGenerated++

			// Persist to database
			w.persistGeneratedTest(ctx, payload.GenerationRunID, test, testPath)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk workspace: %w", err)
	}

	// Update generation run status
	if w.store != nil {
		status := "completed"
		if testsGenerated == 0 {
			status = "failed"
		}
		if err := w.store.UpdateGenerationRunStatus(ctx, payload.GenerationRunID, status); err != nil {
			log.Warn().Err(err).Msg("failed to update run status")
		}
	}

	result := jobs.GenerationResult{
		TestsGenerated: testsGenerated,
		TestFilePaths:  testFilePaths,
		FailedIntents:  failedIntents,
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	// Chain to mutation jobs if requested
	if w.Pipeline() != nil && payload.RunMutation && len(result.TestFilePaths) > 0 {
		for _, testPath := range result.TestFilePaths {
			sourcePath := deriveSourcePath(testPath)
			if sourcePath == "" {
				log.Warn().Str("test_path", testPath).Msg("could not derive source path for mutation testing")
				continue
			}
			_, err := w.Pipeline().CreateMutationJob(ctx, job.ID, payload.RepositoryID, payload.GenerationRunID, testPath, sourcePath)
			if err != nil {
				log.Warn().Err(err).Str("test_path", testPath).Msg("failed to create mutation job")
			}
		}
	}

	// Chain to integration job if tests were generated
	if w.Pipeline() != nil && len(result.TestFilePaths) > 0 {
		_, err := w.Pipeline().CreateIntegrationJob(ctx, job.ID, payload.RepositoryID, payload.GenerationRunID, result.TestFilePaths, payload.CreatePR)
		if err != nil {
			log.Warn().Err(err).Msg("failed to create integration job")
		}
	}

	return nil
}

// deriveSourcePath converts a test file path back to its source file path
func deriveSourcePath(testPath string) string {
	dir := filepath.Dir(testPath)
	base := filepath.Base(testPath)

	var sourceName string
	switch {
	case strings.HasSuffix(base, "_test.go"):
		// Go: foo_test.go -> foo.go
		sourceName = strings.TrimSuffix(base, "_test.go") + ".go"
	case strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py"):
		// Python: test_foo.py -> foo.py
		sourceName = strings.TrimPrefix(base, "test_")
	case strings.HasSuffix(base, ".test.ts"):
		// TypeScript: foo.test.ts -> foo.ts
		sourceName = strings.TrimSuffix(base, ".test.ts") + ".ts"
	case strings.HasSuffix(base, ".test.js"):
		// JavaScript: foo.test.js -> foo.js
		sourceName = strings.TrimSuffix(base, ".test.js") + ".js"
	case strings.HasSuffix(base, ".spec.ts"):
		// TypeScript spec: foo.spec.ts -> foo.ts
		sourceName = strings.TrimSuffix(base, ".spec.ts") + ".ts"
	case strings.HasSuffix(base, ".spec.js"):
		// JavaScript spec: foo.spec.js -> foo.js
		sourceName = strings.TrimSuffix(base, ".spec.js") + ".js"
	default:
		return ""
	}

	return filepath.Join(dir, sourceName)
}

// getWorkspacePath retrieves workspace path from the job chain
func (w *GenerationWorker) getWorkspacePath(ctx context.Context, job *jobs.Job) string {
	// Walk up the parent chain to find ingestion result
	current := job
	for current.ParentJobID != nil {
		parent, err := w.Repository().GetByID(ctx, *current.ParentJobID)
		if err != nil || parent == nil {
			break
		}

		if parent.Type == jobs.JobTypeIngestion {
			var result jobs.IngestionResult
			if err := parent.GetResult(&result); err == nil {
				return result.WorkspacePath
			}
		}
		current = parent
	}
	return ""
}

// writeTestFile writes generated test to a file
func (w *GenerationWorker) writeTestFile(sourcePath string, test generator.GeneratedTest, workspacePath string) (string, error) {
	// Determine test file path based on source file
	dir := filepath.Dir(sourcePath)
	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	var testFileName string
	switch ext {
	case ".go":
		testFileName = name + "_test.go"
	case ".py":
		testFileName = "test_" + name + ".py"
	case ".ts":
		testFileName = name + ".test.ts"
	case ".js":
		testFileName = name + ".test.js"
	default:
		testFileName = name + "_test" + ext
	}

	testPath := filepath.Join(dir, testFileName)

	// Get appropriate adapter for code generation
	var testCode string
	var err error
	switch ext {
	case ".go":
		// Prefer TestSpec-based generation for better assertions
		if len(test.TestSpecs) > 0 {
			specAdapter := adapters.NewGoSpecAdapter()
			testCode, err = specAdapter.GenerateFromSpecs(test.TestSpecs, sourcePath)
			if err != nil {
				log.Warn().Err(err).Msg("TestSpec generation failed, falling back to DSL")
				// Fall back to DSL-based generation
				adapter := adapters.NewGoAdapter()
				testCode, err = adapter.Generate(test.DSL)
			} else {
				log.Info().Int("specs", len(test.TestSpecs)).Msg("generated test from TestSpecs with proper assertions")
			}
		} else {
			adapter := adapters.NewGoAdapter()
			testCode, err = adapter.Generate(test.DSL)
		}
	case ".py":
		adapter := adapters.NewPytestAdapter()
		testCode, err = adapter.Generate(test.DSL)
	case ".ts", ".js":
		adapter := adapters.NewJestAdapter()
		testCode, err = adapter.Generate(test.DSL)
	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}

	if err != nil {
		return "", fmt.Errorf("failed to generate test code: %w", err)
	}

	// Write test file
	if err := os.WriteFile(testPath, []byte(testCode), 0644); err != nil {
		return "", fmt.Errorf("failed to write test file: %w", err)
	}

	log.Info().Str("path", testPath).Msg("wrote test file")
	return testPath, nil
}

// persistGeneratedTest saves the generated test to the database
func (w *GenerationWorker) persistGeneratedTest(ctx context.Context, runID uuid.UUID, test generator.GeneratedTest, testPath string) {
	if w.store == nil {
		return
	}

	dslJSON, err := json.Marshal(test.DSL)
	if err != nil {
		log.Warn().Err(err).Msg("failed to marshal DSL")
		return
	}

	framework := detectFramework(testPath)
	dbTest := &db.GeneratedTest{
		RunID:          runID,
		Name:           test.DSL.Name,
		Type:           string(test.DSL.Type),
		TargetFile:     test.FileName,
		TargetFunction: &test.Function.Name,
		DSL:            dslJSON,
		Framework:      &framework,
		Status:         "pending",
	}

	// Store IRSpec JSON in metadata for traceability
	if test.RawYAML != "" {
		metadata := map[string]interface{}{
			"irspec":      json.RawMessage(test.RawYAML),
			"test_specs":  test.TestSpecs,
			"irspec_mode": true,
		}
		metadataJSON, err := json.Marshal(metadata)
		if err == nil {
			rawMetadata := json.RawMessage(metadataJSON)
			dbTest.Metadata = &rawMetadata
		}
	}

	if err := w.store.CreateGeneratedTest(ctx, dbTest); err != nil {
		log.Warn().Err(err).Msg("failed to persist generated test")
	}
}

// detectFramework returns the test framework based on file extension
func detectFramework(testPath string) string {
	switch {
	case strings.HasSuffix(testPath, "_test.go"):
		return "go"
	case strings.HasSuffix(testPath, ".py"):
		return "pytest"
	case strings.HasSuffix(testPath, ".ts"), strings.HasSuffix(testPath, ".js"):
		return "jest"
	default:
		return "unknown"
	}
}

// MutationWorker runs mutation testing on generated tests
type MutationWorker struct {
	*BaseWorker
	store  *db.Store
	cfg    *config.Config
	runner *mutation.Runner
}

func NewMutationWorker(base *BaseWorker, store *db.Store, cfg *config.Config) *MutationWorker {
	// Create mutation runner with available tools
	runner := mutation.NewRunner(
		mutation.NewGoMutestingTool(),
		mutation.NewSimpleMutationTool(), // Fallback
	)

	w := &MutationWorker{
		BaseWorker: base,
		store:      store,
		cfg:        cfg,
		runner:     runner,
	}
	base.handler = w.handleJob
	return w
}

func (w *MutationWorker) Name() string { return "mutation" }

func (w *MutationWorker) handleJob(ctx context.Context, job *jobs.Job) error {
	var payload jobs.MutationPayload
	if err := job.GetPayload(&payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	log.Info().
		Str("test_file", payload.TestFilePath).
		Str("source_file", payload.SourceFilePath).
		Str("repo_id", payload.RepositoryID.String()).
		Msg("running mutation testing")

	// Validate files exist
	if _, err := os.Stat(payload.SourceFilePath); os.IsNotExist(err) {
		return fmt.Errorf("source file not found: %s", payload.SourceFilePath)
	}
	if _, err := os.Stat(payload.TestFilePath); os.IsNotExist(err) {
		return fmt.Errorf("test file not found: %s", payload.TestFilePath)
	}

	// Configure mutation testing
	mutationCfg := mutation.DefaultConfig()

	// Run mutation testing
	mutResult, err := w.runner.Run(ctx, payload.SourceFilePath, payload.TestFilePath, mutationCfg)
	if err != nil {
		log.Warn().Err(err).Msg("mutation testing failed")
		// Complete job with error result rather than failing
		result := jobs.MutationResult{
			MutantsTotal:  0,
			MutantsKilled: 0,
			MutantsLived:  0,
			MutationScore: 0,
		}
		return w.Repository().Complete(ctx, job.ID, result)
	}

	// Convert to job result
	result := jobs.MutationResult{
		MutantsTotal:  mutResult.Total,
		MutantsKilled: mutResult.Killed,
		MutantsLived:  mutResult.Survived,
		MutationScore: mutResult.Score,
	}

	// Generate report file if there are results
	if mutResult.Total > 0 {
		reportPath := w.generateReport(payload.SourceFilePath, mutResult)
		if reportPath != "" {
			result.ReportFilePath = reportPath
		}
	}

	log.Info().
		Int("total", result.MutantsTotal).
		Int("killed", result.MutantsKilled).
		Int("lived", result.MutantsLived).
		Float64("score", result.MutationScore).
		Str("quality", mutResult.Quality()).
		Msg("mutation testing completed")

	// Update test mutation score in database if available
	w.updateTestMutationScore(ctx, payload, result.MutationScore)

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	return nil
}

// generateReport creates a mutation testing report file
func (w *MutationWorker) generateReport(sourceFile string, result *mutation.Result) string {
	dir := filepath.Dir(sourceFile)
	base := filepath.Base(sourceFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	reportPath := filepath.Join(dir, name+"_mutation_report.json")

	reportData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Warn().Err(err).Msg("failed to marshal mutation report")
		return ""
	}

	if err := os.WriteFile(reportPath, reportData, 0644); err != nil {
		log.Warn().Err(err).Msg("failed to write mutation report")
		return ""
	}

	log.Debug().Str("path", reportPath).Msg("wrote mutation report")
	return reportPath
}

// updateTestMutationScore updates the mutation score for the test in the database
func (w *MutationWorker) updateTestMutationScore(ctx context.Context, payload jobs.MutationPayload, score float64) {
	if w.store == nil {
		return
	}

	// Find tests associated with this run and source file
	tests, err := w.store.ListTestsByRun(ctx, payload.GenerationRunID)
	if err != nil {
		log.Warn().Err(err).Msg("failed to list tests for mutation score update")
		return
	}

	// Update mutation score for matching tests
	for _, test := range tests {
		if test.TargetFile == payload.SourceFilePath ||
			(test.TargetFunction != nil && strings.Contains(payload.TestFilePath, *test.TargetFunction)) {
			// Update test with mutation score
			if err := w.store.UpdateTestMutationScore(ctx, test.ID, score); err != nil {
				log.Warn().Err(err).Str("test_id", test.ID.String()).Msg("failed to update test mutation score")
			}
		}
	}
}

// IntegrationWorker integrates generated tests into the repository
type IntegrationWorker struct {
	*BaseWorker
	store *db.Store
}

func NewIntegrationWorker(base *BaseWorker, store *db.Store) *IntegrationWorker {
	w := &IntegrationWorker{BaseWorker: base, store: store}
	base.handler = w.handleJob
	return w
}

func (w *IntegrationWorker) Name() string { return "integration" }

func (w *IntegrationWorker) handleJob(ctx context.Context, job *jobs.Job) error {
	var payload jobs.IntegrationPayload
	if err := job.GetPayload(&payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	log.Info().
		Strs("test_files", payload.TestFilePaths).
		Bool("create_pr", payload.CreatePR).
		Str("target_branch", payload.TargetBranch).
		Msg("integrating tests")

	// Get workspace path from job chain
	workspacePath := w.getWorkspacePath(ctx, job)
	if workspacePath == "" {
		return fmt.Errorf("could not determine workspace path")
	}

	// Validate test files exist
	var validFiles []string
	for _, testPath := range payload.TestFilePaths {
		if _, err := os.Stat(testPath); err == nil {
			validFiles = append(validFiles, testPath)
		} else {
			log.Warn().Str("path", testPath).Msg("test file not found, skipping")
		}
	}

	if len(validFiles) == 0 {
		log.Warn().Msg("no valid test files to integrate")
		result := jobs.IntegrationResult{
			FilesIntegrated: 0,
		}
		return w.Repository().Complete(ctx, job.ID, result)
	}

	// Run tests to verify they compile/pass
	testsPassed, testOutput := w.runTests(ctx, workspacePath, validFiles)
	if !testsPassed {
		log.Warn().Str("output", testOutput).Msg("some tests failed verification")
		// Continue with integration but mark tests as needing review
	}

	// Update test statuses in database
	w.updateTestStatuses(ctx, payload.GenerationRunID, testsPassed)

	result := jobs.IntegrationResult{
		FilesIntegrated: len(validFiles),
	}

	// Create branch and prepare for PR if requested
	if payload.CreatePR {
		branchName := fmt.Sprintf("qtest/tests-%s", job.ID.String()[:8])
		result.BranchName = branchName

		if err := w.createBranch(ctx, workspacePath, branchName, validFiles); err != nil {
			log.Warn().Err(err).Msg("failed to create branch")
		} else {
			log.Info().Str("branch", branchName).Msg("created branch with test files")
		}
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	return nil
}

// getWorkspacePath retrieves workspace path from the job chain
func (w *IntegrationWorker) getWorkspacePath(ctx context.Context, job *jobs.Job) string {
	current := job
	for current.ParentJobID != nil {
		parent, err := w.Repository().GetByID(ctx, *current.ParentJobID)
		if err != nil || parent == nil {
			break
		}

		if parent.Type == jobs.JobTypeIngestion {
			var result jobs.IngestionResult
			if err := parent.GetResult(&result); err == nil {
				return result.WorkspacePath
			}
		}
		current = parent
	}
	return ""
}

// runTests runs the generated tests to verify they work
func (w *IntegrationWorker) runTests(ctx context.Context, workspacePath string, testFiles []string) (bool, string) {
	// Detect language from test files
	if len(testFiles) == 0 {
		return true, ""
	}

	ext := filepath.Ext(testFiles[0])
	var cmd *exec.Cmd

	switch ext {
	case ".go":
		// Run go test on the workspace
		cmd = exec.CommandContext(ctx, "go", "test", "-v", "./...")
		cmd.Dir = workspacePath
	case ".py":
		// Run pytest
		cmd = exec.CommandContext(ctx, "python", "-m", "pytest", "-v")
		cmd.Dir = workspacePath
	case ".ts", ".js":
		// Run npm test (assumes package.json exists)
		cmd = exec.CommandContext(ctx, "npm", "test")
		cmd.Dir = workspacePath
	default:
		return true, "unknown test framework"
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, string(output)
	}

	return true, string(output)
}

// updateTestStatuses updates the status of generated tests in the database
func (w *IntegrationWorker) updateTestStatuses(ctx context.Context, runID uuid.UUID, passed bool) {
	if w.store == nil {
		return
	}

	status := "accepted"
	if !passed {
		status = "rejected"
	}

	tests, err := w.store.ListTestsByRun(ctx, runID)
	if err != nil {
		log.Warn().Err(err).Msg("failed to list tests for status update")
		return
	}

	for _, test := range tests {
		var reason *string
		if !passed {
			r := "test verification failed"
			reason = &r
		}
		if err := w.store.UpdateTestStatus(ctx, test.ID, status, reason); err != nil {
			log.Warn().Err(err).Str("test_id", test.ID.String()).Msg("failed to update test status")
		}
	}
}

// createBranch creates a git branch with the test files
func (w *IntegrationWorker) createBranch(ctx context.Context, workspacePath, branchName string, testFiles []string) error {
	// Create and checkout new branch
	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", branchName)
	cmd.Dir = workspacePath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch: %s: %w", string(output), err)
	}

	// Add test files
	for _, testFile := range testFiles {
		relPath, err := filepath.Rel(workspacePath, testFile)
		if err != nil {
			relPath = testFile
		}

		cmd := exec.CommandContext(ctx, "git", "add", relPath)
		cmd.Dir = workspacePath
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Warn().Str("file", relPath).Str("output", string(output)).Msg("failed to add file")
		}
	}

	// Commit the changes
	commitMsg := fmt.Sprintf("Add generated tests\n\nGenerated by QTest")
	cmd = exec.CommandContext(ctx, "git", "commit", "-m", commitMsg)
	cmd.Dir = workspacePath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to commit: %s: %w", string(output), err)
	}

	return nil
}
