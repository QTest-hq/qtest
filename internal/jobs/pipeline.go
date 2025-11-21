// Package jobs provides pipeline orchestration for test generation workflows
package jobs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	qtestnats "github.com/QTest-hq/qtest/internal/nats"
)

// Pipeline orchestrates the test generation workflow
type Pipeline struct {
	repo *Repository
	nats *qtestnats.Client
}

// NewPipeline creates a new pipeline manager
func NewPipeline(repo *Repository, nats *qtestnats.Client) *Pipeline {
	return &Pipeline{
		repo: repo,
		nats: nats,
	}
}

// StartIngestion starts the ingestion pipeline for a repository
func (p *Pipeline) StartIngestion(ctx context.Context, payload IngestionPayload) (*Job, error) {
	job, err := NewJob(JobTypeIngestion, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	if err := p.repo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to persist job: %w", err)
	}

	if err := p.publishJob(ctx, job); err != nil {
		log.Error().Err(err).Str("job_id", job.ID.String()).Msg("failed to publish job")
		// Job is in DB, worker can poll for it
	}

	log.Info().
		Str("job_id", job.ID.String()).
		Str("repo_url", payload.RepositoryURL).
		Msg("started ingestion pipeline")

	return job, nil
}

// StartFullPipeline starts the complete test generation pipeline
// This creates the initial ingestion job; subsequent jobs are created by workers
func (p *Pipeline) StartFullPipeline(ctx context.Context, repoURL string, options PipelineOptions) (*Job, error) {
	payload := IngestionPayload{
		RepositoryURL: repoURL,
		Branch:        options.Branch,
	}

	job, err := p.StartIngestion(ctx, payload)
	if err != nil {
		return nil, err
	}

	// Store pipeline options in job metadata for workers to use
	// Workers will read these when creating subsequent jobs
	log.Info().
		Str("job_id", job.ID.String()).
		Str("repo_url", repoURL).
		Int("max_tests", options.MaxTests).
		Int("llm_tier", options.LLMTier).
		Msg("started full pipeline")

	return job, nil
}

// PipelineOptions configures pipeline execution
type PipelineOptions struct {
	Branch      string   // Git branch to use
	MaxTests    int      // Maximum tests to generate
	LLMTier     int      // LLM tier (1=fast, 2=balanced, 3=thorough)
	TestLevels  []string // "unit", "api", "e2e"
	RunMutation bool     // Whether to run mutation testing after generation
	CreatePR    bool     // Whether to create a PR at the end
}

// ChainJob creates a child job linked to a parent
func (p *Pipeline) ChainJob(ctx context.Context, parentID uuid.UUID, jobType JobType, payload interface{}) (*Job, error) {
	job, err := NewJob(jobType, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	job.ParentJobID = &parentID

	// Inherit repository_id from parent if not set
	parent, err := p.repo.GetByID(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent job: %w", err)
	}
	if parent != nil && parent.RepositoryID != nil {
		job.RepositoryID = parent.RepositoryID
	}
	if parent != nil && parent.GenerationRunID != nil {
		job.GenerationRunID = parent.GenerationRunID
	}

	if err := p.repo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to persist job: %w", err)
	}

	if err := p.publishJob(ctx, job); err != nil {
		log.Error().Err(err).Str("job_id", job.ID.String()).Msg("failed to publish job")
	}

	log.Debug().
		Str("job_id", job.ID.String()).
		Str("parent_id", parentID.String()).
		Str("type", string(jobType)).
		Msg("created chained job")

	return job, nil
}

// CreateModelingJob creates a modeling job after ingestion completes
func (p *Pipeline) CreateModelingJob(ctx context.Context, parentID uuid.UUID, repoID uuid.UUID, workspacePath string) (*Job, error) {
	payload := ModelingPayload{
		RepositoryID:  repoID,
		WorkspacePath: workspacePath,
	}

	job, err := p.ChainJob(ctx, parentID, JobTypeModeling, payload)
	if err != nil {
		return nil, err
	}
	job.RepositoryID = &repoID

	return job, nil
}

