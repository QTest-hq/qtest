package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHealthCheck(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("healthCheck returned status %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("status = %s, want ok", resp["status"])
	}
}

func TestReadyCheck(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("GET", "/ready", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("readyCheck returned status %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["status"] != "ready" {
		t.Errorf("status = %s, want ready", resp["status"])
	}
}

func TestCorsMiddleware(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("sets CORS headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("Access-Control-Allow-Origin header not set")
		}
		if rr.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Error("Access-Control-Allow-Methods header not set")
		}
		if rr.Header().Get("Access-Control-Allow-Headers") == "" {
			t.Error("Access-Control-Allow-Headers header not set")
		}
	})

	t.Run("OPTIONS request returns 200", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("OPTIONS returned status %d, want %d", rr.Code, http.StatusOK)
		}
	})
}

func TestRespondJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	data := map[string]string{"key": "value"}
	respondJSON(rr, http.StatusCreated, data)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Error("Content-Type should be application/json")
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp["key"] != "value" {
		t.Errorf("key = %s, want value", resp["key"])
	}
}

func TestRespondJSON_NilData(t *testing.T) {
	rr := httptest.NewRecorder()

	respondJSON(rr, http.StatusNoContent, nil)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	if rr.Body.Len() != 0 {
		t.Error("body should be empty for nil data")
	}
}

func TestRespondError(t *testing.T) {
	rr := httptest.NewRecorder()

	respondError(rr, http.StatusBadRequest, "invalid input")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp["error"] != "invalid input" {
		t.Errorf("error = %s, want 'invalid input'", resp["error"])
	}
}

func TestCreateRepoRequest_Fields(t *testing.T) {
	req := CreateRepoRequest{
		URL:    "https://github.com/test/repo",
		Branch: "develop",
	}

	if req.URL != "https://github.com/test/repo" {
		t.Errorf("URL mismatch")
	}
	if req.Branch != "develop" {
		t.Errorf("Branch = %s, want develop", req.Branch)
	}
}

func TestCreateRepoRequest_JSON(t *testing.T) {
	jsonData := `{"url": "https://github.com/test/repo", "branch": "main"}`

	var req CreateRepoRequest
	if err := json.Unmarshal([]byte(jsonData), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.URL != "https://github.com/test/repo" {
		t.Errorf("URL mismatch")
	}
	if req.Branch != "main" {
		t.Errorf("Branch = %s, want main", req.Branch)
	}
}

func TestCreateRunRequest_Fields(t *testing.T) {
	req := CreateRunRequest{
		Tier:     2,
		MaxTests: 50,
		Config: map[string]interface{}{
			"framework": "go",
		},
	}

	if req.Tier != 2 {
		t.Errorf("Tier = %d, want 2", req.Tier)
	}
	if req.MaxTests != 50 {
		t.Errorf("MaxTests = %d, want 50", req.MaxTests)
	}
	if req.Config["framework"] != "go" {
		t.Error("Config mismatch")
	}
}

func TestCreateRunRequest_JSON(t *testing.T) {
	jsonData := `{"tier": 3, "max_tests": 100, "config": {"language": "python"}}`

	var req CreateRunRequest
	if err := json.Unmarshal([]byte(jsonData), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.Tier != 3 {
		t.Errorf("Tier = %d, want 3", req.Tier)
	}
	if req.MaxTests != 100 {
		t.Errorf("MaxTests = %d, want 100", req.MaxTests)
	}
}

func TestCreateRepo_MissingURL(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	body := bytes.NewBufferString(`{"branch": "main"}`)
	req := httptest.NewRequest("POST", "/api/v1/repos/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("createRepo returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateRepo_InvalidJSON(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/api/v1/repos/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("createRepo returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestGetRepo_InvalidID(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/repos/invalid-uuid", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("getRepo returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestDeleteRepo_InvalidID(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("DELETE", "/api/v1/repos/invalid-uuid", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("deleteRepo with invalid ID returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateJob_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)
	// jobRepo is nil

	body := bytes.NewBufferString(`{"type": "ingestion", "payload": {}}`)
	req := httptest.NewRequest("POST", "/api/v1/jobs/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("createJob returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

func TestListJobs_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/jobs/", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("listJobs returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

func TestGetJob_InvalidID(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/jobs/invalid-uuid", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Should return bad request for invalid UUID
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("getJob returned status %d, want %d or %d", rr.Code, http.StatusBadRequest, http.StatusServiceUnavailable)
	}
}

func TestStartPipeline_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	body := bytes.NewBufferString(`{"repository_url": "https://github.com/test/repo"}`)
	req := httptest.NewRequest("POST", "/api/v1/jobs/pipeline", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("startPipeline returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

func TestCancelJob_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("POST", "/api/v1/jobs/00000000-0000-0000-0000-000000000001/cancel", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("cancelJob returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

func TestRetryJob_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("POST", "/api/v1/jobs/00000000-0000-0000-0000-000000000001/retry", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("retryJob returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

func TestGetTest_InvalidID(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/tests/invalid-uuid", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("getTest with invalid ID returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestAcceptTest_InvalidID(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("PUT", "/api/v1/tests/invalid-uuid/accept", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("acceptTest with invalid ID returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRejectTest_InvalidID(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("PUT", "/api/v1/tests/invalid-uuid/reject", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("rejectTest with invalid ID returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestGetRun_InvalidID(t *testing.T) {
	server := &Server{}
	server.router = setupTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/repos/00000000-0000-0000-0000-000000000001/runs/invalid-uuid", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("getRun returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// setupTestRouter creates a router for testing without database dependencies
func setupTestRouter(s *Server) *chi.Mux {
	router := chi.NewRouter()

	// Health check
	router.Get("/health", s.healthCheck)
	router.Get("/ready", s.readyCheck)

	// API v1
	router.Route("/api/v1", func(r chi.Router) {
		// Repositories
		r.Route("/repos", func(r chi.Router) {
			r.Post("/", s.createRepo)
			r.Get("/", s.listRepos)
			r.Get("/{repoID}", s.getRepo)
			r.Delete("/{repoID}", s.deleteRepo)
			r.Get("/{repoID}/jobs", s.listRepoJobs)
		})

		// Generation runs
		r.Route("/repos/{repoID}/runs", func(r chi.Router) {
			r.Post("/", s.createRun)
			r.Get("/", s.listRuns)
			r.Get("/{runID}", s.getRun)
			r.Get("/{runID}/tests", s.getRunTests)
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
	})

	return router
}
