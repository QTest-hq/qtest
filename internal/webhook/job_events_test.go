package webhook

import (
	"context"
	"testing"

	"github.com/QTest-hq/qtest/internal/jobs"
	"github.com/google/uuid"
)

// TestNewJobEventHandler tests handler creation
func TestNewJobEventHandler(t *testing.T) {
	handler := NewJobEventHandler(nil, nil)
	if handler == nil {
		t.Fatal("expected handler to be created")
	}
	if handler.service != nil {
		t.Error("expected service to be nil")
	}
	if handler.store != nil {
		t.Error("expected store to be nil")
	}
}

// TestJobEventHandler_HandleEvent_NilService tests handling with nil service
func TestJobEventHandler_HandleEvent_NilService(t *testing.T) {
	handler := NewJobEventHandler(nil, nil)

	repoID := uuid.New()
	event := &jobs.JobEvent{
		Type:    jobs.EventTypeJobCompleted,
		JobID:   uuid.New(),
		JobType: jobs.JobTypeGeneration,
		RepoID:  &repoID,
		Status:  "completed",
	}

	// Should not panic
	handler.HandleEvent(context.Background(), event)
}

// TestJobEventHandler_HandleEvent_NilRepoID tests handling with nil repo ID
func TestJobEventHandler_HandleEvent_NilRepoID(t *testing.T) {
	handler := NewJobEventHandler(nil, nil)

	event := &jobs.JobEvent{
		Type:    jobs.EventTypeJobCompleted,
		JobID:   uuid.New(),
		JobType: jobs.JobTypeGeneration,
		RepoID:  nil, // No repo ID
		Status:  "completed",
	}

	// Should not panic and should return early
	handler.HandleEvent(context.Background(), event)
}

// TestJobCompletedData tests JobCompletedData struct
func TestJobCompletedData(t *testing.T) {
	jobID := uuid.New()
	repoID := uuid.New()

	data := &JobCompletedData{
		JobID:        jobID,
		JobType:      "generation",
		RepositoryID: repoID,
		Status:       "completed",
		Duration:     5000,
	}

	if data.JobID != jobID {
		t.Error("JobID mismatch")
	}
	if data.JobType != "generation" {
		t.Errorf("JobType = %s, want generation", data.JobType)
	}
	if data.RepositoryID != repoID {
		t.Error("RepositoryID mismatch")
	}
	if data.Status != "completed" {
		t.Errorf("Status = %s, want completed", data.Status)
	}
	if data.Duration != 5000 {
		t.Errorf("Duration = %d, want 5000", data.Duration)
	}
}

// TestJobFailedData tests JobFailedData struct
func TestJobFailedData(t *testing.T) {
	jobID := uuid.New()
	repoID := uuid.New()

	data := &JobFailedData{
		JobID:        jobID,
		JobType:      "ingestion",
		RepositoryID: repoID,
		Error:        "connection timeout",
		Retries:      3,
	}

	if data.JobID != jobID {
		t.Error("JobID mismatch")
	}
	if data.JobType != "ingestion" {
		t.Errorf("JobType = %s, want ingestion", data.JobType)
	}
	if data.Error != "connection timeout" {
		t.Errorf("Error = %s, want connection timeout", data.Error)
	}
	if data.Retries != 3 {
		t.Errorf("Retries = %d, want 3", data.Retries)
	}
}

// TestJobCompletedData_ZeroDuration tests optional duration field
func TestJobCompletedData_ZeroDuration(t *testing.T) {
	data := &JobCompletedData{
		JobID:    uuid.New(),
		JobType:  "generation",
		Status:   "completed",
		Duration: 0, // Zero is valid
	}

	if data.Duration != 0 {
		t.Errorf("Duration = %d, want 0", data.Duration)
	}
}

// TestJobFailedData_ZeroRetries tests optional retries field
func TestJobFailedData_ZeroRetries(t *testing.T) {
	data := &JobFailedData{
		JobID:   uuid.New(),
		JobType: "generation",
		Error:   "immediate failure",
		Retries: 0, // Zero retries is valid
	}

	if data.Retries != 0 {
		t.Errorf("Retries = %d, want 0", data.Retries)
	}
}

// TestJobEvent_AllJobTypes tests event handling for all job types
func TestJobEvent_AllJobTypes(t *testing.T) {
	handler := NewJobEventHandler(nil, nil)
	ctx := context.Background()

	jobTypes := []jobs.JobType{
		jobs.JobTypeGeneration,
		jobs.JobTypeIngestion,
		jobs.JobTypeMutation,
		jobs.JobTypeValidation,
	}

	for _, jt := range jobTypes {
		t.Run(string(jt), func(t *testing.T) {
			repoID := uuid.New()
			event := &jobs.JobEvent{
				Type:    jobs.EventTypeJobCompleted,
				JobID:   uuid.New(),
				JobType: jt,
				RepoID:  &repoID,
				Status:  "completed",
			}

			// Should not panic (will return early since service is nil)
			handler.HandleEvent(ctx, event)
		})
	}
}

// TestJobEvent_BothEventTypes tests both completed and failed event types
func TestJobEvent_BothEventTypes(t *testing.T) {
	handler := NewJobEventHandler(nil, nil)
	ctx := context.Background()
	repoID := uuid.New()

	tests := []struct {
		name      string
		eventType jobs.EventType
		status    string
		errMsg    string
	}{
		{
			name:      "job completed",
			eventType: jobs.EventTypeJobCompleted,
			status:    "completed",
		},
		{
			name:      "job failed",
			eventType: jobs.EventTypeJobFailed,
			status:    "failed",
			errMsg:    "test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &jobs.JobEvent{
				Type:    tt.eventType,
				JobID:   uuid.New(),
				JobType: jobs.JobTypeGeneration,
				RepoID:  &repoID,
				Status:  tt.status,
				Error:   tt.errMsg,
			}

			// Should not panic
			handler.HandleEvent(ctx, event)
		})
	}
}
