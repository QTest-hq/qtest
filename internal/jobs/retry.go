// Package jobs provides retry policies and error classification for job processing
package jobs

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ErrorCategory classifies errors for retry decisions
type ErrorCategory string

const (
	// ErrorCategoryTransient indicates a temporary error that should be retried
	ErrorCategoryTransient ErrorCategory = "transient"
	// ErrorCategoryPermanent indicates a permanent error that should not be retried
	ErrorCategoryPermanent ErrorCategory = "permanent"
	// ErrorCategoryRateLimited indicates rate limiting, should retry with backoff
	ErrorCategoryRateLimited ErrorCategory = "rate_limited"
	// ErrorCategoryResource indicates resource exhaustion, should retry later
	ErrorCategoryResource ErrorCategory = "resource"
)

// RetryPolicy defines retry behavior for a job type
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialDelay is the first retry delay
	InitialDelay time.Duration
	// MaxDelay is the maximum retry delay
	MaxDelay time.Duration
	// BackoffMultiplier increases delay on each retry (exponential backoff)
	BackoffMultiplier float64
	// Jitter adds randomness to prevent thundering herd (0-1)
	Jitter float64
}

// DefaultRetryPolicy returns the default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:        3,
		InitialDelay:      30 * time.Second,
		MaxDelay:          30 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}
}

// RetryPolicies contains policies for different job types
var RetryPolicies = map[JobType]*RetryPolicy{
	JobTypeIngestion: {
		MaxRetries:        3,
		InitialDelay:      30 * time.Second,
		MaxDelay:          10 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	},
	JobTypeModeling: {
		MaxRetries:        3,
		InitialDelay:      30 * time.Second,
		MaxDelay:          10 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	},
	JobTypePlanning: {
		MaxRetries:        2,
		InitialDelay:      30 * time.Second,
		MaxDelay:          5 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	},
	JobTypeGeneration: {
		MaxRetries:        3,
		InitialDelay:      1 * time.Minute,
		MaxDelay:          15 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            0.15,
	},
	JobTypeValidation: {
		MaxRetries:        2,
		InitialDelay:      30 * time.Second,
		MaxDelay:          5 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	},
	JobTypeMutation: {
		MaxRetries:        2,
		InitialDelay:      30 * time.Second,
		MaxDelay:          5 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	},
	JobTypeIntegration: {
		MaxRetries:        2,
		InitialDelay:      30 * time.Second,
		MaxDelay:          5 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	},
}

// GetRetryPolicy returns the retry policy for a job type
func GetRetryPolicy(jobType JobType) *RetryPolicy {
	if policy, ok := RetryPolicies[jobType]; ok {
		return policy
	}
	return DefaultRetryPolicy()
}

// CalculateRetryDelay calculates the delay before the next retry
func (p *RetryPolicy) CalculateRetryDelay(retryCount int) time.Duration {
	if retryCount <= 0 {
		return p.InitialDelay
	}

	// Exponential backoff: initialDelay * (multiplier ^ retryCount)
	delay := float64(p.InitialDelay) * math.Pow(p.BackoffMultiplier, float64(retryCount))

	// Apply jitter (reduce by up to Jitter%)
	if p.Jitter > 0 {
		// Simple deterministic jitter based on retry count
		jitterFactor := 1.0 - (p.Jitter * float64(retryCount%3) / 3.0)
		delay *= jitterFactor
	}

	// Cap at max delay
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}

	return time.Duration(delay)
}

// RetryableError wraps an error with retry information
type RetryableError struct {
	Err      error
	Category ErrorCategory
	Message  string
}

func (e *RetryableError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "retryable error"
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewTransientError creates a retryable transient error
func NewTransientError(err error, msg string) error {
	return &RetryableError{
		Err:      err,
		Category: ErrorCategoryTransient,
		Message:  msg,
	}
}

// NewPermanentError creates a non-retryable permanent error
func NewPermanentError(err error, msg string) error {
	return &RetryableError{
		Err:      err,
		Category: ErrorCategoryPermanent,
		Message:  msg,
	}
}

// NewRateLimitError creates a rate-limited error
func NewRateLimitError(err error, msg string) error {
	return &RetryableError{
		Err:      err,
		Category: ErrorCategoryRateLimited,
		Message:  msg,
	}
}

// NewResourceError creates a resource exhaustion error
func NewResourceError(err error, msg string) error {
	return &RetryableError{
		Err:      err,
		Category: ErrorCategoryResource,
		Message:  msg,
	}
}

// ClassifyError determines the error category for retry decisions
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryTransient
	}

	// Check if it's already a RetryableError
	var retryErr *RetryableError
	if errors.As(err, &retryErr) {
		return retryErr.Category
	}

	errStr := strings.ToLower(err.Error())

	// Permanent errors - don't retry
	permanentPatterns := []string{
		"invalid",
		"not found",
		"permission denied",
		"unauthorized",
		"forbidden",
		"syntax error",
		"parse error",
		"unsupported",
		"not implemented",
		"malformed",
		"bad request",
	}
	for _, pattern := range permanentPatterns {
		if strings.Contains(errStr, pattern) {
			return ErrorCategoryPermanent
		}
	}

	// Rate limited errors
	rateLimitPatterns := []string{
		"rate limit",
		"too many requests",
		"429",
		"throttle",
		"quota exceeded",
	}
	for _, pattern := range rateLimitPatterns {
		if strings.Contains(errStr, pattern) {
			return ErrorCategoryRateLimited
		}
	}

	// Resource errors
	resourcePatterns := []string{
		"out of memory",
		"resource exhausted",
		"disk full",
		"no space",
		"memory limit",
		"cpu limit",
	}
	for _, pattern := range resourcePatterns {
		if strings.Contains(errStr, pattern) {
			return ErrorCategoryResource
		}
	}

	// Transient errors - do retry
	transientPatterns := []string{
		"timeout",
		"connection",
		"temporary",
		"unavailable",
		"network",
		"reset",
		"refused",
		"retry",
		"deadline",
		"context canceled",
	}
	for _, pattern := range transientPatterns {
		if strings.Contains(errStr, pattern) {
			return ErrorCategoryTransient
		}
	}

	// Default to transient (retry)
	return ErrorCategoryTransient
}

