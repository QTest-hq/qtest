package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/jobs"
	"github.com/QTest-hq/qtest/internal/parser"
)

// IngestionWorker handles repository cloning and initial processing
type IngestionWorker struct {
	*BaseWorker
}

func NewIngestionWorker(base *BaseWorker) *IngestionWorker {
	w := &IngestionWorker{BaseWorker: base}
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

	// Clone repository to workspace
	workspacePath := filepath.Join(os.TempDir(), "qtest", job.ID.String())
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
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
		return fmt.Errorf("git clone failed: %s: %w", string(output), err)
	}

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

	// Create result
	result := jobs.IngestionResult{
		RepositoryID:  uuid.New(), // In real impl, would come from DB
		WorkspacePath: workspacePath,
		FileCount:     fileCount,
		Language:      language,
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	// Chain to modeling job
	if w.Pipeline() != nil {
		_, err := w.Pipeline().CreateModelingJob(ctx, job.ID, result.RepositoryID, workspacePath)
		if err != nil {
			log.Warn().Err(err).Msg("failed to create modeling job")
		}
	}

	return nil
}

// ModelingWorker builds system models from parsed code
type ModelingWorker struct {
	*BaseWorker
}

func NewModelingWorker(base *BaseWorker) *ModelingWorker {
	w := &ModelingWorker{BaseWorker: base}
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
		Msg("modeling repository")

	// Parse all source files
	p := parser.NewParser()
	var functionCount, fileCount int

	err := filepath.Walk(payload.WorkspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
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

		functionCount += len(result.Functions)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk workspace: %w", err)
	}

	result := jobs.ModelingResult{
		ModelID:       uuid.New(),
		FileCount:     fileCount,
		FunctionCount: functionCount,
		EndpointCount: 0, // Would be populated by API framework analysis
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	// Chain to planning job
	if w.Pipeline() != nil {
		_, err := w.Pipeline().CreatePlanningJob(ctx, job.ID, payload.RepositoryID, result.ModelID, 0)
		if err != nil {
			log.Warn().Err(err).Msg("failed to create planning job")
		}
	}

	return nil
}

// PlanningWorker creates test generation plans
type PlanningWorker struct {
	*BaseWorker
}

func NewPlanningWorker(base *BaseWorker) *PlanningWorker {
	w := &PlanningWorker{BaseWorker: base}
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

	// In real implementation, would:
	// 1. Load model from DB
	// 2. Analyze coverage gaps
	// 3. Prioritize test intents
	// 4. Create TestPlan

	result := jobs.PlanningResult{
		PlanID:     uuid.New(),
		TotalTests: payload.MaxTests,
		UnitTests:  int(float64(payload.MaxTests) * 0.6),
		APITests:   int(float64(payload.MaxTests) * 0.3),
		E2ETests:   int(float64(payload.MaxTests) * 0.1),
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	// Chain to generation job
	if w.Pipeline() != nil {
		runID := uuid.New()
		_, err := w.Pipeline().CreateGenerationJob(ctx, job.ID, payload.RepositoryID, runID, result.PlanID, 1)
		if err != nil {
			log.Warn().Err(err).Msg("failed to create generation job")
		}
	}

	return nil
}

// GenerationWorker generates tests using LLM
type GenerationWorker struct {
	*BaseWorker
	cfg *config.Config
}

func NewGenerationWorker(base *BaseWorker, cfg *config.Config) *GenerationWorker {
	w := &GenerationWorker{BaseWorker: base, cfg: cfg}
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
		Int("tier", payload.LLMTier).
		Msg("generating tests")

	// In real implementation, would:
	// 1. Load plan and specs from DB
	// 2. For each intent, call LLM to generate test
	// 3. Convert DSL to target language
	// 4. Write test files

	result := jobs.GenerationResult{
		TestsGenerated: 10, // Placeholder
		TestFilePaths:  []string{},
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	// Chain to integration job if tests were generated
	if w.Pipeline() != nil && len(result.TestFilePaths) > 0 {
		_, err := w.Pipeline().CreateIntegrationJob(ctx, job.ID, payload.RepositoryID, payload.GenerationRunID, result.TestFilePaths, false)
		if err != nil {
			log.Warn().Err(err).Msg("failed to create integration job")
		}
	}

	return nil
}

// MutationWorker runs mutation testing on generated tests
type MutationWorker struct {
	*BaseWorker
}

func NewMutationWorker(base *BaseWorker) *MutationWorker {
	w := &MutationWorker{BaseWorker: base}
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
		Msg("running mutation testing")

	// In real implementation, would:
	// 1. Generate mutants for source file
	// 2. Run test file against each mutant
	// 3. Calculate mutation score

	result := jobs.MutationResult{
		MutantsTotal:  100,
		MutantsKilled: 85,
		MutantsLived:  15,
		MutationScore: 0.85,
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	return nil
}

// IntegrationWorker integrates generated tests into the repository
type IntegrationWorker struct {
	*BaseWorker
}

func NewIntegrationWorker(base *BaseWorker) *IntegrationWorker {
	w := &IntegrationWorker{BaseWorker: base}
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
		Msg("integrating tests")

	// In real implementation, would:
	// 1. Copy test files to repository
	// 2. Run tests to verify they pass
	// 3. Create branch and PR if requested

	result := jobs.IntegrationResult{
		FilesIntegrated: len(payload.TestFilePaths),
	}

	if payload.CreatePR {
		result.BranchName = fmt.Sprintf("qtest/tests-%s", job.ID.String()[:8])
		// Would create actual PR here
	}

	if err := w.Repository().Complete(ctx, job.ID, result); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	return nil
}
