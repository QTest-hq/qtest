package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/flow"
	"github.com/QTest-hq/qtest/internal/jobs"
)

// E2E API Request/Response types

// E2EDiscoverRequest is the request to start flow discovery
type E2EDiscoverRequest struct {
	URL           string `json:"url"`
	MaxPages      int    `json:"max_pages,omitempty"`
	PlaywrightURL string `json:"playwright_url,omitempty"`
}

// E2EGenerateRequest is the request to generate E2E tests
type E2EGenerateRequest struct {
	FlowID    string      `json:"flow_id,omitempty"`
	Flows     []flow.Flow `json:"flows,omitempty"`
	Framework string      `json:"framework,omitempty"` // playwright, cypress
	Language  string      `json:"language,omitempty"`  // typescript, javascript
	BaseURL   string      `json:"base_url,omitempty"`
	Enhance   bool        `json:"enhance,omitempty"`
}

// E2ERunRequest is the request to run E2E tests
type E2ERunRequest struct {
	TestDir  string `json:"test_dir"`
	Pattern  string `json:"pattern,omitempty"`
	Browser  string `json:"browser,omitempty"` // chromium, firefox, webkit
	Headless bool   `json:"headless,omitempty"`
	Workers  int    `json:"workers,omitempty"`
	Retries  int    `json:"retries,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
}

// E2EFlowResponse represents a flow in API responses
type E2EFlowResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	StartURL    string    `json:"start_url"`
	StepCount   int       `json:"step_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// E2ERunResponse represents an E2E run in API responses
type E2ERunResponse struct {
	ID          uuid.UUID       `json:"id"`
	Status      string          `json:"status"`
	TotalTests  int             `json:"total_tests"`
	Passed      int             `json:"passed"`
	Failed      int             `json:"failed"`
	Skipped     int             `json:"skipped"`
	Duration    string          `json:"duration,omitempty"`
	Browser     string          `json:"browser,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Results     json.RawMessage `json:"results,omitempty"`
}

// E2EGenerateResponse is the response for test generation
type E2EGenerateResponse struct {
	JobID     uuid.UUID `json:"job_id"`
	Status    string    `json:"status"`
	TestCount int       `json:"test_count,omitempty"`
	StepCount int       `json:"step_count,omitempty"`
	OutputDir string    `json:"output_dir,omitempty"`
	Files     []string  `json:"files,omitempty"`
}

// startE2EDiscovery starts a flow discovery job
func (s *Server) startE2EDiscovery(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	var req E2EDiscoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.URL == "" {
		respondError(w, http.StatusBadRequest, "url is required")
		return
	}

	// Set defaults
	if req.MaxPages == 0 {
		req.MaxPages = 5
	}
	if req.PlaywrightURL == "" {
		req.PlaywrightURL = "http://localhost:3000"
	}

	// Create job payload
	payload, _ := json.Marshal(map[string]interface{}{
		"url":            req.URL,
		"max_pages":      req.MaxPages,
		"playwright_url": req.PlaywrightURL,
	})

	// Create job
	job := &jobs.Job{
		ID:         uuid.New(),
		Type:       jobs.JobType("e2e_discovery"),
		Status:     jobs.StatusPending,
		Priority:   50,
		Payload:    payload,
		MaxRetries: 3,
	}

	if err := s.jobRepo.Create(r.Context(), job); err != nil {
		log.Error().Err(err).Msg("failed to create e2e discovery job")
		respondError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id": job.ID,
		"status": "pending",
	})
}

// generateE2ETests generates E2E tests from flows
func (s *Server) generateE2ETests(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	var req E2EGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FlowID == "" && len(req.Flows) == 0 {
		respondError(w, http.StatusBadRequest, "flow_id or flows is required")
		return
	}

	// Set defaults
	if req.Framework == "" {
		req.Framework = "playwright"
	}
	if req.Language == "" {
		req.Language = "typescript"
	}

	// Create job payload
	payload, _ := json.Marshal(map[string]interface{}{
		"flow_id":   req.FlowID,
		"flows":     req.Flows,
		"framework": req.Framework,
		"language":  req.Language,
		"base_url":  req.BaseURL,
		"enhance":   req.Enhance,
	})

	// Create job
	job := &jobs.Job{
		ID:         uuid.New(),
		Type:       jobs.JobType("e2e_generation"),
		Status:     jobs.StatusPending,
		Priority:   50,
		Payload:    payload,
		MaxRetries: 3,
	}

	if err := s.jobRepo.Create(r.Context(), job); err != nil {
		log.Error().Err(err).Msg("failed to create e2e generation job")
		respondError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(E2EGenerateResponse{
		JobID:  job.ID,
		Status: "pending",
	})
}

// startE2ERun starts an E2E test run
func (s *Server) startE2ERun(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	var req E2ERunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TestDir == "" {
		respondError(w, http.StatusBadRequest, "test_dir is required")
		return
	}

	// Set defaults
	if req.Browser == "" {
		req.Browser = "chromium"
	}
	if req.Workers == 0 {
		req.Workers = 4
	}
	if req.Retries == 0 {
		req.Retries = 2
	}

	// Create job payload
	payload, _ := json.Marshal(map[string]interface{}{
		"test_dir": req.TestDir,
		"pattern":  req.Pattern,
		"browser":  req.Browser,
		"headless": req.Headless,
		"workers":  req.Workers,
		"retries":  req.Retries,
		"base_url": req.BaseURL,
	})

	// Create job
	job := &jobs.Job{
		ID:         uuid.New(),
		Type:       jobs.JobType("e2e_run"),
		Status:     jobs.StatusPending,
		Priority:   60, // Higher priority for test runs
		Payload:    payload,
		MaxRetries: 1, // Test runs shouldn't auto-retry
	}

	if err := s.jobRepo.Create(r.Context(), job); err != nil {
		log.Error().Err(err).Msg("failed to create e2e run job")
		respondError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(E2ERunResponse{
		ID:        job.ID,
		Status:    "pending",
		Browser:   req.Browser,
		CreatedAt: time.Now(),
	})
}

// listE2ERuns lists E2E test runs
func (s *Server) listE2ERuns(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	// Parse query params
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit == 0 {
		limit = 20
	}

	// Get all jobs and filter by type
	jobList, err := s.jobRepo.ListRecent(r.Context(), limit*3) // Get more to account for filtering
	if err != nil {
		log.Error().Err(err).Msg("failed to list e2e runs")
		respondError(w, http.StatusInternalServerError, "failed to list runs")
		return
	}

	// Filter to e2e jobs
	runs := make([]E2ERunResponse, 0)
	for _, j := range jobList {
		if j.Type == "e2e_run" || j.Type == "e2e_generation" || j.Type == "e2e_discovery" {
			run := E2ERunResponse{
				ID:        j.ID,
				Status:    string(j.Status),
				CreatedAt: j.CreatedAt,
			}
			if j.CompletedAt != nil {
				run.CompletedAt = j.CompletedAt
			}
			if j.Result != nil {
				run.Results = *j.Result
			}
			runs = append(runs, run)
			if len(runs) >= limit {
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"runs":   runs,
		"total":  len(runs),
		"limit":  limit,
		"offset": offset,
	})
}

// getE2ERun gets a specific E2E test run
func (s *Server) getE2ERun(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	// Get job
	job, err := s.jobRepo.GetByID(r.Context(), runID)
	if err != nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}

	// Verify it's an E2E job
	if job.Type != "e2e_run" && job.Type != "e2e_generation" && job.Type != "e2e_discovery" {
		respondError(w, http.StatusBadRequest, "not an e2e job")
		return
	}

	run := E2ERunResponse{
		ID:        job.ID,
		Status:    string(job.Status),
		CreatedAt: job.CreatedAt,
	}

	if job.CompletedAt != nil {
		run.CompletedAt = job.CompletedAt
	}
	if job.Result != nil {
		run.Results = *job.Result
		// Parse results for test counts
		var result struct {
			TotalTests int    `json:"total_tests"`
			Passed     int    `json:"passed"`
			Failed     int    `json:"failed"`
			Skipped    int    `json:"skipped"`
			Duration   string `json:"duration"`
		}
		if err := json.Unmarshal(*job.Result, &result); err == nil {
			run.TotalTests = result.TotalTests
			run.Passed = result.Passed
			run.Failed = result.Failed
			run.Skipped = result.Skipped
			run.Duration = result.Duration
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}

// listE2EFlows lists available flows
func (s *Server) listE2EFlows(w http.ResponseWriter, r *http.Request) {
	// For now, return empty list - flows would be stored in DB
	// This is a placeholder for future implementation
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"flows": []E2EFlowResponse{},
		"total": 0,
	})
}

// uploadE2EFlow uploads a new flow specification
func (s *Server) uploadE2EFlow(w http.ResponseWriter, r *http.Request) {
	var flowSpec flow.Flow
	if err := json.NewDecoder(r.Body).Decode(&flowSpec); err != nil {
		respondError(w, http.StatusBadRequest, "invalid flow specification")
		return
	}

	if flowSpec.Name == "" {
		respondError(w, http.StatusBadRequest, "flow name is required")
		return
	}

	// Generate ID if not provided
	if flowSpec.ID == "" {
		flowSpec.ID = uuid.New().String()
	}

	// For now, just return the flow - storage would be implemented later
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(E2EFlowResponse{
		ID:          flowSpec.ID,
		Name:        flowSpec.Name,
		Type:        string(flowSpec.Type),
		Description: flowSpec.Description,
		StartURL:    flowSpec.StartURL,
		StepCount:   len(flowSpec.Steps),
		CreatedAt:   time.Now(),
	})
}
