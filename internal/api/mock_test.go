package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/QTest-hq/qtest/internal/jobs"
)

// MockJobRepository is a mock implementation for testing
// It implements the JobRepository interface defined in server.go
type MockJobRepository struct {
	jobs      map[uuid.UUID]*jobs.Job
	createErr error
	getErr    error
	listErr   error
}

// Compile-time check that MockJobRepository implements JobRepository
var _ JobRepository = (*MockJobRepository)(nil)

func NewMockJobRepository() *MockJobRepository {
	return &MockJobRepository{
		jobs: make(map[uuid.UUID]*jobs.Job),
	}
}

func (m *MockJobRepository) Create(ctx context.Context, job *jobs.Job) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.jobs[job.ID] = job
	return nil
}

func (m *MockJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*jobs.Job, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	job, ok := m.jobs[id]
	if !ok {
		return nil, nil
	}
	return job, nil
}

func (m *MockJobRepository) ListByStatus(ctx context.Context, status jobs.JobStatus, limit int) ([]*jobs.Job, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*jobs.Job
	for _, j := range m.jobs {
		if j.Status == status {
			result = append(result, j)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockJobRepository) ListPendingByType(ctx context.Context, jobType jobs.JobType, limit int) ([]*jobs.Job, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*jobs.Job
	for _, j := range m.jobs {
		if j.Type == jobType && j.Status == jobs.StatusPending {
			result = append(result, j)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockJobRepository) ListByRepository(ctx context.Context, repoID uuid.UUID, limit int) ([]*jobs.Job, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*jobs.Job
	for _, j := range m.jobs {
		if j.RepositoryID != nil && *j.RepositoryID == repoID {
			result = append(result, j)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockJobRepository) Cancel(ctx context.Context, jobID uuid.UUID) error {
	job, ok := m.jobs[jobID]
	if !ok {
		return nil
	}
	job.Status = jobs.StatusCancelled
	return nil
}

func (m *MockJobRepository) Retry(ctx context.Context, jobID uuid.UUID) error {
	job, ok := m.jobs[jobID]
	if !ok {
		return nil
	}
	job.Status = jobs.StatusPending
	job.RetryCount++
	return nil
}

// AddJob adds a test job to the mock repository
func (m *MockJobRepository) AddJob(job *jobs.Job) {
	m.jobs[job.ID] = job
}

// setupMockServer creates a test server with a mock job repository
func setupMockServer(mockRepo *MockJobRepository) *Server {
	server := &Server{
		jobRepo: mockRepo,
	}
	server.router = setupFullTestRouter(server)
	return server
}

// setupFullTestRouter creates a router with all routes for testing
func setupFullTestRouter(s *Server) *chi.Mux {
	router := chi.NewRouter()

	router.Get("/health", s.healthCheck)
	router.Get("/ready", s.readyCheck)

	router.Route("/api/v1", func(r chi.Router) {
		// Repositories
		r.Route("/repos", func(r chi.Router) {
			r.Post("/", s.createRepo)
			r.Get("/", s.listRepos)
			r.Get("/{repoID}", s.getRepo)
			r.Delete("/{repoID}", s.deleteRepo)
			r.Get("/{repoID}/jobs", s.listRepoJobs)
			r.Get("/{repoID}/mutation", s.listRepoMutationRuns)
		})

		// Jobs
		r.Route("/jobs", func(r chi.Router) {
			r.Post("/", s.createJob)
			r.Post("/pipeline", s.startPipeline)
			r.Get("/", s.listJobs)
			r.Get("/{jobID}", s.getJob)
			r.Post("/{jobID}/cancel", s.cancelJob)
			r.Post("/{jobID}/retry", s.retryJob)
		})

		// Tests
		r.Route("/tests", func(r chi.Router) {
			r.Get("/{testID}", s.getTest)
			r.Put("/{testID}/accept", s.acceptTest)
			r.Put("/{testID}/reject", s.rejectTest)
		})

		// Mutation
		r.Route("/mutation", func(r chi.Router) {
			r.Post("/", s.createMutationRun)
			r.Get("/", s.listMutationRuns)
			r.Get("/{mutationID}", s.getMutationRun)
		})
	})

	return router
}

// TestMockCreateMutationRun_Success tests successful mutation creation
func TestMockCreateMutationRun_Success(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	body := bytes.NewBufferString(`{
		"source_file_path": "calculator.go",
		"test_file_path": "calculator_test.go"
	}`)
	req := httptest.NewRequest("POST", "/api/v1/mutation/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("createMutationRun returned status %d, want %d", rr.Code, http.StatusCreated)
		t.Logf("Response: %s", rr.Body.String())
	}

	var resp MutationRunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.SourceFile != "calculator.go" {
		t.Errorf("SourceFile = %s, want 'calculator.go'", resp.SourceFile)
	}
	if resp.TestFile != "calculator_test.go" {
		t.Errorf("TestFile = %s, want 'calculator_test.go'", resp.TestFile)
	}
	if resp.Status != "pending" {
		t.Errorf("Status = %s, want 'pending'", resp.Status)
	}
}

// TestMockCreateMutationRun_WithValidOptionalFields tests mutation creation with valid optional IDs
func TestMockCreateMutationRun_WithValidOptionalFields(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	repoID := uuid.New()
	runID := uuid.New()

	body := bytes.NewBufferString(`{
		"source_file_path": "main.go",
		"test_file_path": "main_test.go",
		"repository_id": "` + repoID.String() + `",
		"generation_run_id": "` + runID.String() + `",
		"mode": "thorough"
	}`)
	req := httptest.NewRequest("POST", "/api/v1/mutation/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("createMutationRun returned status %d, want %d", rr.Code, http.StatusCreated)
	}

	var resp MutationRunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.RepositoryID == nil || *resp.RepositoryID != repoID {
		t.Error("RepositoryID mismatch")
	}
	if resp.GenerationRunID == nil || *resp.GenerationRunID != runID {
		t.Error("GenerationRunID mismatch")
	}
}

// TestMockCreateMutationRun_ValidationErrors tests validation with mock repo
func TestMockCreateMutationRun_ValidationErrors(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "missing source file",
			body:     `{"test_file_path": "test.go"}`,
			expected: "source_file_path is required",
		},
		{
			name:     "missing test file",
			body:     `{"source_file_path": "main.go"}`,
			expected: "test_file_path is required",
		},
		{
			name:     "invalid repository ID",
			body:     `{"source_file_path": "main.go", "test_file_path": "main_test.go", "repository_id": "not-uuid"}`,
			expected: "invalid repository_id",
		},
		{
			name:     "invalid generation run ID",
			body:     `{"source_file_path": "main.go", "test_file_path": "main_test.go", "generation_run_id": "bad-uuid"}`,
			expected: "invalid generation_run_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/mutation/", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			server.router.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", rr.Code)
			}

			var resp map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if resp["error"] != tt.expected {
				t.Errorf("error = %s, want %s", resp["error"], tt.expected)
			}
		})
	}
}

// TestMockGetMutationRun_Success tests getting an existing mutation run
func TestMockGetMutationRun_Success(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	// Create a mutation job
	jobID := uuid.New()
	payload, _ := json.Marshal(jobs.MutationPayload{
		SourceFilePath: "calc.go",
		TestFilePath:   "calc_test.go",
	})
	mockRepo.AddJob(&jobs.Job{
		ID:        jobID,
		Type:      jobs.JobTypeMutation,
		Status:    jobs.StatusPending,
		Payload:   payload,
		CreatedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", "/api/v1/mutation/"+jobID.String(), nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("getMutationRun returned status %d, want %d", rr.Code, http.StatusOK)
	}

	var resp MutationRunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.ID != jobID {
		t.Errorf("ID = %s, want %s", resp.ID, jobID)
	}
	if resp.SourceFile != "calc.go" {
		t.Errorf("SourceFile = %s, want 'calc.go'", resp.SourceFile)
	}
}

// TestMockGetMutationRun_NotFound tests getting a non-existent mutation run
func TestMockGetMutationRun_NotFound(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	req := httptest.NewRequest("GET", "/api/v1/mutation/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("getMutationRun returned status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// TestMockGetMutationRun_WrongJobType tests getting a non-mutation job via mutation endpoint
func TestMockGetMutationRun_WrongJobType(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	// Create a non-mutation job
	jobID := uuid.New()
	mockRepo.AddJob(&jobs.Job{
		ID:        jobID,
		Type:      jobs.JobTypeIngestion, // Not mutation
		Status:    jobs.StatusPending,
		CreatedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", "/api/v1/mutation/"+jobID.String(), nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("getMutationRun returned status %d, want %d (wrong job type)", rr.Code, http.StatusNotFound)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp["error"] != "not a mutation job" {
		t.Errorf("error = %s, want 'not a mutation job'", resp["error"])
	}
}

// TestMockGetMutationRun_InvalidUUID tests getting with invalid UUID
func TestMockGetMutationRun_InvalidUUID(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	req := httptest.NewRequest("GET", "/api/v1/mutation/invalid-uuid", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("getMutationRun returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestMockListMutationRuns_Empty tests listing when no mutation jobs exist
func TestMockListMutationRuns_Empty(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	req := httptest.NewRequest("GET", "/api/v1/mutation/", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("listMutationRuns returned status %d, want %d", rr.Code, http.StatusOK)
	}

	var resp []MutationRunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

// TestMockListMutationRuns_WithJobs tests listing with mutation jobs
func TestMockListMutationRuns_WithJobs(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	// Add mutation jobs
	for i := 0; i < 3; i++ {
		payload, _ := json.Marshal(jobs.MutationPayload{
			SourceFilePath: "file.go",
			TestFilePath:   "file_test.go",
		})
		mockRepo.AddJob(&jobs.Job{
			ID:        uuid.New(),
			Type:      jobs.JobTypeMutation,
			Status:    jobs.StatusPending,
			Payload:   payload,
			CreatedAt: time.Now(),
		})
	}

	req := httptest.NewRequest("GET", "/api/v1/mutation/", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("listMutationRuns returned status %d, want %d", rr.Code, http.StatusOK)
	}

	var resp []MutationRunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(resp) != 3 {
		t.Errorf("expected 3 items, got %d", len(resp))
	}
}

// TestMockListRepoMutationRuns tests listing mutation runs for a specific repo
func TestMockListRepoMutationRuns(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	repoID := uuid.New()
	otherRepoID := uuid.New()

	// Add mutation job for target repo
	payload, _ := json.Marshal(jobs.MutationPayload{
		SourceFilePath: "calc.go",
		TestFilePath:   "calc_test.go",
	})
	mockRepo.AddJob(&jobs.Job{
		ID:           uuid.New(),
		Type:         jobs.JobTypeMutation,
		Status:       jobs.StatusPending,
		RepositoryID: &repoID,
		Payload:      payload,
		CreatedAt:    time.Now(),
	})

	// Add mutation job for other repo
	mockRepo.AddJob(&jobs.Job{
		ID:           uuid.New(),
		Type:         jobs.JobTypeMutation,
		Status:       jobs.StatusPending,
		RepositoryID: &otherRepoID,
		Payload:      payload,
		CreatedAt:    time.Now(),
	})

	req := httptest.NewRequest("GET", "/api/v1/repos/"+repoID.String()+"/mutation", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("listRepoMutationRuns returned status %d, want %d", rr.Code, http.StatusOK)
	}

	var resp []MutationRunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(resp) != 1 {
		t.Errorf("expected 1 item for target repo, got %d", len(resp))
	}
}

// TestMockCreateJob_Success tests successful job creation
func TestMockCreateJob_Success(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	body := bytes.NewBufferString(`{
		"type": "ingestion",
		"payload": {"repository_url": "https://github.com/test/repo"}
	}`)
	req := httptest.NewRequest("POST", "/api/v1/jobs/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("createJob returned status %d, want %d", rr.Code, http.StatusCreated)
		t.Logf("Response: %s", rr.Body.String())
	}
}

// TestMockListJobs_Empty tests listing jobs when none exist
func TestMockListJobs_Empty(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	req := httptest.NewRequest("GET", "/api/v1/jobs/", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("listJobs returned status %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestMockCancelJob_Success tests successful job cancellation
func TestMockCancelJob_Success(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	// Add a job
	jobID := uuid.New()
	mockRepo.AddJob(&jobs.Job{
		ID:        jobID,
		Type:      jobs.JobTypeIngestion,
		Status:    jobs.StatusPending,
		CreatedAt: time.Now(),
	})

	req := httptest.NewRequest("POST", "/api/v1/jobs/"+jobID.String()+"/cancel", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("cancelJob returned status %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestMockRetryJob_Success tests successful job retry
func TestMockRetryJob_Success(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	// Add a failed job
	jobID := uuid.New()
	mockRepo.AddJob(&jobs.Job{
		ID:        jobID,
		Type:      jobs.JobTypeIngestion,
		Status:    jobs.StatusFailed,
		CreatedAt: time.Now(),
	})

	req := httptest.NewRequest("POST", "/api/v1/jobs/"+jobID.String()+"/retry", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("retryJob returned status %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestMockStartPipeline_NoPipeline tests pipeline start without pipeline configured
// Note: startPipeline requires s.pipeline which needs a real Repository (not interface)
// This test documents the expected behavior when pipeline is nil
func TestMockStartPipeline_NoPipeline(t *testing.T) {
	mockRepo := NewMockJobRepository()
	server := setupMockServer(mockRepo)

	body := bytes.NewBufferString(`{
		"repository_url": "https://github.com/test/repo",
		"branch": "main",
		"max_tests": 10,
		"llm_tier": 1
	}`)
	req := httptest.NewRequest("POST", "/api/v1/jobs/pipeline", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Without pipeline configured, returns 503
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("startPipeline returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestStartPipelineRequest_Validation tests the request structure validation
func TestStartPipelineRequest_Validation(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		valid bool
	}{
		{
			name:  "valid request",
			body:  `{"repository_url": "https://github.com/test/repo", "branch": "main"}`,
			valid: true,
		},
		{
			name:  "with all fields",
			body:  `{"repository_url": "https://github.com/test/repo", "branch": "develop", "max_tests": 50, "llm_tier": 2}`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req StartPipelineRequest
			err := json.Unmarshal([]byte(tt.body), &req)
			if tt.valid && err != nil {
				t.Errorf("expected valid JSON, got error: %v", err)
			}
			if tt.valid && req.RepositoryURL == "" {
				t.Error("RepositoryURL should be set")
			}
		})
	}
}
