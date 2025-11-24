package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// setupE2ETestRouter creates a router with E2E routes for testing
func setupE2ETestRouter(s *Server) *chi.Mux {
	router := chi.NewRouter()

	router.Route("/api/v1", func(r chi.Router) {
		r.Route("/e2e", func(r chi.Router) {
			r.Post("/discover", s.startE2EDiscovery)
			r.Post("/generate", s.generateE2ETests)
			r.Post("/runs", s.startE2ERun)
			r.Get("/runs", s.listE2ERuns)
			r.Get("/runs/{runID}", s.getE2ERun)
			r.Get("/flows", s.listE2EFlows)
			r.Post("/flows", s.uploadE2EFlow)
		})
	})

	return router
}

// TestStartE2EDiscovery_NoJobSystem tests discovery without job system
func TestStartE2EDiscovery_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{"url": "https://example.com", "max_pages": 5}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/discover", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("startE2EDiscovery returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "job system not available" {
		t.Errorf("error = %s, want 'job system not available'", resp["error"])
	}
}

// TestStartE2EDiscovery_MissingURL tests discovery without URL
func TestStartE2EDiscovery_MissingURL(t *testing.T) {
	server := &Server{jobRepo: NewMockJobRepository()}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{"max_pages": 5}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/discover", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("startE2EDiscovery returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "url is required" {
		t.Errorf("error = %s, want 'url is required'", resp["error"])
	}
}

// TestStartE2EDiscovery_InvalidJSON tests discovery with invalid JSON
func TestStartE2EDiscovery_InvalidJSON(t *testing.T) {
	server := &Server{jobRepo: NewMockJobRepository()}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/discover", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("startE2EDiscovery returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestStartE2EDiscovery_Success tests successful discovery job creation
func TestStartE2EDiscovery_Success(t *testing.T) {
	server := &Server{jobRepo: NewMockJobRepository()}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{"url": "https://example.com", "max_pages": 5}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/discover", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("startE2EDiscovery returned status %d, want %d", rr.Code, http.StatusAccepted)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := resp["job_id"]; !ok {
		t.Error("response should contain job_id")
	}

	if resp["status"] != "pending" {
		t.Errorf("status = %s, want 'pending'", resp["status"])
	}
}

// TestGenerateE2ETests_NoJobSystem tests generation without job system
func TestGenerateE2ETests_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{"flows": [{"name": "test flow"}]}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/generate", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("generateE2ETests returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestGenerateE2ETests_MissingFlows tests generation without flows
func TestGenerateE2ETests_MissingFlows(t *testing.T) {
	server := &Server{jobRepo: NewMockJobRepository()}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{"framework": "playwright"}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/generate", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("generateE2ETests returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "flow_id or flows is required" {
		t.Errorf("error = %s, want 'flow_id or flows is required'", resp["error"])
	}
}

// TestGenerateE2ETests_Success tests successful generation job creation
func TestGenerateE2ETests_Success(t *testing.T) {
	server := &Server{jobRepo: NewMockJobRepository()}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{"flows": [{"name": "login flow", "type": "authentication"}], "framework": "playwright"}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/generate", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("generateE2ETests returned status %d, want %d", rr.Code, http.StatusAccepted)
	}

	var resp E2EGenerateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.JobID == uuid.Nil {
		t.Error("response should contain non-nil job_id")
	}

	if resp.Status != "pending" {
		t.Errorf("status = %s, want 'pending'", resp.Status)
	}
}

// TestStartE2ERun_NoJobSystem tests run without job system
func TestStartE2ERun_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{"test_dir": "./tests"}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/runs", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("startE2ERun returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestStartE2ERun_MissingTestDir tests run without test directory
func TestStartE2ERun_MissingTestDir(t *testing.T) {
	server := &Server{jobRepo: NewMockJobRepository()}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{"browser": "chromium"}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/runs", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("startE2ERun returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "test_dir is required" {
		t.Errorf("error = %s, want 'test_dir is required'", resp["error"])
	}
}

// TestStartE2ERun_Success tests successful run job creation
func TestStartE2ERun_Success(t *testing.T) {
	server := &Server{jobRepo: NewMockJobRepository()}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{"test_dir": "./tests", "browser": "chromium", "headless": true}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/runs", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("startE2ERun returned status %d, want %d", rr.Code, http.StatusAccepted)
	}

	var resp E2ERunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ID == uuid.Nil {
		t.Error("response should contain non-nil id")
	}

	if resp.Status != "pending" {
		t.Errorf("status = %s, want 'pending'", resp.Status)
	}

	if resp.Browser != "chromium" {
		t.Errorf("browser = %s, want 'chromium'", resp.Browser)
	}
}

