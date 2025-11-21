// Package jobs provides job persistence and management
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Repository handles job persistence
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new job repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new job into the database
func (r *Repository) Create(ctx context.Context, job *Job) error {
	query := `
		INSERT INTO jobs (
			id, type, status, priority, repository_id, generation_run_id,
			parent_job_id, payload, max_retries, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.ExecContext(ctx, query,
		job.ID, job.Type, job.Status, job.Priority,
		job.RepositoryID, job.GenerationRunID, job.ParentJobID,
		job.Payload, job.MaxRetries, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

// GetByID retrieves a job by ID
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Job, error) {
	query := `
		SELECT id, type, status, priority, repository_id, generation_run_id,
			   parent_job_id, payload, result, error_message, error_details,
			   retry_count, max_retries, created_at, updated_at, started_at,
			   completed_at, locked_until, worker_id
		FROM jobs WHERE id = $1
	`

	job := &Job{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID, &job.Type, &job.Status, &job.Priority,
		&job.RepositoryID, &job.GenerationRunID, &job.ParentJobID,
		&job.Payload, &job.Result, &job.ErrorMessage, &job.ErrorDetails,
		&job.RetryCount, &job.MaxRetries, &job.CreatedAt, &job.UpdatedAt,
		&job.StartedAt, &job.CompletedAt, &job.LockedUntil, &job.WorkerID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return job, nil
}

// Claim attempts to claim a job for processing (distributed locking)
func (r *Repository) Claim(ctx context.Context, jobID uuid.UUID, workerID string, lockDuration time.Duration) (*Job, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Try to claim the job with optimistic locking
	now := time.Now()
	lockedUntil := now.Add(lockDuration)

	query := `
		UPDATE jobs
		SET status = $1, worker_id = $2, locked_until = $3,
			started_at = $4, updated_at = $4
		WHERE id = $5
		  AND (status = 'pending' OR (status = 'running' AND locked_until < $4))
		RETURNING id, type, status, priority, repository_id, generation_run_id,
				  parent_job_id, payload, result, error_message, error_details,
				  retry_count, max_retries, created_at, updated_at, started_at,
				  completed_at, locked_until, worker_id
	`

	job := &Job{}
	err = tx.QueryRowContext(ctx, query,
		StatusRunning, workerID, lockedUntil, now, jobID,
	).Scan(
		&job.ID, &job.Type, &job.Status, &job.Priority,
		&job.RepositoryID, &job.GenerationRunID, &job.ParentJobID,
		&job.Payload, &job.Result, &job.ErrorMessage, &job.ErrorDetails,
		&job.RetryCount, &job.MaxRetries, &job.CreatedAt, &job.UpdatedAt,
		&job.StartedAt, &job.CompletedAt, &job.LockedUntil, &job.WorkerID,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Job already claimed or not pending
	}
	if err != nil {
		return nil, fmt.Errorf("failed to claim job: %w", err)
	}

	// Record history
	if err := r.recordHistory(ctx, tx, job.ID, string(StatusPending), string(StatusRunning), workerID); err != nil {
		log.Warn().Err(err).Msg("failed to record job history")
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return job, nil
}

// Complete marks a job as completed with result
func (r *Repository) Complete(ctx context.Context, jobID uuid.UUID, result interface{}) error {
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	query := `
		UPDATE jobs
		SET status = $1, result = $2, completed_at = $3, updated_at = $3,
			locked_until = NULL
		WHERE id = $4
		RETURNING worker_id
	`

	var workerID *string
	err = tx.QueryRowContext(ctx, query, StatusCompleted, resultBytes, now, jobID).Scan(&workerID)
	if err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	changedBy := "system"
	if workerID != nil {
		changedBy = *workerID
	}
	if err := r.recordHistory(ctx, tx, jobID, string(StatusRunning), string(StatusCompleted), changedBy); err != nil {
		log.Warn().Err(err).Msg("failed to record job history")
	}

	return tx.Commit()
}

// Fail marks a job as failed with error details
func (r *Repository) Fail(ctx context.Context, jobID uuid.UUID, errMsg string, errDetails interface{}) error {
	var errDetailsBytes []byte
	if errDetails != nil {
		var err error
		errDetailsBytes, err = json.Marshal(errDetails)
		if err != nil {
			errDetailsBytes = []byte(fmt.Sprintf(`{"raw": %q}`, err.Error()))
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if can retry
	var retryCount, maxRetries int
	var workerID *string
	err = tx.QueryRowContext(ctx,
		"SELECT retry_count, max_retries, worker_id FROM jobs WHERE id = $1", jobID,
	).Scan(&retryCount, &maxRetries, &workerID)
	if err != nil {
		return fmt.Errorf("failed to get job retry info: %w", err)
	}

	now := time.Now()
	newStatus := StatusFailed
	if retryCount < maxRetries {
		newStatus = StatusRetrying
	}

	query := `
		UPDATE jobs
		SET status = $1, error_message = $2, error_details = $3,
			retry_count = retry_count + 1, updated_at = $4, locked_until = NULL
		WHERE id = $5
	`

	_, err = tx.ExecContext(ctx, query, newStatus, errMsg, errDetailsBytes, now, jobID)
	if err != nil {
		return fmt.Errorf("failed to fail job: %w", err)
	}

	changedBy := "system"
	if workerID != nil {
		changedBy = *workerID
	}
	if err := r.recordHistory(ctx, tx, jobID, string(StatusRunning), string(newStatus), changedBy); err != nil {
		log.Warn().Err(err).Msg("failed to record job history")
	}

	return tx.Commit()
}

// Retry requeues a failed job for retry
func (r *Repository) Retry(ctx context.Context, jobID uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		UPDATE jobs
		SET status = $1, updated_at = $2, worker_id = NULL,
			started_at = NULL, locked_until = NULL
		WHERE id = $3 AND status = 'retrying'
	`

	result, err := tx.ExecContext(ctx, query, StatusPending, time.Now(), jobID)
	if err != nil {
		return fmt.Errorf("failed to retry job: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("job not in retrying status")
	}

	if err := r.recordHistory(ctx, tx, jobID, string(StatusRetrying), string(StatusPending), "system"); err != nil {
		log.Warn().Err(err).Msg("failed to record job history")
	}

	return tx.Commit()
}

// Cancel cancels a pending job
func (r *Repository) Cancel(ctx context.Context, jobID uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var prevStatus string
	err = tx.QueryRowContext(ctx, "SELECT status FROM jobs WHERE id = $1", jobID).Scan(&prevStatus)
	if err != nil {
		return fmt.Errorf("failed to get job status: %w", err)
	}

	if prevStatus != string(StatusPending) && prevStatus != string(StatusRetrying) {
		return fmt.Errorf("can only cancel pending or retrying jobs")
	}

	query := `UPDATE jobs SET status = $1, updated_at = $2 WHERE id = $3`
	_, err = tx.ExecContext(ctx, query, StatusCancelled, time.Now(), jobID)
	if err != nil {
		return fmt.Errorf("failed to cancel job: %w", err)
	}

	if err := r.recordHistory(ctx, tx, jobID, prevStatus, string(StatusCancelled), "api"); err != nil {
		log.Warn().Err(err).Msg("failed to record job history")
	}

	return tx.Commit()
}

// ListByRepository returns jobs for a repository
func (r *Repository) ListByRepository(ctx context.Context, repoID uuid.UUID, limit int) ([]*Job, error) {
	query := `
		SELECT id, type, status, priority, repository_id, generation_run_id,
			   parent_job_id, payload, result, error_message, error_details,
			   retry_count, max_retries, created_at, updated_at, started_at,
			   completed_at, locked_until, worker_id
		FROM jobs
		WHERE repository_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	return r.queryJobs(ctx, query, repoID, limit)
}

// ListByStatus returns jobs with a specific status
func (r *Repository) ListByStatus(ctx context.Context, status JobStatus, limit int) ([]*Job, error) {
	query := `
		SELECT id, type, status, priority, repository_id, generation_run_id,
			   parent_job_id, payload, result, error_message, error_details,
			   retry_count, max_retries, created_at, updated_at, started_at,
			   completed_at, locked_until, worker_id
		FROM jobs
		WHERE status = $1
		ORDER BY priority DESC, created_at
		LIMIT $2
	`

	return r.queryJobs(ctx, query, status, limit)
}

// ListPendingByType returns pending jobs of a specific type
func (r *Repository) ListPendingByType(ctx context.Context, jobType JobType, limit int) ([]*Job, error) {
	query := `
		SELECT id, type, status, priority, repository_id, generation_run_id,
			   parent_job_id, payload, result, error_message, error_details,
			   retry_count, max_retries, created_at, updated_at, started_at,
			   completed_at, locked_until, worker_id
		FROM jobs
		WHERE type = $1 AND status = 'pending'
		ORDER BY priority DESC, created_at
		LIMIT $2
	`

	return r.queryJobs(ctx, query, jobType, limit)
}

// GetChildJobs returns all child jobs of a parent job
func (r *Repository) GetChildJobs(ctx context.Context, parentID uuid.UUID) ([]*Job, error) {
	query := `
		SELECT id, type, status, priority, repository_id, generation_run_id,
			   parent_job_id, payload, result, error_message, error_details,
			   retry_count, max_retries, created_at, updated_at, started_at,
			   completed_at, locked_until, worker_id
		FROM jobs
		WHERE parent_job_id = $1
		ORDER BY created_at
	`

	return r.queryJobs(ctx, query, parentID)
}

// ExtendLock extends the lock on a running job
func (r *Repository) ExtendLock(ctx context.Context, jobID uuid.UUID, workerID string, duration time.Duration) error {
	query := `
		UPDATE jobs
		SET locked_until = $1, updated_at = $2
		WHERE id = $3 AND worker_id = $4 AND status = 'running'
	`

	result, err := r.db.ExecContext(ctx, query, time.Now().Add(duration), time.Now(), jobID, workerID)
	if err != nil {
		return fmt.Errorf("failed to extend lock: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("job not found or not owned by worker")
	}

	return nil
}

// CleanupStale resets jobs that have been running too long (stale locks)
func (r *Repository) CleanupStale(ctx context.Context) (int, error) {
	query := `
		UPDATE jobs
		SET status = 'pending', worker_id = NULL, started_at = NULL,
			locked_until = NULL, updated_at = $1
		WHERE status = 'running' AND locked_until < $1
	`

	result, err := r.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup stale jobs: %w", err)
	}

	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// queryJobs is a helper to query multiple jobs
func (r *Repository) queryJobs(ctx context.Context, query string, args ...interface{}) ([]*Job, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		job := &Job{}
		err := rows.Scan(
			&job.ID, &job.Type, &job.Status, &job.Priority,
			&job.RepositoryID, &job.GenerationRunID, &job.ParentJobID,
			&job.Payload, &job.Result, &job.ErrorMessage, &job.ErrorDetails,
			&job.RetryCount, &job.MaxRetries, &job.CreatedAt, &job.UpdatedAt,
			&job.StartedAt, &job.CompletedAt, &job.LockedUntil, &job.WorkerID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// recordHistory records a job status change in the history table
func (r *Repository) recordHistory(ctx context.Context, tx *sql.Tx, jobID uuid.UUID, prevStatus, newStatus, changedBy string) error {
	query := `
		INSERT INTO job_history (id, job_id, previous_status, new_status, changed_by)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := tx.ExecContext(ctx, query, uuid.New(), jobID, prevStatus, newStatus, changedBy)
	return err
}