// ShouldRetry determines if a job should be retried based on error and policy
func ShouldRetry(job *Job, err error, policy *RetryPolicy) bool {
	if job == nil || err == nil {
		return false
	}

	// Check retry count
	if job.RetryCount >= policy.MaxRetries {
		return false
	}

	// Check error category
	category := ClassifyError(err)
	switch category {
	case ErrorCategoryPermanent:
		return false
	case ErrorCategoryTransient, ErrorCategoryRateLimited, ErrorCategoryResource:
		return true
	default:
		return true
	}
}

// RetryInfo contains information about a retry attempt
type RetryInfo struct {
	ShouldRetry    bool
	NextRetryAt    time.Time
	RetryDelay     time.Duration
	ErrorCategory  ErrorCategory
	RetriesLeft    int
	TotalRetries   int
}

// GetRetryInfo calculates retry information for a job
func GetRetryInfo(job *Job, err error) *RetryInfo {
	policy := GetRetryPolicy(job.Type)

	info := &RetryInfo{
		ErrorCategory: ClassifyError(err),
		TotalRetries:  policy.MaxRetries,
		RetriesLeft:   policy.MaxRetries - job.RetryCount,
	}

	info.ShouldRetry = ShouldRetry(job, err, policy)

	if info.ShouldRetry {
		info.RetryDelay = policy.CalculateRetryDelay(job.RetryCount)
		info.NextRetryAt = time.Now().Add(info.RetryDelay)
	}

	return info
}

// ScheduleRetry schedules a job for retry with proper delay
func (r *Repository) ScheduleRetry(ctx context.Context, jobID, repoID *uuid.UUID, jobType JobType, retryCount int, err error) error {
	policy := GetRetryPolicy(jobType)
	info := &RetryInfo{
		ErrorCategory: ClassifyError(err),
		TotalRetries:  policy.MaxRetries,
		RetriesLeft:   policy.MaxRetries - retryCount,
	}

	if retryCount >= policy.MaxRetries || info.ErrorCategory == ErrorCategoryPermanent {
		// Mark as failed, no more retries
		return r.Fail(ctx, *jobID, err.Error(), map[string]interface{}{
			"category":     info.ErrorCategory,
			"final":        true,
			"retry_count":  retryCount,
			"max_retries":  policy.MaxRetries,
		})
	}

	// Calculate next retry time
	delay := policy.CalculateRetryDelay(retryCount)
	nextRetryAt := time.Now().Add(delay)

	// Update job with retry info
	query := `
		UPDATE jobs
		SET status = $1,
			retry_count = retry_count + 1,
			error_message = $2,
			locked_until = $3,
			updated_at = $4
		WHERE id = $5
	`

	_, dbErr := r.db.ExecContext(ctx, query,
		StatusRetrying,
		err.Error(),
		nextRetryAt,
		time.Now(),
		*jobID,
	)
	if dbErr != nil {
		return dbErr
	}

	log.Info().
		Str("job_id", jobID.String()).
		Str("category", string(info.ErrorCategory)).
		Dur("delay", delay).
		Time("next_retry_at", nextRetryAt).
		Int("retry_count", retryCount+1).
		Int("max_retries", policy.MaxRetries).
		Msg("job scheduled for retry")

	return nil
}

// GetRetryableJobs returns jobs that are ready for retry
func (r *Repository) GetRetryableJobs(ctx context.Context, limit int) ([]*Job, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, type, status, priority, repository_id, generation_run_id,
			   parent_job_id, payload, result, error_message, error_details,
			   retry_count, max_retries, created_at, updated_at, started_at,
			   completed_at, locked_until, worker_id
		FROM jobs
		WHERE status = $1
		  AND (locked_until IS NULL OR locked_until <= $2)
		ORDER BY priority DESC, created_at ASC
		LIMIT $3
	`

	return r.queryJobs(ctx, query, StatusRetrying, time.Now(), limit)
}

// ProcessRetryQueue processes jobs that are ready for retry
func (p *Pipeline) ProcessRetryQueue(ctx context.Context) (int, error) {
	jobs, err := p.repo.GetRetryableJobs(ctx, 50)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, job := range jobs {
		// Move back to pending
		if err := p.repo.Retry(ctx, job.ID); err != nil {
			log.Warn().Err(err).Str("job_id", job.ID.String()).Msg("failed to retry job")
			continue
		}

		// Republish to queue
		job.Status = StatusPending
		if err := p.publishJob(ctx, job); err != nil {
			log.Warn().Err(err).Str("job_id", job.ID.String()).Msg("failed to republish job")
		}

		count++
	}

	return count, nil
}
