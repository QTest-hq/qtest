package jobs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJobType_Constants(t *testing.T) {
	tests := []struct {
		jobType JobType
		want    string
	}{
		{JobTypeIngestion, "ingestion"},
		{JobTypeModeling, "modeling"},
		{JobTypePlanning, "planning"},
		{JobTypeGeneration, "generation"},
		{JobTypeMutation, "mutation"},
		{JobTypeIntegration, "integration"},
	}

	for _, tt := range tests {
		if string(tt.jobType) != tt.want {
			t.Errorf("JobType %v = %s, want %s", tt.jobType, string(tt.jobType), tt.want)
		}
	}
}

func TestJobStatus_Constants(t *testing.T) {
	tests := []struct {
		status JobStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusRetrying, "retrying"},
		{StatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("JobStatus %v = %s, want %s", tt.status, string(tt.status), tt.want)
		}
	}
}

func TestNewJob(t *testing.T) {
	payload := IngestionPayload{
		RepositoryURL: "https://github.com/test/repo",
		Branch:        "main",
	}

	job, err := NewJob(JobTypeIngestion, payload)
	if err != nil {
		t.Fatalf("NewJob failed: %v", err)
	}

	if job.ID == uuid.Nil {
		t.Error("job.ID should not be nil")
	}
	if job.Type != JobTypeIngestion {
		t.Errorf("job.Type = %s, want ingestion", job.Type)
	}
	if job.Status != StatusPending {
		t.Errorf("job.Status = %s, want pending", job.Status)
	}
	if job.RetryCount != 0 {
		t.Errorf("job.RetryCount = %d, want 0", job.RetryCount)
	}
	if job.MaxRetries != 3 {
		t.Errorf("job.MaxRetries = %d, want 3", job.MaxRetries)
	}
}

func TestJob_GetSetPayload(t *testing.T) {
	job := &Job{
		ID:        uuid.New(),
		Type:      JobTypeIngestion,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	original := IngestionPayload{
		RepositoryURL: "https://github.com/test/repo",
		Branch:        "main",
		CommitHash:    "abc123",
	}

	if err := job.SetPayload(original); err != nil {
		t.Fatalf("SetPayload failed: %v", err)
	}

	var retrieved IngestionPayload
	if err := job.GetPayload(&retrieved); err != nil {
		t.Fatalf("GetPayload failed: %v", err)
	}

	if retrieved.RepositoryURL != original.RepositoryURL {
		t.Errorf("RepositoryURL = %s, want %s", retrieved.RepositoryURL, original.RepositoryURL)
	}
	if retrieved.Branch != original.Branch {
		t.Errorf("Branch = %s, want %s", retrieved.Branch, original.Branch)
	}
	if retrieved.CommitHash != original.CommitHash {
		t.Errorf("CommitHash = %s, want %s", retrieved.CommitHash, original.CommitHash)
	}
}

func TestJob_GetSetResult(t *testing.T) {
	job := &Job{
		ID:     uuid.New(),
		Type:   JobTypeIngestion,
		Status: StatusCompleted,
	}

	original := IngestionResult{
		RepositoryID:  uuid.New(),
		WorkspacePath: "/tmp/workspace",
		FileCount:     42,
		Language:      "go",
	}

	if err := job.SetResult(original); err != nil {
		t.Fatalf("SetResult failed: %v", err)
	}

	var retrieved IngestionResult
	if err := job.GetResult(&retrieved); err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}

	if retrieved.RepositoryID != original.RepositoryID {
		t.Errorf("RepositoryID mismatch")
	}
	if retrieved.FileCount != original.FileCount {
		t.Errorf("FileCount = %d, want %d", retrieved.FileCount, original.FileCount)
	}
}

func TestJob_CanRetry(t *testing.T) {
	tests := []struct {
		name       string
		retryCount int
		maxRetries int
		want       bool
	}{
		{"can retry", 0, 3, true},
		{"can retry once more", 2, 3, true},
		{"cannot retry", 3, 3, false},
		{"exceeded", 5, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{
				RetryCount: tt.retryCount,
				MaxRetries: tt.maxRetries,
			}
			if got := job.CanRetry(); got != tt.want {
				t.Errorf("CanRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJobMessage_Encode(t *testing.T) {
	msg := &JobMessage{
		JobID:    uuid.New(),
		Type:     JobTypeGeneration,
		Priority: 5,
	}

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeJobMessage(data)
	if err != nil {
		t.Fatalf("DecodeJobMessage failed: %v", err)
	}

	if decoded.JobID != msg.JobID {
		t.Errorf("JobID mismatch")
	}
	if decoded.Type != msg.Type {
		t.Errorf("Type = %s, want %s", decoded.Type, msg.Type)
	}
	if decoded.Priority != msg.Priority {
		t.Errorf("Priority = %d, want %d", decoded.Priority, msg.Priority)
	}
}

func TestPayload_JSON(t *testing.T) {
	tests := []struct {
		name    string
		payload interface{}
	}{
		{"IngestionPayload", IngestionPayload{RepositoryURL: "url", Branch: "main"}},
		{"ModelingPayload", ModelingPayload{RepositoryID: uuid.New(), WorkspacePath: "/tmp"}},
		{"PlanningPayload", PlanningPayload{RepositoryID: uuid.New(), MaxTests: 100}},
		{"GenerationPayload", GenerationPayload{RepositoryID: uuid.New(), LLMTier: 2}},
		{"MutationPayload", MutationPayload{TestFilePath: "test.go", SourceFilePath: "main.go"}},
		{"IntegrationPayload", IntegrationPayload{TestFilePaths: []string{"a.go", "b.go"}, CreatePR: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if len(data) == 0 {
				t.Error("marshaled data should not be empty")
			}
		})
	}
}

func TestResult_JSON(t *testing.T) {
	tests := []struct {
		name   string
		result interface{}
	}{
		{"IngestionResult", IngestionResult{RepositoryID: uuid.New(), FileCount: 10, Language: "go"}},
		{"ModelingResult", ModelingResult{ModelID: uuid.New(), FunctionCount: 50}},
		{"PlanningResult", PlanningResult{PlanID: uuid.New(), TotalTests: 100}},
		{"GenerationResult", GenerationResult{TestsGenerated: 20, TestFilePaths: []string{"a_test.go"}}},
		{"MutationResult", MutationResult{MutantsTotal: 100, MutantsKilled: 85, MutationScore: 0.85}},
		{"IntegrationResult", IntegrationResult{FilesIntegrated: 5, PRNumber: 123}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.result)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if len(data) == 0 {
				t.Error("marshaled data should not be empty")
			}
		})
	}
}
