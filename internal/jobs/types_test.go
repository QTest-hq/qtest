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

// =============================================================================
// NewJob Edge Cases
// =============================================================================

func TestNewJob_AllJobTypes(t *testing.T) {
	tests := []struct {
		jobType JobType
		payload interface{}
	}{
		{JobTypeIngestion, IngestionPayload{RepositoryURL: "url"}},
		{JobTypeModeling, ModelingPayload{RepositoryID: uuid.New()}},
		{JobTypePlanning, PlanningPayload{RepositoryID: uuid.New()}},
		{JobTypeGeneration, GenerationPayload{RepositoryID: uuid.New()}},
		{JobTypeMutation, MutationPayload{TestFilePath: "test.go"}},
		{JobTypeIntegration, IntegrationPayload{CreatePR: true}},
	}

	for _, tt := range tests {
		t.Run(string(tt.jobType), func(t *testing.T) {
			job, err := NewJob(tt.jobType, tt.payload)
			if err != nil {
				t.Fatalf("NewJob() error: %v", err)
			}
			if job.Type != tt.jobType {
				t.Errorf("Type = %s, want %s", job.Type, tt.jobType)
			}
			if job.Status != StatusPending {
				t.Errorf("Status = %s, want pending", job.Status)
			}
		})
	}
}

func TestNewJob_InvalidPayload(t *testing.T) {
	// Channel can't be JSON marshaled
	_, err := NewJob(JobTypeIngestion, make(chan int))
	if err == nil {
		t.Error("NewJob() should error for unmarshalable payload")
	}
}

func TestNewJob_NilPayload(t *testing.T) {
	job, err := NewJob(JobTypeIngestion, nil)
	if err != nil {
		t.Fatalf("NewJob() error: %v", err)
	}
	if string(job.Payload) != "null" {
		t.Errorf("Payload = %s, want null", string(job.Payload))
	}
}

func TestNewJob_EmptyPayload(t *testing.T) {
	job, err := NewJob(JobTypeIngestion, struct{}{})
	if err != nil {
		t.Fatalf("NewJob() error: %v", err)
	}
	if job.Payload == nil {
		t.Error("Payload should not be nil")
	}
}

// =============================================================================
// Job SetPayload/GetPayload Edge Cases
// =============================================================================

func TestJob_SetPayload_InvalidType(t *testing.T) {
	job := &Job{ID: uuid.New()}
	err := job.SetPayload(make(chan int))
	if err == nil {
		t.Error("SetPayload() should error for unmarshalable type")
	}
}

func TestJob_GetPayload_InvalidJSON(t *testing.T) {
	job := &Job{
		ID:      uuid.New(),
		Payload: json.RawMessage(`{invalid json`),
	}
	var payload IngestionPayload
	err := job.GetPayload(&payload)
	if err == nil {
		t.Error("GetPayload() should error for invalid JSON")
	}
}

func TestJob_GetPayload_EmptyPayload(t *testing.T) {
	job := &Job{
		ID:      uuid.New(),
		Payload: json.RawMessage(`{}`),
	}
	var payload IngestionPayload
	err := job.GetPayload(&payload)
	if err != nil {
		t.Fatalf("GetPayload() error: %v", err)
	}
	if payload.RepositoryURL != "" {
		t.Errorf("RepositoryURL = %s, want empty", payload.RepositoryURL)
	}
}

// =============================================================================
// Job SetResult/GetResult Edge Cases
// =============================================================================

func TestJob_SetResult_InvalidType(t *testing.T) {
	job := &Job{ID: uuid.New()}
	err := job.SetResult(make(chan int))
	if err == nil {
		t.Error("SetResult() should error for unmarshalable type")
	}
}

func TestJob_GetResult_InvalidJSON(t *testing.T) {
	job := &Job{
		ID:     uuid.New(),
		Result: json.RawMessage(`{invalid}`),
	}
	var result IngestionResult
	err := job.GetResult(&result)
	if err == nil {
		t.Error("GetResult() should error for invalid JSON")
	}
}

func TestJob_GetResult_NilResult(t *testing.T) {
	job := &Job{
		ID:     uuid.New(),
		Result: nil,
	}
	var result IngestionResult
	err := job.GetResult(&result)
	if err == nil {
		t.Error("GetResult() should error for nil result")
	}
}

// =============================================================================
// Job CanRetry Edge Cases
// =============================================================================

func TestJob_CanRetry_ZeroMaxRetries(t *testing.T) {
	job := &Job{
		RetryCount: 0,
		MaxRetries: 0,
	}
	if job.CanRetry() {
		t.Error("CanRetry() should return false when MaxRetries is 0")
	}
}

