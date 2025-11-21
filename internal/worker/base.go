// Package worker provides base worker functionality with NATS integration
package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/jobs"
	qtestnats "github.com/QTest-hq/qtest/internal/nats"
)

// BaseWorker provides common functionality for all workers
type BaseWorker struct {
	cfg        *config.Config
	workerID   string
	jobType    jobs.JobType
	repo       *jobs.Repository
	nats       *qtestnats.Client
	pipeline   *jobs.Pipeline
	consumer   jetstream.Consumer
	handler    JobHandler
	pollPeriod time.Duration
	lockTime   time.Duration
}

// JobHandler is the function type for processing jobs
type JobHandler func(ctx context.Context, job *jobs.Job) error

// BaseWorkerConfig configures a base worker
type BaseWorkerConfig struct {
	Config     *config.Config
	WorkerID   string
	JobType    jobs.JobType
	Repository *jobs.Repository
	NATS       *qtestnats.Client
	Pipeline   *jobs.Pipeline
	Handler    JobHandler
}

// NewBaseWorker creates a new base worker
func NewBaseWorker(cfg BaseWorkerConfig) *BaseWorker {
	workerID := cfg.WorkerID
	if workerID == "" {
		workerID = fmt.Sprintf("%s-%s", cfg.JobType, uuid.New().String()[:8])
	}

	return &BaseWorker{
		cfg:        cfg.Config,
		workerID:   workerID,
		jobType:    cfg.JobType,
		repo:       cfg.Repository,
		nats:       cfg.NATS,
		pipeline:   cfg.Pipeline,
		handler:    cfg.Handler,
		pollPeriod: 5 * time.Second,
		lockTime:   5 * time.Minute,
	}
}

// Run starts the worker processing loop
func (w *BaseWorker) Run(ctx context.Context) error {
	logger := log.With().
		Str("worker_id", w.workerID).
		Str("job_type", string(w.jobType)).
		Logger()

	// Try to set up NATS consumer
	if w.nats != nil && w.nats.IsConnected() {
		consumerName := qtestnats.ConsumerForJobType(string(w.jobType))
		consumer, err := w.nats.JetStream().Consumer(ctx, qtestnats.StreamJobs, consumerName)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to get consumer, falling back to polling")
		} else {
			w.consumer = consumer
			logger.Info().Msg("connected to NATS consumer")
		}
	}

	logger.Info().Msg("worker started")

	// Main processing loop
	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("worker stopping")
			return nil
		default:
			if err := w.processNext(ctx); err != nil {
				logger.Error().Err(err).Msg("error processing job")
			}
		}
	}
}

// processNext fetches and processes the next available job
func (w *BaseWorker) processNext(ctx context.Context) error {
	// Try NATS first if available
	if w.consumer != nil {
		return w.processFromNATS(ctx)
	}

	// Fall back to database polling
	return w.processFromDB(ctx)
}

// processFromNATS fetches jobs via NATS JetStream
func (w *BaseWorker) processFromNATS(ctx context.Context) error {
	// Fetch with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, w.pollPeriod)
	defer cancel()

	msgs, err := w.consumer.Fetch(1, jetstream.FetchMaxWait(w.pollPeriod))
	if err != nil {
		if err == context.DeadlineExceeded || fetchCtx.Err() != nil {
			return nil // Normal timeout, no jobs available
		}
		return fmt.Errorf("failed to fetch from NATS: %w", err)
	}

	for msg := range msgs.Messages() {
		jobMsg, err := jobs.DecodeJobMessage(msg.Data())
		if err != nil {
			log.Error().Err(err).Msg("failed to decode job message")
			msg.Nak() // Negative ack to retry
			continue
		}

		// Claim the job from DB
		job, err := w.repo.Claim(ctx, jobMsg.JobID, w.workerID, w.lockTime)
		if err != nil {
			log.Error().Err(err).Str("job_id", jobMsg.JobID.String()).Msg("failed to claim job")
			msg.Nak()
			continue
		}

		if job == nil {
			// Job already claimed by another worker
			msg.Ack()
			continue
		}

		// Process the job
		if err := w.processJob(ctx, job); err != nil {
			log.Error().Err(err).Str("job_id", job.ID.String()).Msg("job processing failed")
		}

		msg.Ack()
	}

	if msgs.Error() != nil && msgs.Error() != context.DeadlineExceeded {
		return msgs.Error()
	}

	return nil
}

// processFromDB polls the database for pending jobs
func (w *BaseWorker) processFromDB(ctx context.Context) error {
	// Get pending jobs
	pendingJobs, err := w.repo.ListPendingByType(ctx, w.jobType, 1)
	if err != nil {
		return fmt.Errorf("failed to list pending jobs: %w", err)
	}

	if len(pendingJobs) == 0 {
		// No jobs, wait before polling again
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(w.pollPeriod):
			return nil
		}
	}

	for _, pending := range pendingJobs {
		// Try to claim the job
		job, err := w.repo.Claim(ctx, pending.ID, w.workerID, w.lockTime)
		if err != nil {
			log.Warn().Err(err).Str("job_id", pending.ID.String()).Msg("failed to claim job")
			continue
		}

		if job == nil {
			// Job already claimed
			continue
		}

		if err := w.processJob(ctx, job); err != nil {
			log.Error().Err(err).Str("job_id", job.ID.String()).Msg("job processing failed")
		}
	}

	return nil
}

// processJob executes the job handler with proper error handling
func (w *BaseWorker) processJob(ctx context.Context, job *jobs.Job) error {
	logger := log.With().
		Str("worker_id", w.workerID).
		Str("job_id", job.ID.String()).
		Str("job_type", string(job.Type)).
		Logger()

	logger.Info().Msg("processing job")

	// Create a context with timeout based on lock time
	jobCtx, cancel := context.WithTimeout(ctx, w.lockTime-30*time.Second)
	defer cancel()

	// Start lock extension goroutine
	done := make(chan struct{})
	go w.extendLockPeriodically(ctx, job.ID, done)

	// Execute the handler
	err := w.handler(jobCtx, job)

	// Stop lock extension
	close(done)

	if err != nil {
		logger.Error().Err(err).Msg("job failed")
		if failErr := w.repo.Fail(ctx, job.ID, err.Error(), nil); failErr != nil {
			logger.Error().Err(failErr).Msg("failed to mark job as failed")
		}
		return err
	}

	logger.Info().Msg("job completed")
	return nil
}

// extendLockPeriodically extends the lock while job is processing
func (w *BaseWorker) extendLockPeriodically(ctx context.Context, jobID uuid.UUID, done chan struct{}) {
	ticker := time.NewTicker(w.lockTime / 2)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.repo.ExtendLock(ctx, jobID, w.workerID, w.lockTime); err != nil {
				log.Warn().Err(err).Str("job_id", jobID.String()).Msg("failed to extend lock")
			}
		}
	}
}

// WorkerID returns the worker's unique ID
func (w *BaseWorker) WorkerID() string {
	return w.workerID
}

// JobType returns the job type this worker handles
func (w *BaseWorker) JobType() jobs.JobType {
	return w.jobType
}

// SetPollPeriod sets the polling interval
func (w *BaseWorker) SetPollPeriod(d time.Duration) {
	w.pollPeriod = d
}

// SetLockTime sets the job lock duration
func (w *BaseWorker) SetLockTime(d time.Duration) {
	w.lockTime = d
}

// Repository returns the job repository
func (w *BaseWorker) Repository() *jobs.Repository {
	return w.repo
}

// Pipeline returns the pipeline manager
func (w *BaseWorker) Pipeline() *jobs.Pipeline {
	return w.pipeline
}
