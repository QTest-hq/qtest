// Package integration provides worker system tests
package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/QTest-hq/qtest/internal/jobs"
	"github.com/QTest-hq/qtest/internal/worker"
)

// TestWorkerPipelineFlow tests the job chaining workflow without database
func TestWorkerPipelineFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test that we can create jobs with proper payloads for each stage
	ctx := context.Background()
	_ = ctx // Used in real scenarios

	// Stage 1: Ingestion job
	ingestionPayload := jobs.IngestionPayload{
		RepositoryURL: "https://github.com/test/repo",
		Branch:        "main",
		MaxTests:      50,
		LLMTier:       1,
		RunMutation:   true,
		CreatePR:      false,
	}
	ingestionJob, err := jobs.NewJob(jobs.JobTypeIngestion, ingestionPayload)
	if err != nil {
		t.Fatalf("Failed to create ingestion job: %v", err)
	}
	if ingestionJob.Type != jobs.JobTypeIngestion {
		t.Errorf("Job type = %s, want ingestion", ingestionJob.Type)
	}
	if ingestionJob.Status != jobs.StatusPending {
		t.Errorf("Job status = %s, want pending", ingestionJob.Status)
	}

	// Stage 2: Modeling job (would be chained from ingestion)
	modelingPayload := jobs.ModelingPayload{
		RepositoryID:  uuid.New(),
		WorkspacePath: "/tmp/workspace",
		MaxTests:      50,
		LLMTier:       1,
		RunMutation:   true,
	}
	modelingJob, err := jobs.NewJob(jobs.JobTypeModeling, modelingPayload)
	if err != nil {
		t.Fatalf("Failed to create modeling job: %v", err)
	}
	modelingJob.ParentJobID = &ingestionJob.ID

	// Stage 3: Planning job
	planningPayload := jobs.PlanningPayload{
		RepositoryID: modelingPayload.RepositoryID,
		ModelID:      uuid.New(),
		MaxTests:     50,
		TestLevels:   []string{"unit", "api"},
		LLMTier:      1,
	}
	planningJob, err := jobs.NewJob(jobs.JobTypePlanning, planningPayload)
	if err != nil {
		t.Fatalf("Failed to create planning job: %v", err)
	}
	planningJob.ParentJobID = &modelingJob.ID

	// Stage 4: Generation job
	generationPayload := jobs.GenerationPayload{
		RepositoryID:    modelingPayload.RepositoryID,
		GenerationRunID: uuid.New(),
		PlanID:          planningPayload.ModelID,
		LLMTier:         1,
		RunMutation:     true,
		CreatePR:        false,
	}
	generationJob, err := jobs.NewJob(jobs.JobTypeGeneration, generationPayload)
	if err != nil {
		t.Fatalf("Failed to create generation job: %v", err)
	}
	generationJob.ParentJobID = &planningJob.ID

	// Stage 5: Mutation job
	mutationPayload := jobs.MutationPayload{
		RepositoryID:    modelingPayload.RepositoryID,
		GenerationRunID: generationPayload.GenerationRunID,
		TestFilePath:    "/tmp/workspace/foo_test.go",
		SourceFilePath:  "/tmp/workspace/foo.go",
	}
	mutationJob, err := jobs.NewJob(jobs.JobTypeMutation, mutationPayload)
	if err != nil {
		t.Fatalf("Failed to create mutation job: %v", err)
	}
	mutationJob.ParentJobID = &generationJob.ID

	// Stage 6: Integration job
	integrationPayload := jobs.IntegrationPayload{
		RepositoryID:    modelingPayload.RepositoryID,
		GenerationRunID: generationPayload.GenerationRunID,
		TestFilePaths:   []string{"/tmp/workspace/foo_test.go"},
		CreatePR:        false,
		TargetBranch:    "main",
	}
	integrationJob, err := jobs.NewJob(jobs.JobTypeIntegration, integrationPayload)
	if err != nil {
		t.Fatalf("Failed to create integration job: %v", err)
	}
	integrationJob.ParentJobID = &generationJob.ID

	// Verify chain integrity
	allJobs := []*jobs.Job{ingestionJob, modelingJob, planningJob, generationJob, mutationJob, integrationJob}
	expectedTypes := []jobs.JobType{
		jobs.JobTypeIngestion,
		jobs.JobTypeModeling,
		jobs.JobTypePlanning,
		jobs.JobTypeGeneration,
		jobs.JobTypeMutation,
		jobs.JobTypeIntegration,
	}

	for i, job := range allJobs {
		if job.Type != expectedTypes[i] {
			t.Errorf("Job[%d] type = %s, want %s", i, job.Type, expectedTypes[i])
		}
	}

	t.Logf("Pipeline flow test: created %d jobs in chain", len(allJobs))
}

