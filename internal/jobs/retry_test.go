package jobs

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	assert.Equal(t, 3, policy.MaxRetries)
	assert.Equal(t, 30*time.Second, policy.InitialDelay)
	assert.Equal(t, 30*time.Minute, policy.MaxDelay)
	assert.Equal(t, 2.0, policy.BackoffMultiplier)
}

func TestGetRetryPolicy(t *testing.T) {
	tests := []struct {
		jobType JobType
		wantMax int
	}{
		{JobTypeIngestion, 3},
		{JobTypeModeling, 3},
		{JobTypePlanning, 2},
		{JobTypeGeneration, 3},
		{JobTypeValidation, 2},
		{JobTypeMutation, 2},
		{JobTypeIntegration, 2},
	}

	for _, tt := range tests {
		t.Run(string(tt.jobType), func(t *testing.T) {
			policy := GetRetryPolicy(tt.jobType)
			assert.Equal(t, tt.wantMax, policy.MaxRetries)
		})
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:        3,
		InitialDelay:      10 * time.Second,
		MaxDelay:          5 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            0,
	}

	tests := []struct {
		retryCount int
		wantDelay  time.Duration
	}{
		{0, 10 * time.Second},  // First retry: 10s
		{1, 20 * time.Second},  // Second retry: 10s * 2 = 20s
		{2, 40 * time.Second},  // Third retry: 10s * 4 = 40s
		{3, 80 * time.Second},  // Fourth retry: 10s * 8 = 80s
	}

	for _, tt := range tests {
		delay := policy.CalculateRetryDelay(tt.retryCount)
		assert.Equal(t, tt.wantDelay, delay, "retry count %d", tt.retryCount)
	}
}

func TestCalculateRetryDelay_MaxCap(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:        10,
		InitialDelay:      1 * time.Minute,
		MaxDelay:          5 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            0,
	}

	// After several retries, delay should be capped at MaxDelay
	delay := policy.CalculateRetryDelay(5)
	assert.Equal(t, 5*time.Minute, delay)
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCategory
	}{
		// Permanent errors
		{"invalid input", errors.New("invalid request format"), ErrorCategoryPermanent},
		{"not found", errors.New("resource not found"), ErrorCategoryPermanent},
		{"permission denied", errors.New("permission denied"), ErrorCategoryPermanent},
		{"unauthorized", errors.New("unauthorized access"), ErrorCategoryPermanent},
		{"forbidden", errors.New("forbidden action"), ErrorCategoryPermanent},
		{"syntax error", errors.New("syntax error in query"), ErrorCategoryPermanent},
		{"parse error", errors.New("parse error: invalid JSON"), ErrorCategoryPermanent},
		{"unsupported", errors.New("unsupported operation"), ErrorCategoryPermanent},

		// Rate limited errors
		{"rate limit", errors.New("rate limit exceeded"), ErrorCategoryRateLimited},
		{"too many requests", errors.New("too many requests"), ErrorCategoryRateLimited},
		{"429 status", errors.New("HTTP 429: throttled"), ErrorCategoryRateLimited},
		{"quota exceeded", errors.New("API quota exceeded"), ErrorCategoryRateLimited},

		// Resource errors
		{"out of memory", errors.New("out of memory"), ErrorCategoryResource},
		{"disk full", errors.New("disk full"), ErrorCategoryResource},
		{"resource exhausted", errors.New("resource exhausted"), ErrorCategoryResource},

		// Transient errors
		{"timeout", errors.New("connection timeout"), ErrorCategoryTransient},
		{"connection reset", errors.New("connection reset by peer"), ErrorCategoryTransient},
		{"network error", errors.New("network error"), ErrorCategoryTransient},
		{"connection refused", errors.New("connection refused"), ErrorCategoryTransient},
		{"temporarily unavailable", errors.New("service temporarily unavailable"), ErrorCategoryTransient},
		{"deadline exceeded", errors.New("context deadline exceeded"), ErrorCategoryTransient},

		// Default to transient
		{"unknown error", errors.New("some unknown error"), ErrorCategoryTransient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := ClassifyError(tt.err)
			assert.Equal(t, tt.expected, category)
		})
	}
}

func TestClassifyError_RetryableError(t *testing.T) {
	// Test that RetryableError category is preserved
	err := &RetryableError{
		Err:      errors.New("test error"),
		Category: ErrorCategoryPermanent,
		Message:  "permanent error",
	}

	category := ClassifyError(err)
	assert.Equal(t, ErrorCategoryPermanent, category)
}

