package worker

import (
	"context"
	"fmt"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/rs/zerolog/log"
)

// WorkerType represents the type of worker
type WorkerType string

const (
	WorkerIngestion  WorkerType = "ingestion"
	WorkerModeling   WorkerType = "modeling"
	WorkerPlanning   WorkerType = "planning"
	WorkerGeneration WorkerType = "generation"
	WorkerMutation   WorkerType = "mutation"
	WorkerAll        WorkerType = "all"
)

// Pool manages a pool of workers
type Pool struct {
	cfg        *config.Config
	workerType WorkerType
	workers    []Worker
}

// Worker is the interface all workers must implement
type Worker interface {
	Name() string
	Run(ctx context.Context) error
}

// NewPool creates a new worker pool
func NewPool(cfg *config.Config, workerType string) (*Pool, error) {
	p := &Pool{
		cfg:        cfg,
		workerType: WorkerType(workerType),
		workers:    make([]Worker, 0),
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
		p.workers = append(p.workers, NewIngestionWorker(p.cfg))
		p.workers = append(p.workers, NewModelingWorker(p.cfg))
		p.workers = append(p.workers, NewPlanningWorker(p.cfg))
		p.workers = append(p.workers, NewGenerationWorker(p.cfg))
		p.workers = append(p.workers, NewMutationWorker(p.cfg))
	case WorkerIngestion:
		p.workers = append(p.workers, NewIngestionWorker(p.cfg))
	case WorkerModeling:
		p.workers = append(p.workers, NewModelingWorker(p.cfg))
	case WorkerPlanning:
		p.workers = append(p.workers, NewPlanningWorker(p.cfg))
	case WorkerGeneration:
		p.workers = append(p.workers, NewGenerationWorker(p.cfg))
	case WorkerMutation:
		p.workers = append(p.workers, NewMutationWorker(p.cfg))
	default:
		return fmt.Errorf("unknown worker type: %s", p.workerType)
	}

	return nil
}

// Run starts all workers and blocks until context is cancelled
func (p *Pool) Run(ctx context.Context) error {
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