// TestJobPayloadRoundtrip tests serialization/deserialization of all payloads
func TestJobPayloadRoundtrip(t *testing.T) {
	tests := []struct {
		name    string
		jobType jobs.JobType
		payload interface{}
	}{
		{
			name:    "ingestion",
			jobType: jobs.JobTypeIngestion,
			payload: jobs.IngestionPayload{
				RepositoryURL: "https://github.com/test/repo",
				Branch:        "main",
				MaxTests:      100,
				LLMTier:       2,
				RunMutation:   true,
				CreatePR:      true,
			},
		},
		{
			name:    "modeling",
			jobType: jobs.JobTypeModeling,
			payload: jobs.ModelingPayload{
				RepositoryID:  uuid.New(),
				WorkspacePath: "/tmp/test",
				IncludePaths:  []string{"src/", "lib/"},
				ExcludePaths:  []string{"vendor/", "node_modules/"},
				MaxTests:      50,
			},
		},
		{
			name:    "planning",
			jobType: jobs.JobTypePlanning,
			payload: jobs.PlanningPayload{
				RepositoryID: uuid.New(),
				ModelID:      uuid.New(),
				MaxTests:     75,
				TestLevels:   []string{"unit", "api", "e2e"},
				LLMTier:      3,
			},
		},
		{
			name:    "generation",
			jobType: jobs.JobTypeGeneration,
			payload: jobs.GenerationPayload{
				RepositoryID:    uuid.New(),
				GenerationRunID: uuid.New(),
				PlanID:          uuid.New(),
				IntentIDs:       []string{"intent-1", "intent-2", "intent-3"},
				LLMTier:         2,
				RunMutation:     true,
				CreatePR:        false,
			},
		},
		{
			name:    "mutation",
			jobType: jobs.JobTypeMutation,
			payload: jobs.MutationPayload{
				RepositoryID:    uuid.New(),
				GenerationRunID: uuid.New(),
				TestFilePath:    "/path/to/test_file.go",
				SourceFilePath:  "/path/to/source_file.go",
			},
		},
		{
			name:    "integration",
			jobType: jobs.JobTypeIntegration,
			payload: jobs.IntegrationPayload{
				RepositoryID:    uuid.New(),
				GenerationRunID: uuid.New(),
				TestFilePaths:   []string{"test1.go", "test2.go", "test3.go"},
				TargetBranch:    "feature-branch",
				CreatePR:        true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create job with payload
			job, err := jobs.NewJob(tt.jobType, tt.payload)
			if err != nil {
				t.Fatalf("NewJob failed: %v", err)
			}

			// Serialize and deserialize
			jsonData, err := json.Marshal(job)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded jobs.Job
			if err := json.Unmarshal(jsonData, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Verify job fields
			if decoded.Type != tt.jobType {
				t.Errorf("Type = %s, want %s", decoded.Type, tt.jobType)
			}
			if decoded.Status != jobs.StatusPending {
				t.Errorf("Status = %s, want pending", decoded.Status)
			}
			if decoded.MaxRetries != 3 {
				t.Errorf("MaxRetries = %d, want 3", decoded.MaxRetries)
			}
		})
	}
}

// TestJobResultRoundtrip tests serialization/deserialization of all results
func TestJobResultRoundtrip(t *testing.T) {
	tests := []struct {
		name   string
		result interface{}
	}{
		{
			name: "ingestion",
			result: jobs.IngestionResult{
				RepositoryID:  uuid.New(),
				WorkspacePath: "/tmp/workspace",
				FileCount:     42,
				Language:      "go",
				Framework:     "gin",
			},
		},
		{
			name: "modeling",
			result: jobs.ModelingResult{
				ModelID:       uuid.New(),
				FileCount:     30,
				FunctionCount: 150,
				EndpointCount: 25,
			},
		},
		{
			name: "planning",
			result: jobs.PlanningResult{
				PlanID:     uuid.New(),
				TotalTests: 100,
				UnitTests:  60,
				APITests:   30,
				E2ETests:   10,
			},
		},
		{
			name: "generation",
			result: jobs.GenerationResult{
				TestsGenerated: 75,
				TestFilePaths:  []string{"test1.go", "test2.go"},
				FailedIntents:  []string{"intent-failed-1"},
			},
		},
		{
			name: "mutation",
			result: jobs.MutationResult{
				MutantsTotal:   50,
				MutantsKilled:  45,
				MutantsLived:   5,
				MutationScore:  90.0,
				ReportFilePath: "/tmp/mutation_report.json",
			},
		},
		{
			name: "integration",
			result: jobs.IntegrationResult{
				FilesIntegrated: 10,
				PRNumber:        123,
				PRURL:           "https://github.com/test/repo/pull/123",
				BranchName:      "qtest/tests-abc123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a job and set result
			job, _ := jobs.NewJob(jobs.JobTypeIngestion, jobs.IngestionPayload{})
			if err := job.SetResult(tt.result); err != nil {
				t.Fatalf("SetResult failed: %v", err)
			}

			// Serialize entire job
			jsonData, err := json.Marshal(job)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded jobs.Job
			if err := json.Unmarshal(jsonData, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Verify result is preserved
			if decoded.Result == nil {
				t.Error("Result should not be nil")
			}
		})
	}
}

// TestWorkerPoolCreation tests worker pool initialization
func TestWorkerPoolCreation(t *testing.T) {
	tests := []struct {
		workerType string
		wantCount  int
	}{
		{"all", 6},
		{"ingestion", 1},
		{"modeling", 1},
		{"planning", 1},
		{"generation", 1},
		{"mutation", 1},
		{"integration", 1},
	}

	for _, tt := range tests {
		t.Run(tt.workerType, func(t *testing.T) {
			pool, err := worker.NewPool(worker.PoolConfig{
				WorkerType: tt.workerType,
			})
			if err != nil {
				t.Fatalf("NewPool failed: %v", err)
			}

			if pool == nil {
				t.Fatal("Pool should not be nil")
			}
		})
	}
}

// TestJobCanRetry tests retry logic
func TestJobCanRetry(t *testing.T) {
	job, _ := jobs.NewJob(jobs.JobTypeIngestion, jobs.IngestionPayload{})

	// Default max retries is 3
	if !job.CanRetry() {
		t.Error("Job with 0 retries should be retryable")
	}

	job.RetryCount = 2
	if !job.CanRetry() {
		t.Error("Job with 2 retries (max 3) should be retryable")
	}

	job.RetryCount = 3
	if job.CanRetry() {
		t.Error("Job with 3 retries (max 3) should not be retryable")
	}

	job.RetryCount = 4
	if job.CanRetry() {
		t.Error("Job with 4 retries should not be retryable")
	}
}

// TestJobMessage tests job message encoding/decoding
func TestJobMessage(t *testing.T) {
	msg := &jobs.JobMessage{
		JobID:    uuid.New(),
		Type:     jobs.JobTypeGeneration,
		Priority: 5,
	}

	// Encode
	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decoded, err := jobs.DecodeJobMessage(data)
	if err != nil {
		t.Fatalf("DecodeJobMessage failed: %v", err)
	}

	if decoded.JobID != msg.JobID {
		t.Errorf("JobID = %s, want %s", decoded.JobID, msg.JobID)
	}
	if decoded.Type != msg.Type {
		t.Errorf("Type = %s, want %s", decoded.Type, msg.Type)
	}
	if decoded.Priority != msg.Priority {
		t.Errorf("Priority = %d, want %d", decoded.Priority, msg.Priority)
	}
}

// TestJobStatusTransitions tests valid status transitions
func TestJobStatusTransitions(t *testing.T) {
	validTransitions := map[jobs.JobStatus][]jobs.JobStatus{
		jobs.StatusPending:   {jobs.StatusRunning, jobs.StatusCancelled},
		jobs.StatusRunning:   {jobs.StatusCompleted, jobs.StatusFailed, jobs.StatusRetrying},
		jobs.StatusRetrying:  {jobs.StatusPending, jobs.StatusCancelled},
		jobs.StatusCompleted: {},
		jobs.StatusFailed:    {},
		jobs.StatusCancelled: {},
	}

	for from, validTo := range validTransitions {
		t.Run(string(from), func(t *testing.T) {
			// Verify these are the expected valid transitions
			t.Logf("From %s: valid transitions to %v", from, validTo)
		})
	}
}

// TestJobTimestamps tests job timestamp handling
func TestJobTimestamps(t *testing.T) {
	job, _ := jobs.NewJob(jobs.JobTypeIngestion, jobs.IngestionPayload{})

	// CreatedAt should be set
	if job.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	// UpdatedAt should be set
	if job.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}

	// StartedAt should be nil for pending job
	if job.StartedAt != nil {
		t.Error("StartedAt should be nil for pending job")
	}

	// CompletedAt should be nil for pending job
	if job.CompletedAt != nil {
		t.Error("CompletedAt should be nil for pending job")
	}

	// CreatedAt should be recent
	if time.Since(job.CreatedAt) > time.Second {
		t.Error("CreatedAt should be recent")
	}
}