// TestListE2ERuns_NoJobSystem tests listing runs without job system
func TestListE2ERuns_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupE2ETestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/e2e/runs", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("listE2ERuns returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestGetE2ERun_NoJobSystem tests getting run without job system
func TestGetE2ERun_NoJobSystem(t *testing.T) {
	server := &Server{}
	server.router = setupE2ETestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/e2e/runs/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("getE2ERun returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestGetE2ERun_InvalidID tests getting run with invalid ID
func TestGetE2ERun_InvalidID(t *testing.T) {
	server := &Server{jobRepo: NewMockJobRepository()}
	server.router = setupE2ETestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/e2e/runs/invalid-uuid", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("getE2ERun returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestListE2EFlows tests listing flows
func TestListE2EFlows(t *testing.T) {
	server := &Server{}
	server.router = setupE2ETestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/e2e/flows", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Currently returns empty list
	if rr.Code != http.StatusOK {
		t.Errorf("listE2EFlows returned status %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["total"].(float64) != 0 {
		t.Errorf("total = %v, want 0", resp["total"])
	}
}

// TestUploadE2EFlow_InvalidJSON tests uploading flow with invalid JSON
func TestUploadE2EFlow_InvalidJSON(t *testing.T) {
	server := &Server{}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/flows", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("uploadE2EFlow returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestUploadE2EFlow_MissingName tests uploading flow without name
func TestUploadE2EFlow_MissingName(t *testing.T) {
	server := &Server{}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{"type": "authentication"}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/flows", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("uploadE2EFlow returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "flow name is required" {
		t.Errorf("error = %s, want 'flow name is required'", resp["error"])
	}
}

// TestUploadE2EFlow_Success tests successful flow upload
func TestUploadE2EFlow_Success(t *testing.T) {
	server := &Server{}
	server.router = setupE2ETestRouter(server)

	body := bytes.NewBufferString(`{
		"name": "Login Flow",
		"type": "authentication",
		"description": "User login flow",
		"start_url": "https://example.com/login",
		"steps": [
			{"type": "navigate", "url": "https://example.com/login"},
			{"type": "fill", "selector": "#email", "value": "test@example.com"}
		]
	}`)
	req := httptest.NewRequest("POST", "/api/v1/e2e/flows", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("uploadE2EFlow returned status %d, want %d", rr.Code, http.StatusCreated)
	}

	var resp E2EFlowResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Name != "Login Flow" {
		t.Errorf("name = %s, want 'Login Flow'", resp.Name)
	}

	if resp.Type != "authentication" {
		t.Errorf("type = %s, want 'authentication'", resp.Type)
	}

	if resp.StepCount != 2 {
		t.Errorf("step_count = %d, want 2", resp.StepCount)
	}

	if resp.ID == "" {
		t.Error("response should contain non-empty id")
	}
}

// TestE2ERequestTypes tests request type field tags
func TestE2ERequestTypes(t *testing.T) {
	// Test E2EDiscoverRequest
	discoverReq := E2EDiscoverRequest{
		URL:           "https://example.com",
		MaxPages:      10,
		PlaywrightURL: "http://localhost:3000",
	}
	data, err := json.Marshal(discoverReq)
	if err != nil {
		t.Fatalf("failed to marshal E2EDiscoverRequest: %v", err)
	}
	if string(data) == "" {
		t.Error("E2EDiscoverRequest should marshal to non-empty JSON")
	}

	// Test E2EGenerateRequest
	genReq := E2EGenerateRequest{
		FlowID:    "flow-1",
		Framework: "playwright",
		Language:  "typescript",
		BaseURL:   "https://example.com",
		Enhance:   true,
	}
	data, err = json.Marshal(genReq)
	if err != nil {
		t.Fatalf("failed to marshal E2EGenerateRequest: %v", err)
	}
	if string(data) == "" {
		t.Error("E2EGenerateRequest should marshal to non-empty JSON")
	}

	// Test E2ERunRequest
	runReq := E2ERunRequest{
		TestDir:  "./tests",
		Pattern:  "*.spec.ts",
		Browser:  "chromium",
		Headless: true,
		Workers:  4,
		Retries:  2,
		BaseURL:  "https://example.com",
	}
	data, err = json.Marshal(runReq)
	if err != nil {
		t.Fatalf("failed to marshal E2ERunRequest: %v", err)
	}
	if string(data) == "" {
		t.Error("E2ERunRequest should marshal to non-empty JSON")
	}
}