func TestRetryableError(t *testing.T) {
	underlying := errors.New("underlying error")

	t.Run("NewTransientError", func(t *testing.T) {
		err := NewTransientError(underlying, "transient message")
		var retryErr *RetryableError
		assert.True(t, errors.As(err, &retryErr))
		assert.Equal(t, ErrorCategoryTransient, retryErr.Category)
		assert.Equal(t, "transient message", retryErr.Message)
		assert.Equal(t, underlying, retryErr.Unwrap())
	})

	t.Run("NewPermanentError", func(t *testing.T) {
		err := NewPermanentError(underlying, "permanent message")
		var retryErr *RetryableError
		assert.True(t, errors.As(err, &retryErr))
		assert.Equal(t, ErrorCategoryPermanent, retryErr.Category)
	})

	t.Run("NewRateLimitError", func(t *testing.T) {
		err := NewRateLimitError(underlying, "rate limited")
		var retryErr *RetryableError
		assert.True(t, errors.As(err, &retryErr))
		assert.Equal(t, ErrorCategoryRateLimited, retryErr.Category)
	})

	t.Run("NewResourceError", func(t *testing.T) {
		err := NewResourceError(underlying, "resource error")
		var retryErr *RetryableError
		assert.True(t, errors.As(err, &retryErr))
		assert.Equal(t, ErrorCategoryResource, retryErr.Category)
	})
}

func TestShouldRetry(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:        3,
		InitialDelay:      10 * time.Second,
		MaxDelay:          5 * time.Minute,
		BackoffMultiplier: 2.0,
	}

	tests := []struct {
		name       string
		retryCount int
		err        error
		want       bool
	}{
		{"nil job", 0, errors.New("error"), false},
		{"nil error", 0, nil, false},
		{"first retry transient", 0, errors.New("connection timeout"), true},
		{"second retry transient", 1, errors.New("network error"), true},
		{"max retries reached", 3, errors.New("timeout"), false},
		{"permanent error", 0, errors.New("invalid request"), false},
		{"rate limited", 0, errors.New("rate limit exceeded"), true},
		{"resource error", 0, errors.New("out of memory"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var job *Job
			if tt.name != "nil job" {
				job = &Job{
					Type:       JobTypeGeneration,
					RetryCount: tt.retryCount,
					MaxRetries: policy.MaxRetries,
				}
			}

			result := ShouldRetry(job, tt.err, policy)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestGetRetryInfo(t *testing.T) {
	job := &Job{
		Type:       JobTypeGeneration,
		RetryCount: 1,
		MaxRetries: 3,
	}

	err := errors.New("connection timeout")
	info := GetRetryInfo(job, err)

	assert.True(t, info.ShouldRetry)
	assert.Equal(t, ErrorCategoryTransient, info.ErrorCategory)
	assert.Equal(t, 3, info.TotalRetries)
	assert.Equal(t, 2, info.RetriesLeft) // 3 - 1 = 2
	assert.True(t, info.RetryDelay > 0)
	assert.True(t, info.NextRetryAt.After(time.Now()))
}

func TestGetRetryInfo_NoRetry(t *testing.T) {
	job := &Job{
		Type:       JobTypeGeneration,
		RetryCount: 3, // Max retries reached
		MaxRetries: 3,
	}

	err := errors.New("connection timeout")
	info := GetRetryInfo(job, err)

	assert.False(t, info.ShouldRetry)
	assert.Equal(t, 0, info.RetriesLeft)
}

func TestRetryableErrorMessage(t *testing.T) {
	t.Run("with message", func(t *testing.T) {
		err := &RetryableError{
			Err:      errors.New("underlying"),
			Category: ErrorCategoryTransient,
			Message:  "custom message",
		}
		assert.Equal(t, "custom message", err.Error())
	})

	t.Run("without message", func(t *testing.T) {
		err := &RetryableError{
			Err:      errors.New("underlying"),
			Category: ErrorCategoryTransient,
		}
		assert.Equal(t, "underlying", err.Error())
	})

	t.Run("no err no message", func(t *testing.T) {
		err := &RetryableError{
			Category: ErrorCategoryTransient,
		}
		assert.Equal(t, "retryable error", err.Error())
	})
}

func TestRetryPolicyExists(t *testing.T) {
	// Ensure all job types have policies defined
	jobTypes := []JobType{
		JobTypeIngestion,
		JobTypeModeling,
		JobTypePlanning,
		JobTypeGeneration,
		JobTypeValidation,
		JobTypeMutation,
		JobTypeIntegration,
	}

	for _, jt := range jobTypes {
		policy := GetRetryPolicy(jt)
		assert.NotNil(t, policy, "policy for %s should exist", jt)
		assert.True(t, policy.MaxRetries > 0, "max retries for %s should be > 0", jt)
		assert.True(t, policy.InitialDelay > 0, "initial delay for %s should be > 0", jt)
		assert.True(t, policy.MaxDelay > policy.InitialDelay, "max delay for %s should be > initial delay", jt)
	}
}

func TestCalculateRetryDelay_WithJitter(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:        3,
		InitialDelay:      10 * time.Second,
		MaxDelay:          5 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            0.3, // 30% jitter
	}

	// With jitter, delays should vary but still follow exponential pattern
	delay0 := policy.CalculateRetryDelay(0)
	delay1 := policy.CalculateRetryDelay(1)
	delay2 := policy.CalculateRetryDelay(2)

	// Each subsequent delay should generally be larger
	assert.True(t, delay0 <= 10*time.Second)
	assert.True(t, delay1 > delay0 || delay1 <= 20*time.Second)
	assert.True(t, delay2 > delay0)
}

func TestClassifyError_Nil(t *testing.T) {
	category := ClassifyError(nil)
	assert.Equal(t, ErrorCategoryTransient, category)
}