func TestJob_CanRetry_NegativeRetryCount(t *testing.T) {
	job := &Job{
		RetryCount: -1,
		MaxRetries: 3,
	}
	// -1 < 3, so should be able to retry
	if !job.CanRetry() {
		t.Error("CanRetry() should return true when RetryCount is negative")
	}
}

// =============================================================================
// JobMessage Edge Cases
// =============================================================================

func TestDecodeJobMessage_InvalidJSON(t *testing.T) {
	_, err := DecodeJobMessage([]byte(`{invalid}`))
	if err == nil {
		t.Error("DecodeJobMessage() should error for invalid JSON")
	}
}

func TestDecodeJobMessage_EmptyJSON(t *testing.T) {
	msg, err := DecodeJobMessage([]byte(`{}`))
	if err != nil {
		t.Fatalf("DecodeJobMessage() error: %v", err)
	}
	if msg.JobID != uuid.Nil {
		t.Errorf("JobID = %s, want nil UUID", msg.JobID)
	}
	if msg.Type != "" {
		t.Errorf("Type = %s, want empty", msg.Type)
	}
}

func TestDecodeJobMessage_PartialJSON(t *testing.T) {
	data := []byte(`{"job_id": "550e8400-e29b-41d4-a716-446655440000", "priority": 5}`)
	msg, err := DecodeJobMessage(data)
	if err != nil {
		t.Fatalf("DecodeJobMessage() error: %v", err)
	}
	if msg.Priority != 5 {
		t.Errorf("Priority = %d, want 5", msg.Priority)
	}
	if msg.Type != "" {
		t.Errorf("Type = %s, want empty for partial JSON", msg.Type)
	}
}

func TestJobMessage_Encode_EmptyMessage(t *testing.T) {
	msg := &JobMessage{}
	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}
	if len(data) == 0 {
		t.Error("Encode() should return non-empty data")
	}
}

// =============================================================================
// Job Fields Tests
// =============================================================================

func TestJob_Fields(t *testing.T) {
	repoID := uuid.New()
	runID := uuid.New()
	parentID := uuid.New()
	workerID := "worker-1"
	errMsg := "test error"
	now := time.Now()

	job := &Job{
		ID:              uuid.New(),
		Type:            JobTypeGeneration,
		Status:          StatusRunning,
		Priority:        5,
		RepositoryID:    &repoID,
		GenerationRunID: &runID,
		ParentJobID:     &parentID,
		Payload:         json.RawMessage(`{}`),
		Result:          json.RawMessage(`{}`),
		ErrorMessage:    &errMsg,
		ErrorDetails:    json.RawMessage(`{"code": 500}`),
		RetryCount:      2,
		MaxRetries:      5,
		CreatedAt:       now,
		UpdatedAt:       now,
		StartedAt:       &now,
		CompletedAt:     &now,
		LockedUntil:     &now,
		WorkerID:        &workerID,
	}

	if job.Priority != 5 {
		t.Errorf("Priority = %d, want 5", job.Priority)
	}
	if *job.RepositoryID != repoID {
		t.Error("RepositoryID mismatch")
	}
	if *job.GenerationRunID != runID {
		t.Error("GenerationRunID mismatch")
	}
	if *job.ParentJobID != parentID {
		t.Error("ParentJobID mismatch")
	}
	if *job.ErrorMessage != errMsg {
		t.Errorf("ErrorMessage = %s, want %s", *job.ErrorMessage, errMsg)
	}
	if *job.WorkerID != workerID {
		t.Errorf("WorkerID = %s, want %s", *job.WorkerID, workerID)
	}
}

func TestJob_NilOptionalFields(t *testing.T) {
	job := &Job{
		ID:     uuid.New(),
		Type:   JobTypeIngestion,
		Status: StatusPending,
	}

	if job.RepositoryID != nil {
		t.Error("RepositoryID should be nil")
	}
	if job.GenerationRunID != nil {
		t.Error("GenerationRunID should be nil")
	}
	if job.ParentJobID != nil {
		t.Error("ParentJobID should be nil")
	}
	if job.ErrorMessage != nil {
		t.Error("ErrorMessage should be nil")
	}
	if job.StartedAt != nil {
		t.Error("StartedAt should be nil")
	}
	if job.CompletedAt != nil {
		t.Error("CompletedAt should be nil")
	}
	if job.LockedUntil != nil {
		t.Error("LockedUntil should be nil")
	}
	if job.WorkerID != nil {
		t.Error("WorkerID should be nil")
	}
}

// =============================================================================
// Payload Specific Field Tests
// =============================================================================

