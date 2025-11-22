package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// setupMutationTestRouter creates a router with mutation routes for testing
func setupMutationTestRouter(s *Server) *chi.Mux {
	router := chi.NewRouter()

	router.Route("/api/v1", func(r chi.Router) {
		// Mutation routes
		r.Route("/mutation", func(r chi.Router) {
			r.Post("/", s.createMutationRun)
			r.Get("/", s.listMutationRuns)
			r.Get("/{mutationID}", s.getMutationRun)
		})

		// Repo mutation routes
		r.Get("/repos/{repoID}/mutation", s.listRepoMutationRuns)
	})

	return router
}

// TestCreateMutationRun_NoJobSystem tests mutation creation without job system
func TestCreateMutationRun_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	body := bytes.NewBufferString(`{
		"source_file_path": "calculator.go",
		"test_file_path": "calculator_test.go"
	}`)
	req := httptest.NewRequest("POST", "/api/v1/mutation/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("createMutationRun returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "job system not available" {
		t.Errorf("error = %s, want 'job system not available'", resp["error"])
	}
}

// TestCreateMutationRun_InvalidJSON tests mutation creation with invalid JSON
// Note: The handler checks for nil jobRepo first, so without job system this returns 503
func TestCreateMutationRun_InvalidJSON(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/api/v1/mutation/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Without jobRepo, returns 503 before parsing JSON
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("createMutationRun returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestCreateMutationRun_MissingSourceFile tests mutation creation without source file
// Note: Without jobRepo, validation is skipped - test documents behavior
func TestCreateMutationRun_MissingSourceFile(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	body := bytes.NewBufferString(`{
		"test_file_path": "calculator_test.go"
	}`)
	req := httptest.NewRequest("POST", "/api/v1/mutation/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Without jobRepo, returns 503 before validation
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("createMutationRun returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestCreateMutationRun_MissingTestFile tests mutation creation without test file
// Note: Without jobRepo, validation is skipped - test documents behavior
func TestCreateMutationRun_MissingTestFile(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	body := bytes.NewBufferString(`{
		"source_file_path": "calculator.go"
	}`)
	req := httptest.NewRequest("POST", "/api/v1/mutation/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Without jobRepo, returns 503 before validation
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("createMutationRun returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestCreateMutationRun_InvalidRepositoryID tests mutation creation with invalid repo ID
// Note: Without jobRepo, validation is skipped - test documents behavior
func TestCreateMutationRun_InvalidRepositoryID(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	body := bytes.NewBufferString(`{
		"source_file_path": "calculator.go",
		"test_file_path": "calculator_test.go",
		"repository_id": "not-a-valid-uuid"
	}`)
	req := httptest.NewRequest("POST", "/api/v1/mutation/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Without jobRepo, returns 503 before validation
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("createMutationRun returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestCreateMutationRun_InvalidGenerationRunID tests mutation creation with invalid run ID
// Note: Without jobRepo, validation is skipped - test documents behavior
func TestCreateMutationRun_InvalidGenerationRunID(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	body := bytes.NewBufferString(`{
		"source_file_path": "calculator.go",
		"test_file_path": "calculator_test.go",
		"generation_run_id": "not-a-valid-uuid"
	}`)
	req := httptest.NewRequest("POST", "/api/v1/mutation/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Without jobRepo, returns 503 before validation
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("createMutationRun returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestGetMutationRun_NoJobSystem tests get mutation without job system
func TestGetMutationRun_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/mutation/00000000-0000-0000-0000-000000000001", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("getMutationRun returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestGetMutationRun_InvalidID tests get mutation with invalid UUID
// Note: jobRepo check happens first, so we get 503 before UUID validation
func TestGetMutationRun_InvalidID(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/mutation/invalid-uuid", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Without jobRepo, returns 503 before UUID validation
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("getMutationRun returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestListMutationRuns_NoJobSystem tests list mutations without job system
func TestListMutationRuns_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/mutation/", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("listMutationRuns returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestListMutationRuns_WithStatusFilter tests list mutations with status query param
func TestListMutationRuns_WithStatusFilter(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/mutation/?status=completed", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Still expect service unavailable since no job system
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("listMutationRuns returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestListMutationRuns_WithLimitParam tests list mutations with limit param
func TestListMutationRuns_WithLimitParam(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/mutation/?limit=50", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Still expect service unavailable since no job system
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("listMutationRuns returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestListRepoMutationRuns_NoJobSystem tests listing repo mutations without job system
func TestListRepoMutationRuns_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/repos/00000000-0000-0000-0000-000000000001/mutation", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("listRepoMutationRuns returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestListRepoMutationRuns_InvalidRepoID tests listing repo mutations with invalid repo ID
// Note: jobRepo check happens first, so we get 503 before UUID validation
func TestListRepoMutationRuns_InvalidRepoID(t *testing.T) {
	server := &Server{}
	server.router = setupMutationTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/repos/invalid-uuid/mutation", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Without jobRepo, returns 503 before UUID validation
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("listRepoMutationRuns returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestCreateMutationRequest_Fields tests CreateMutationRequest struct fields
func TestCreateMutationRequest_Fields(t *testing.T) {
	req := CreateMutationRequest{
		SourceFilePath:  "src/calculator.go",
		TestFilePath:    "src/calculator_test.go",
		RepositoryID:    "00000000-0000-0000-0000-000000000001",
		GenerationRunID: "00000000-0000-0000-0000-000000000002",
		Mode:            "thorough",
	}

	if req.SourceFilePath != "src/calculator.go" {
		t.Errorf("SourceFilePath = %s, want 'src/calculator.go'", req.SourceFilePath)
	}
	if req.TestFilePath != "src/calculator_test.go" {
		t.Errorf("TestFilePath = %s, want 'src/calculator_test.go'", req.TestFilePath)
	}
	if req.Mode != "thorough" {
		t.Errorf("Mode = %s, want 'thorough'", req.Mode)
	}
}

// TestCreateMutationRequest_JSON tests JSON unmarshaling of CreateMutationRequest
func TestCreateMutationRequest_JSON(t *testing.T) {
	jsonData := `{
		"source_file_path": "main.go",
		"test_file_path": "main_test.go",
		"repository_id": "550e8400-e29b-41d4-a716-446655440000",
		"mode": "fast"
	}`

	var req CreateMutationRequest
	if err := json.Unmarshal([]byte(jsonData), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SourceFilePath != "main.go" {
		t.Errorf("SourceFilePath = %s, want 'main.go'", req.SourceFilePath)
	}
	if req.TestFilePath != "main_test.go" {
		t.Errorf("TestFilePath = %s, want 'main_test.go'", req.TestFilePath)
	}
	if req.RepositoryID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("RepositoryID mismatch")
	}
	if req.Mode != "fast" {
		t.Errorf("Mode = %s, want 'fast'", req.Mode)
	}
}

// TestMutationRunResponse_Fields tests MutationRunResponse struct fields
func TestMutationRunResponse_Fields(t *testing.T) {
	resp := MutationRunResponse{
		Status:     "completed",
		SourceFile: "calculator.go",
		TestFile:   "calculator_test.go",
		CreatedAt:  "2024-01-15T10:30:00Z",
	}

	if resp.Status != "completed" {
		t.Errorf("Status = %s, want 'completed'", resp.Status)
	}
	if resp.SourceFile != "calculator.go" {
		t.Errorf("SourceFile = %s, want 'calculator.go'", resp.SourceFile)
	}
}

// TestMutationResultResponse_Fields tests MutationResultResponse struct fields
func TestMutationResultResponse_Fields(t *testing.T) {
	result := MutationResultResponse{
		Total:    100,
		Killed:   80,
		Survived: 15,
		Timeout:  5,
		Score:    0.80,
		Quality:  "good",
	}

	if result.Total != 100 {
		t.Errorf("Total = %d, want 100", result.Total)
	}
	if result.Killed != 80 {
		t.Errorf("Killed = %d, want 80", result.Killed)
	}
	if result.Score != 0.80 {
		t.Errorf("Score = %f, want 0.80", result.Score)
	}
	if result.Quality != "good" {
		t.Errorf("Quality = %s, want 'good'", result.Quality)
	}
}

// TestMutantResponse_Fields tests MutantResponse struct fields
func TestMutantResponse_Fields(t *testing.T) {
	mutant := MutantResponse{
		Line:        42,
		Type:        "arithmetic",
		Status:      "killed",
		Description: "Changed + to -",
	}

	if mutant.Line != 42 {
		t.Errorf("Line = %d, want 42", mutant.Line)
	}
	if mutant.Type != "arithmetic" {
		t.Errorf("Type = %s, want 'arithmetic'", mutant.Type)
	}
	if mutant.Status != "killed" {
		t.Errorf("Status = %s, want 'killed'", mutant.Status)
	}
}

// TestGetQualityLabel tests the quality label assignment based on score
// Thresholds: >= 0.70 is good, >= 0.50 is acceptable, < 0.50 is poor
func TestGetQualityLabel(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{0.90, "good"},     // >= 0.70
		{0.80, "good"},     // >= 0.70
		{0.70, "good"},     // boundary: >= 0.70
		{0.69, "acceptable"}, // < 0.70 but >= 0.50
		{0.60, "acceptable"},
		{0.50, "acceptable"}, // boundary: >= 0.50
		{0.49, "poor"},      // < 0.50
		{0.30, "poor"},
		{0.0, "poor"},
	}

	for _, tt := range tests {
		result := getQualityLabel(tt.score)
		if result != tt.expected {
			t.Errorf("getQualityLabel(%f) = %s, want %s", tt.score, result, tt.expected)
		}
	}
}

// TestMutationJobToResponse_NilJob tests mutationJobToResponse with nil input
func TestMutationJobToResponse_NilJob(t *testing.T) {
	resp := mutationJobToResponse(nil)
	if resp != nil {
		t.Error("mutationJobToResponse(nil) should return nil")
	}
}

// TestMutationResultResponse_JSON tests JSON marshaling of MutationResultResponse
func TestMutationResultResponse_JSON(t *testing.T) {
	result := MutationResultResponse{
		Total:    50,
		Killed:   45,
		Survived: 3,
		Timeout:  2,
		Score:    0.90,
		Quality:  "good",
		Mutants: []MutantResponse{
			{Line: 10, Type: "arithmetic", Status: "killed", Description: "test"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled MutationResultResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Total != 50 {
		t.Errorf("Total = %d, want 50", unmarshaled.Total)
	}
	if len(unmarshaled.Mutants) != 1 {
		t.Errorf("Mutants length = %d, want 1", len(unmarshaled.Mutants))
	}
}

// TestMutationRunResponse_JSON tests JSON marshaling of MutationRunResponse
func TestMutationRunResponse_JSON(t *testing.T) {
	completedAt := "2024-01-15T11:00:00Z"
	resp := MutationRunResponse{
		Status:      "completed",
		SourceFile:  "src/main.go",
		TestFile:    "src/main_test.go",
		CreatedAt:   "2024-01-15T10:30:00Z",
		CompletedAt: &completedAt,
		Result: &MutationResultResponse{
			Total:   20,
			Killed:  18,
			Score:   0.90,
			Quality: "good",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled MutationRunResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Status != "completed" {
		t.Errorf("Status = %s, want 'completed'", unmarshaled.Status)
	}
	if unmarshaled.CompletedAt == nil || *unmarshaled.CompletedAt != completedAt {
		t.Error("CompletedAt mismatch")
	}
	if unmarshaled.Result == nil {
		t.Error("Result should not be nil")
	}
}