// CreatePlanningJob creates a planning job after modeling completes
func (p *Pipeline) CreatePlanningJob(ctx context.Context, parentID uuid.UUID, repoID, modelID uuid.UUID, maxTests int) (*Job, error) {
	payload := PlanningPayload{
		RepositoryID: repoID,
		ModelID:      modelID,
		MaxTests:     maxTests,
	}

	return p.ChainJob(ctx, parentID, JobTypePlanning, payload)
}

// GenerationJobOptions configures a generation job
type GenerationJobOptions struct {
	Tier        int  // LLM tier
	RunMutation bool // Whether to run mutation testing after generation
	CreatePR    bool // Whether to create a PR at the end
}

// CreateGenerationJob creates a generation job after planning completes
func (p *Pipeline) CreateGenerationJob(ctx context.Context, parentID uuid.UUID, repoID, runID, planID uuid.UUID, opts GenerationJobOptions) (*Job, error) {
	payload := GenerationPayload{
		RepositoryID:    repoID,
		GenerationRunID: runID,
		PlanID:          planID,
		LLMTier:         opts.Tier,
		RunMutation:     opts.RunMutation,
		CreatePR:        opts.CreatePR,
	}

	job, err := p.ChainJob(ctx, parentID, JobTypeGeneration, payload)
	if err != nil {
		return nil, err
	}
	job.GenerationRunID = &runID

	return job, nil
}

// CreateIntegrationJob creates an integration job after generation completes
func (p *Pipeline) CreateIntegrationJob(ctx context.Context, parentID uuid.UUID, repoID, runID uuid.UUID, testPaths []string, createPR bool) (*Job, error) {
	payload := IntegrationPayload{
		RepositoryID:    repoID,
		GenerationRunID: runID,
		TestFilePaths:   testPaths,
		CreatePR:        createPR,
	}

	return p.ChainJob(ctx, parentID, JobTypeIntegration, payload)
}

// CreateMutationJob creates a mutation testing job for a specific test/source pair
func (p *Pipeline) CreateMutationJob(ctx context.Context, parentID uuid.UUID, repoID, runID uuid.UUID, testFilePath, sourceFilePath string) (*Job, error) {
	payload := MutationPayload{
		RepositoryID:    repoID,
		GenerationRunID: runID,
		TestFilePath:    testFilePath,
		SourceFilePath:  sourceFilePath,
	}

	job, err := p.ChainJob(ctx, parentID, JobTypeMutation, payload)
	if err != nil {
		return nil, err
	}
	job.GenerationRunID = &runID

	return job, nil
}

// publishJob publishes a job notification to NATS
func (p *Pipeline) publishJob(ctx context.Context, job *Job) error {
	if p.nats == nil {
		return nil // NATS not configured, workers will poll DB
	}

	msg := &JobMessage{
		JobID:    job.ID,
		Type:     job.Type,
		Priority: job.Priority,
	}

	data, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	subject := qtestnats.SubjectForJobType(string(job.Type))
	if subject == "" {
		return fmt.Errorf("unknown job type: %s", job.Type)
	}

	_, err = p.nats.Publish(ctx, subject, data)
	return err
}

// GetJobStatus returns the current status of a job and its children
func (p *Pipeline) GetJobStatus(ctx context.Context, jobID uuid.UUID) (*JobStatusReport, error) {
	job, err := p.repo.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("job not found")
	}

	children, err := p.repo.GetChildJobs(ctx, jobID)
	if err != nil {
		return nil, err
	}

	return &JobStatusReport{
		Job:      job,
		Children: children,
	}, nil
}

// JobStatusReport contains a job and its child jobs
type JobStatusReport struct {
	Job      *Job   `json:"job"`
	Children []*Job `json:"children,omitempty"`
}

// RetryFailedJobs requeues all jobs in retrying status
func (p *Pipeline) RetryFailedJobs(ctx context.Context) (int, error) {
	jobs, err := p.repo.ListByStatus(ctx, StatusRetrying, 100)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, job := range jobs {
		if err := p.repo.Retry(ctx, job.ID); err != nil {
			log.Warn().Err(err).Str("job_id", job.ID.String()).Msg("failed to retry job")
			continue
		}

		// Republish to NATS
		job.Status = StatusPending
		if err := p.publishJob(ctx, job); err != nil {
			log.Warn().Err(err).Str("job_id", job.ID.String()).Msg("failed to republish job")
		}

		count++
	}

	return count, nil
}
