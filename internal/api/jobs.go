package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/jobs"
)

// CreateJobRequest is the request body for creating a job
type CreateJobRequest struct {
	Type     string                 `json:"type"`               // ingestion, modeling, planning, generation, mutation, integration
	Priority int                    `json:"priority,omitempty"` // Higher = more urgent
	Payload  map[string]interface{} `json:"payload"`
}

// StartPipelineRequest is the request body for starting a full pipeline
type StartPipelineRequest struct {
	RepositoryURL string   `json:"repository_url"`
	Branch        string   `json:"branch,omitempty"`
	MaxTests      int      `json:"max_tests,omitempty"`
	LLMTier       int      `json:"llm_tier,omitempty"` // 1=fast, 2=balanced, 3=thorough
	TestLevels    []string `json:"test_levels,omitempty"`
	RunMutation   bool     `json:"run_mutation,omitempty"`
	CreatePR      bool     `json:"create_pr,omitempty"`
}

// JobResponse is the API response for a job
type JobResponse struct {
	ID              uuid.UUID       `json:"id"`
	Type            string          `json:"type"`
	Status          string          `json:"status"`
	Priority        int             `json:"priority"`
	RepositoryID    *uuid.UUID      `json:"repository_id,omitempty"`
	GenerationRunID *uuid.UUID      `json:"generation_run_id,omitempty"`
	ParentJobID     *uuid.UUID      `json:"parent_job_id,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	Result          json.RawMessage `json:"result,omitempty"`
	ErrorMessage    *string         `json:"error_message,omitempty"`
	RetryCount      int             `json:"retry_count"`
	MaxRetries      int             `json:"max_retries"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	StartedAt       *string         `json:"started_at,omitempty"`
	CompletedAt     *string         `json:"completed_at,omitempty"`
	WorkerID        *string         `json:"worker_id,omitempty"`
}

// JobStatusResponse includes job and its children
type JobStatusResponse struct {
	Job      *JobResponse   `json:"job"`
	Children []*JobResponse `json:"children,omitempty"`
}

// jobToResponse converts a job to API response format
func jobToResponse(j *jobs.Job) *JobResponse {
	if j == nil {
		return nil
	}

	resp := &JobResponse{
		ID:              j.ID,
		Type:            string(j.Type),
		Status:          string(j.Status),
		Priority:        j.Priority,
		RepositoryID:    j.RepositoryID,
		GenerationRunID: j.GenerationRunID,
		ParentJobID:     j.ParentJobID,
		Payload:         j.Payload,
		ErrorMessage:    j.ErrorMessage,
		RetryCount:      j.RetryCount,
		MaxRetries:      j.MaxRetries,
		CreatedAt:       j.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       j.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		WorkerID:        j.WorkerID,
	}
	if j.Result != nil {
		resp.Result = *j.Result
	}

	if j.StartedAt != nil {
		s := j.StartedAt.Format("2006-01-02T15:04:05Z")
		resp.StartedAt = &s
	}
	if j.CompletedAt != nil {
		s := j.CompletedAt.Format("2006-01-02T15:04:05Z")
		resp.CompletedAt = &s
	}

	return resp
}

// startPipeline starts a full test generation pipeline
func (s *Server) startPipeline(w http.ResponseWriter, r *http.Request) {
	if s.pipeline == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	var req StartPipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RepositoryURL == "" {
		respondError(w, http.StatusBadRequest, "repository_url is required")
		return
	}

	options := jobs.PipelineOptions{
		Branch:      req.Branch,
		MaxTests:    req.MaxTests,
		LLMTier:     req.LLMTier,
		TestLevels:  req.TestLevels,
		RunMutation: req.RunMutation,
		CreatePR:    req.CreatePR,
	}

	job, err := s.pipeline.StartFullPipeline(r.Context(), req.RepositoryURL, options)
	if err != nil {
		log.Error().Err(err).Msg("failed to start pipeline")
		respondError(w, http.StatusInternalServerError, "failed to start pipeline")
		return
	}

	respondJSON(w, http.StatusCreated, jobToResponse(job))
}

// createJob creates a new job
func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate job type
	jobType := jobs.JobType(req.Type)
	switch jobType {
	case jobs.JobTypeIngestion, jobs.JobTypeModeling, jobs.JobTypePlanning,
		jobs.JobTypeGeneration, jobs.JobTypeMutation, jobs.JobTypeIntegration:
		// Valid
	default:
		respondError(w, http.StatusBadRequest, "invalid job type")
		return
	}

	job, err := jobs.NewJob(jobType, req.Payload)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	job.Priority = req.Priority

	if err := s.jobRepo.Create(r.Context(), job); err != nil {
		log.Error().Err(err).Msg("failed to create job")
		respondError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	// Publish to NATS if available
	if s.pipeline != nil {
		// Pipeline handles publishing internally
	}

	respondJSON(w, http.StatusCreated, jobToResponse(job))
}

// listJobs lists jobs with optional filters
func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	status := r.URL.Query().Get("status")
	jobType := r.URL.Query().Get("type")

	var jobList []*jobs.Job
	var err error

	if status != "" {
		jobList, err = s.jobRepo.ListByStatus(r.Context(), jobs.JobStatus(status), limit)
	} else if jobType != "" {
		jobList, err = s.jobRepo.ListPendingByType(r.Context(), jobs.JobType(jobType), limit)
	} else {
		// List all recent jobs by default
		jobList, err = s.jobRepo.ListRecent(r.Context(), limit)
	}

	if err != nil {
		log.Error().Err(err).Msg("failed to list jobs")
		respondError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	responses := make([]*JobResponse, len(jobList))
	for i, j := range jobList {
		responses[i] = jobToResponse(j)
	}

	respondJSON(w, http.StatusOK, responses)
}

// getJob gets a job by ID with its children
func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	if s.pipeline == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	report, err := s.pipeline.GetJobStatus(r.Context(), jobID)
	if err != nil {
		respondError(w, http.StatusNotFound, "job not found")
		return
	}

	children := make([]*JobResponse, len(report.Children))
	for i, c := range report.Children {
		children[i] = jobToResponse(c)
	}

	resp := &JobStatusResponse{
		Job:      jobToResponse(report.Job),
		Children: children,
	}

	respondJSON(w, http.StatusOK, resp)
}

// cancelJob cancels a pending job
func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	if err := s.jobRepo.Cancel(r.Context(), jobID); err != nil {
		log.Error().Err(err).Str("job_id", jobID.String()).Msg("failed to cancel job")
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// retryJob retries a failed job
func (s *Server) retryJob(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	if err := s.jobRepo.Retry(r.Context(), jobID); err != nil {
		log.Error().Err(err).Str("job_id", jobID.String()).Msg("failed to retry job")
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Re-fetch and return
	job, _ := s.jobRepo.GetByID(r.Context(), jobID)
	respondJSON(w, http.StatusOK, jobToResponse(job))
}

// listRepoJobs lists jobs for a specific repository
func (s *Server) listRepoJobs(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	repoID, err := uuid.Parse(chi.URLParam(r, "repoID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid repo ID")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	jobList, err := s.jobRepo.ListByRepository(r.Context(), repoID, limit)
	if err != nil {
		log.Error().Err(err).Msg("failed to list jobs")
		respondError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	responses := make([]*JobResponse, len(jobList))
	for i, j := range jobList {
		responses[i] = jobToResponse(j)
	}

	respondJSON(w, http.StatusOK, responses)
}