func TestIngestionPayload_AllFields(t *testing.T) {
	payload := IngestionPayload{
		RepositoryURL: "https://github.com/test/repo",
		Branch:        "develop",
		CommitHash:    "abc123def456",
		WorkspacePath: "/tmp/workspace",
	}

	if payload.RepositoryURL != "https://github.com/test/repo" {
		t.Errorf("RepositoryURL mismatch")
	}
	if payload.Branch != "develop" {
		t.Errorf("Branch = %s, want develop", payload.Branch)
	}
	if payload.CommitHash != "abc123def456" {
		t.Errorf("CommitHash = %s, want abc123def456", payload.CommitHash)
	}
	if payload.WorkspacePath != "/tmp/workspace" {
		t.Errorf("WorkspacePath = %s, want /tmp/workspace", payload.WorkspacePath)
	}
}

func TestModelingPayload_AllFields(t *testing.T) {
	repoID := uuid.New()
	payload := ModelingPayload{
		RepositoryID:  repoID,
		WorkspacePath: "/tmp/workspace",
		IncludePaths:  []string{"src/", "lib/"},
		ExcludePaths:  []string{"vendor/", "node_modules/"},
	}

	if payload.RepositoryID != repoID {
		t.Error("RepositoryID mismatch")
	}
	if len(payload.IncludePaths) != 2 {
		t.Errorf("len(IncludePaths) = %d, want 2", len(payload.IncludePaths))
	}
	if len(payload.ExcludePaths) != 2 {
		t.Errorf("len(ExcludePaths) = %d, want 2", len(payload.ExcludePaths))
	}
}

func TestPlanningPayload_AllFields(t *testing.T) {
	repoID := uuid.New()
	modelID := uuid.New()
	payload := PlanningPayload{
		RepositoryID: repoID,
		ModelID:      modelID,
		MaxTests:     500,
		TestLevels:   []string{"unit", "integration", "e2e"},
	}

	if payload.RepositoryID != repoID {
		t.Error("RepositoryID mismatch")
	}
	if payload.ModelID != modelID {
		t.Error("ModelID mismatch")
	}
	if payload.MaxTests != 500 {
		t.Errorf("MaxTests = %d, want 500", payload.MaxTests)
	}
	if len(payload.TestLevels) != 3 {
		t.Errorf("len(TestLevels) = %d, want 3", len(payload.TestLevels))
	}
}

func TestGenerationPayload_AllFields(t *testing.T) {
	repoID := uuid.New()
	runID := uuid.New()
	planID := uuid.New()
	payload := GenerationPayload{
		RepositoryID:    repoID,
		GenerationRunID: runID,
		PlanID:          planID,
		IntentIDs:       []string{"intent1", "intent2"},
		LLMTier:         3,
	}

	if payload.RepositoryID != repoID {
		t.Error("RepositoryID mismatch")
	}
	if payload.GenerationRunID != runID {
		t.Error("GenerationRunID mismatch")
	}
	if payload.PlanID != planID {
		t.Error("PlanID mismatch")
	}
	if len(payload.IntentIDs) != 2 {
		t.Errorf("len(IntentIDs) = %d, want 2", len(payload.IntentIDs))
	}
	if payload.LLMTier != 3 {
		t.Errorf("LLMTier = %d, want 3", payload.LLMTier)
	}
}

func TestMutationPayload_AllFields(t *testing.T) {
	repoID := uuid.New()
	runID := uuid.New()
	payload := MutationPayload{
		RepositoryID:    repoID,
		GenerationRunID: runID,
		TestFilePath:    "tests/math_test.go",
		SourceFilePath:  "src/math.go",
	}

	if payload.RepositoryID != repoID {
		t.Error("RepositoryID mismatch")
	}
	if payload.GenerationRunID != runID {
		t.Error("GenerationRunID mismatch")
	}
	if payload.TestFilePath != "tests/math_test.go" {
		t.Errorf("TestFilePath = %s", payload.TestFilePath)
	}
	if payload.SourceFilePath != "src/math.go" {
		t.Errorf("SourceFilePath = %s", payload.SourceFilePath)
	}
}

func TestIntegrationPayload_AllFields(t *testing.T) {
	repoID := uuid.New()
	runID := uuid.New()
	payload := IntegrationPayload{
		RepositoryID:    repoID,
		GenerationRunID: runID,
		TestFilePaths:   []string{"test1.go", "test2.go", "test3.go"},
		TargetBranch:    "feature/tests",
		CreatePR:        true,
	}

	if payload.RepositoryID != repoID {
		t.Error("RepositoryID mismatch")
	}
	if len(payload.TestFilePaths) != 3 {
		t.Errorf("len(TestFilePaths) = %d, want 3", len(payload.TestFilePaths))
	}
	if payload.TargetBranch != "feature/tests" {
		t.Errorf("TargetBranch = %s, want feature/tests", payload.TargetBranch)
	}
	if !payload.CreatePR {
		t.Error("CreatePR should be true")
	}
}

// =============================================================================
// Result Specific Field Tests
// =============================================================================

