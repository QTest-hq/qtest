package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/jobs"
	"github.com/google/uuid"
)

func TestJobToResponse(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-time.Minute)
	completedAt := now

	job := &jobs.Job{
		ID:              uuid.New(),
		Type:            jobs.JobTypeGeneration,
		Status:          jobs.StatusCompleted,
		Priority:        5,
		RepositoryID:    ptr(uuid.New()),
		GenerationRunID: ptr(uuid.New()),
		Payload:         json.RawMessage(`{"key": "value"}`),
		Result:          json.RawMessage(`{"tests": 10}`),
		RetryCount:      1,
		MaxRetries:      3,
		CreatedAt:       now.Add(-5 * time.Minute),
		UpdatedAt:       now,
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		WorkerID:        strPtr("worker-1"),
	}

	resp := jobToResponse(job)

	if resp.ID != job.ID {
		t.Errorf("ID mismatch")
	}
	if resp.Type != "generation" {
		t.Errorf("Type = %s, want generation", resp.Type)
	}
	if resp.Status != "completed" {
		t.Errorf("Status = %s, want completed", resp.Status)
	}
	if resp.Priority != 5 {
		t.Errorf("Priority = %d, want 5", resp.Priority)
	}
	if resp.RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", resp.RetryCount)
	}
	if resp.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", resp.MaxRetries)
	}
	if resp.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}
	if resp.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
	if resp.WorkerID == nil || *resp.WorkerID != "worker-1" {
		t.Errorf("WorkerID = %v, want worker-1", resp.WorkerID)
	}
}

func TestJobToResponse_NilJob(t *testing.T) {
	resp := jobToResponse(nil)
	if resp != nil {
		t.Error("expected nil response for nil job")
	}
}

func TestJobToResponse_MinimalJob(t *testing.T) {
	job := &jobs.Job{
		ID:        uuid.New(),
		Type:      jobs.JobTypeIngestion,
		Status:    jobs.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp := jobToResponse(job)

	if resp.Type != "ingestion" {
		t.Errorf("Type = %s, want ingestion", resp.Type)
	}
	if resp.Status != "pending" {
		t.Errorf("Status = %s, want pending", resp.Status)
	}
	if resp.StartedAt != nil {
		t.Error("StartedAt should be nil")
	}
	if resp.CompletedAt != nil {
		t.Error("CompletedAt should be nil")
	}
	if resp.RepositoryID != nil {
		t.Error("RepositoryID should be nil")
	}
}

func TestCreateJobRequest_Valid(t *testing.T) {
	req := CreateJobRequest{
		Type:     "ingestion",
		Priority: 10,
		Payload: map[string]interface{}{
			"repository_url": "https://github.com/test/repo",
			"branch":         "main",
		},
	}

	if req.Type != "ingestion" {
		t.Errorf("Type = %s, want ingestion", req.Type)
	}
	if req.Priority != 10 {
		t.Errorf("Priority = %d, want 10", req.Priority)
	}
	if req.Payload["repository_url"] != "https://github.com/test/repo" {
		t.Error("Payload mismatch")
	}
}

func TestStartPipelineRequest_Valid(t *testing.T) {
	req := StartPipelineRequest{
		RepositoryURL: "https://github.com/test/repo",
		Branch:        "develop",
		MaxTests:      100,
		LLMTier:       2,
		TestLevels:    []string{"unit", "api"},
		CreatePR:      true,
	}

	if req.RepositoryURL != "https://github.com/test/repo" {
		t.Errorf("RepositoryURL mismatch")
	}
	if req.Branch != "develop" {
		t.Errorf("Branch = %s, want develop", req.Branch)
	}
	if req.MaxTests != 100 {
		t.Errorf("MaxTests = %d, want 100", req.MaxTests)
	}
	if req.LLMTier != 2 {
		t.Errorf("LLMTier = %d, want 2", req.LLMTier)
	}
	if len(req.TestLevels) != 2 {
		t.Errorf("len(TestLevels) = %d, want 2", len(req.TestLevels))
	}
	if !req.CreatePR {
		t.Error("CreatePR should be true")
	}
}

func TestJobStatusResponse_Structure(t *testing.T) {
	parent := &JobResponse{
		ID:     uuid.New(),
		Type:   "ingestion",
		Status: "completed",
	}

	children := []*JobResponse{
		{ID: uuid.New(), Type: "modeling", Status: "completed"},
		{ID: uuid.New(), Type: "planning", Status: "running"},
	}

	resp := &JobStatusResponse{
		Job:      parent,
		Children: children,
	}

	if resp.Job == nil {
		t.Error("Job should not be nil")
	}
	if len(resp.Children) != 2 {
		t.Errorf("len(Children) = %d, want 2", len(resp.Children))
	}
}

func TestJobResponse_JSON(t *testing.T) {
	resp := &JobResponse{
		ID:         uuid.New(),
		Type:       "generation",
		Status:     "pending",
		Priority:   5,
		RetryCount: 0,
		MaxRetries: 3,
		CreatedAt:  "2024-01-01T00:00:00Z",
		UpdatedAt:  "2024-01-01T00:00:00Z",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed JobResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.ID != resp.ID {
		t.Errorf("ID mismatch after JSON roundtrip")
	}
	if parsed.Type != resp.Type {
		t.Errorf("Type mismatch after JSON roundtrip")
	}
}

// Helper functions
func ptr(u uuid.UUID) *uuid.UUID {
	return &u
}

func strPtr(s string) *string {
	return &s
}
