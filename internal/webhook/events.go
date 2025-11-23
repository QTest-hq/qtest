// Package webhook provides webhook delivery functionality
package webhook

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event type constants
const (
	EventJobCompleted      = "job.completed"
	EventJobFailed         = "job.failed"
	EventRunStarted        = "run.started"
	EventRunCompleted      = "run.completed"
	EventTestsGenerated    = "tests.generated"
	EventTestsValidated    = "tests.validated"
	EventMutationCompleted = "mutation.completed"
)

// AllEventTypes returns all supported event types
func AllEventTypes() []string {
	return []string{
		EventJobCompleted,
		EventJobFailed,
		EventRunStarted,
		EventRunCompleted,
		EventTestsGenerated,
		EventTestsValidated,
		EventMutationCompleted,
	}
}

// Event represents a webhook event payload
type Event struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	CreatedAt      time.Time       `json:"created_at"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	Data           json.RawMessage `json:"data"`
}

// NewEvent creates a new event
func NewEvent(eventType string, orgID uuid.UUID, data interface{}) (*Event, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &Event{
		ID:             "evt_" + uuid.New().String()[:8],
		Type:           eventType,
		CreatedAt:      time.Now().UTC(),
		OrganizationID: orgID,
		Data:           dataJSON,
	}, nil
}

// JobCompletedData represents data for job.completed event
type JobCompletedData struct {
	JobID        uuid.UUID `json:"job_id"`
	JobType      string    `json:"job_type"`
	RepositoryID uuid.UUID `json:"repository_id"`
	Status       string    `json:"status"`
	Duration     int64     `json:"duration_ms,omitempty"`
}

// JobFailedData represents data for job.failed event
type JobFailedData struct {
	JobID        uuid.UUID `json:"job_id"`
	JobType      string    `json:"job_type"`
	RepositoryID uuid.UUID `json:"repository_id"`
	Error        string    `json:"error"`
	Retries      int       `json:"retries"`
}

// RunStartedData represents data for run.started event
type RunStartedData struct {
	RunID        uuid.UUID `json:"run_id"`
	RepositoryID uuid.UUID `json:"repository_id"`
}

// RunCompletedData represents data for run.completed event
type RunCompletedData struct {
	RunID          uuid.UUID `json:"run_id"`
	RepositoryID   uuid.UUID `json:"repository_id"`
	Status         string    `json:"status"`
	TestsGenerated int       `json:"tests_generated"`
	TestsPassed    int       `json:"tests_passed,omitempty"`
	TestsFailed    int       `json:"tests_failed,omitempty"`
	Duration       int64     `json:"duration_ms,omitempty"`
}

// TestsGeneratedData represents data for tests.generated event
type TestsGeneratedData struct {
	RunID        uuid.UUID `json:"run_id"`
	RepositoryID uuid.UUID `json:"repository_id"`
	TestCount    int       `json:"test_count"`
	FilePath     string    `json:"file_path,omitempty"`
}

// TestsValidatedData represents data for tests.validated event
type TestsValidatedData struct {
	RunID        uuid.UUID `json:"run_id"`
	RepositoryID uuid.UUID `json:"repository_id"`
	Passed       int       `json:"passed"`
	Failed       int       `json:"failed"`
	Skipped      int       `json:"skipped"`
}

// MutationCompletedData represents data for mutation.completed event
type MutationCompletedData struct {
	MutationRunID uuid.UUID `json:"mutation_run_id"`
	RepositoryID  uuid.UUID `json:"repository_id"`
	TotalMutants  int       `json:"total_mutants"`
	Killed        int       `json:"killed"`
	Survived      int       `json:"survived"`
	Score         float64   `json:"mutation_score"`
}