func TestIngestionResult_AllFields(t *testing.T) {
	repoID := uuid.New()
	result := IngestionResult{
		RepositoryID:  repoID,
		WorkspacePath: "/home/qtest/workspaces/abc123",
		FileCount:     150,
		Language:      "go",
		Framework:     "gin",
	}

	if result.RepositoryID != repoID {
		t.Error("RepositoryID mismatch")
	}
	if result.FileCount != 150 {
		t.Errorf("FileCount = %d, want 150", result.FileCount)
	}
	if result.Language != "go" {
		t.Errorf("Language = %s, want go", result.Language)
	}
	if result.Framework != "gin" {
		t.Errorf("Framework = %s, want gin", result.Framework)
	}
}

func TestModelingResult_AllFields(t *testing.T) {
	modelID := uuid.New()
	result := ModelingResult{
		ModelID:       modelID,
		FileCount:     75,
		FunctionCount: 200,
		EndpointCount: 25,
	}

	if result.ModelID != modelID {
		t.Error("ModelID mismatch")
	}
	if result.FileCount != 75 {
		t.Errorf("FileCount = %d, want 75", result.FileCount)
	}
	if result.FunctionCount != 200 {
		t.Errorf("FunctionCount = %d, want 200", result.FunctionCount)
	}
	if result.EndpointCount != 25 {
		t.Errorf("EndpointCount = %d, want 25", result.EndpointCount)
	}
}

func TestPlanningResult_AllFields(t *testing.T) {
	planID := uuid.New()
	result := PlanningResult{
		PlanID:     planID,
		TotalTests: 300,
		UnitTests:  200,
		APITests:   75,
		E2ETests:   25,
	}

	if result.PlanID != planID {
		t.Error("PlanID mismatch")
	}
	if result.TotalTests != 300 {
		t.Errorf("TotalTests = %d, want 300", result.TotalTests)
	}
	if result.UnitTests != 200 {
		t.Errorf("UnitTests = %d, want 200", result.UnitTests)
	}
	if result.APITests != 75 {
		t.Errorf("APITests = %d, want 75", result.APITests)
	}
	if result.E2ETests != 25 {
		t.Errorf("E2ETests = %d, want 25", result.E2ETests)
	}
}

func TestGenerationResult_AllFields(t *testing.T) {
	result := GenerationResult{
		TestsGenerated: 50,
		TestFilePaths:  []string{"test1.go", "test2.go"},
		FailedIntents:  []string{"intent_5", "intent_12"},
	}

	if result.TestsGenerated != 50 {
		t.Errorf("TestsGenerated = %d, want 50", result.TestsGenerated)
	}
	if len(result.TestFilePaths) != 2 {
		t.Errorf("len(TestFilePaths) = %d, want 2", len(result.TestFilePaths))
	}
	if len(result.FailedIntents) != 2 {
		t.Errorf("len(FailedIntents) = %d, want 2", len(result.FailedIntents))
	}
}

func TestMutationResult_AllFields(t *testing.T) {
	result := MutationResult{
		MutantsTotal:   200,
		MutantsKilled:  180,
		MutantsLived:   20,
		MutationScore:  90.0,
		ReportFilePath: "/artifacts/mutation-report.html",
	}

	if result.MutantsTotal != 200 {
		t.Errorf("MutantsTotal = %d, want 200", result.MutantsTotal)
	}
	if result.MutantsKilled != 180 {
		t.Errorf("MutantsKilled = %d, want 180", result.MutantsKilled)
	}
	if result.MutantsLived != 20 {
		t.Errorf("MutantsLived = %d, want 20", result.MutantsLived)
	}
	if result.MutationScore != 90.0 {
		t.Errorf("MutationScore = %f, want 90.0", result.MutationScore)
	}
	if result.ReportFilePath != "/artifacts/mutation-report.html" {
		t.Errorf("ReportFilePath = %s", result.ReportFilePath)
	}
}

func TestIntegrationResult_AllFields(t *testing.T) {
	result := IntegrationResult{
		FilesIntegrated: 10,
		PRNumber:        42,
		PRURL:           "https://github.com/test/repo/pull/42",
		BranchName:      "qtest/add-tests",
	}

	if result.FilesIntegrated != 10 {
		t.Errorf("FilesIntegrated = %d, want 10", result.FilesIntegrated)
	}
	if result.PRNumber != 42 {
		t.Errorf("PRNumber = %d, want 42", result.PRNumber)
	}
	if result.PRURL != "https://github.com/test/repo/pull/42" {
		t.Errorf("PRURL = %s", result.PRURL)
	}
	if result.BranchName != "qtest/add-tests" {
		t.Errorf("BranchName = %s, want qtest/add-tests", result.BranchName)
	}
}
