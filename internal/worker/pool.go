package worker

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/jobs"
	qtestnats "github.com/QTest-hq/qtest/internal/nats"
)

// WorkerType represents the type of worker
type WorkerType string

const (
	WorkerIngestion   WorkerType = "ingestion"
	WorkerModeling    WorkerType = "modeling"
	WorkerPlanning    WorkerType = "planning"
	WorkerGeneration  WorkerType = "generation"
	WorkerMutation    WorkerType = "mutation"
	WorkerIntegration WorkerType = "integration"
	WorkerAll         WorkerType = "all"
)

// Pool manages a pool of workers
type Pool struct {
	cfg        *config.Config
	workerType WorkerType
	workers    []Worker
	nats       *qtestnats.Client
	repo       *jobs.Repository
	pipeline   *jobs.Pipeline
	db         *sql.DB
}

// Worker is the interface all workers must implement
type Worker interface {
	Name() string
	Run(ctx context.Context) error
}

// PoolConfig configures the worker pool
type PoolConfig struct {
	Config     *config.Config
	WorkerType string
	DB         *sql.DB
	NATS       *qtestnats.Client
}

// NewPool creates a new worker pool
func NewPool(cfg PoolConfig) (*Pool, error) {
	p := &Pool{
		cfg:        cfg.Config,
		workerType: WorkerType(cfg.WorkerType),
		workers:    make([]Worker, 0),
		db:         cfg.DB,
		nats:       cfg.NATS,
	}

	// Initialize job repository if DB is available
	if cfg.DB != nil {
		p.repo = jobs.NewRepository(cfg.DB)
		p.pipeline = jobs.NewPipeline(p.repo, cfg.NATS)
	}

	if err := p.initWorkers(); err != nil {
		return nil, fmt.Errorf("failed to initialize workers: %w", err)
	}

	return p, nil
}

func (p *Pool) initWorkers() error {
	switch p.workerType {
	case WorkerAll:
		// Add all worker types
		p.addWorker(jobs.JobTypeIngestion)
		p.addWorker(jobs.JobTypeModeling)
		p.addWorker(jobs.JobTypePlanning)
		p.addWorker(jobs.JobTypeGeneration)
		p.addWorker(jobs.JobTypeMutation)
		p.addWorker(jobs.JobTypeIntegration)
	case WorkerIngestion:
		p.addWorker(jobs.JobTypeIngestion)
	case WorkerModeling:
		p.addWorker(jobs.JobTypeModeling)
	case WorkerPlanning:
		p.addWorker(jobs.JobTypePlanning)
	case WorkerGeneration:
		p.addWorker(jobs.JobTypeGeneration)
	case WorkerMutation:
		p.addWorker(jobs.JobTypeMutation)
	case WorkerIntegration:
		p.addWorker(jobs.JobTypeIntegration)
	default:
		return fmt.Errorf("unknown worker type: %s", p.workerType)
	}

	return nil
}

func (p *Pool) addWorker(jobType jobs.JobType) {
	baseCfg := BaseWorkerConfig{
		Config:     p.cfg,
		JobType:    jobType,
		Repository: p.repo,
		NATS:       p.nats,
		Pipeline:   p.pipeline,
	}

	base := NewBaseWorker(baseCfg)

	var worker Worker
	switch jobType {
	case jobs.JobTypeIngestion:
		worker = NewIngestionWorker(base)
	case jobs.JobTypeModeling:
		worker = NewModelingWorker(base)
	case jobs.JobTypePlanning:
		worker = NewPlanningWorker(base)
	case jobs.JobTypeGeneration:
		worker = NewGenerationWorker(base, p.cfg)
	case jobs.JobTypeMutation:
		worker = NewMutationWorker(base)
	case jobs.JobTypeIntegration:
		worker = NewIntegrationWorker(base)
	}

	if worker != nil {
		p.workers = append(p.workers, worker)
	}
}

// Run starts all workers and blocks until context is cancelled
func (p *Pool) Run(ctx context.Context) error {
	if len(p.workers) == 0 {
		return fmt.Errorf("no workers configured")
	}

	// Set up NATS streams if connected
	if p.nats != nil && p.nats.IsConnected() {
		if err := p.nats.SetupStreams(ctx); err != nil {
			log.Warn().Err(err).Msg("failed to setup NATS streams, workers will poll DB")
		} else {
			log.Info().Msg("NATS streams configured")
		}
	}

	errCh := make(chan error, len(p.workers))

	// Start all workers
	for _, w := range p.workers {
		go func(worker Worker) {
			log.Info().Str("worker", worker.Name()).Msg("starting worker")
			if err := worker.Run(ctx); err != nil {
				errCh <- fmt.Errorf("worker %s failed: %w", worker.Name(), err)
			}
		}(w)
	}

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		log.Info().Msg("context cancelled, stopping workers")
		return nil
	case err := <-errCh:
		return err
	}
}

// Pipeline returns the job pipeline manager
func (p *Pool) Pipeline() *jobs.Pipeline {
	return p.pipeline
}

// Repository returns the job repository
func (p *Pool) Repository() *jobs.Repository {
	return p.repo
}

// NATS returns the NATS client
func (p *Pool) NATS() *qtestnats.Client {
	return p.nats
}
