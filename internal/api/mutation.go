package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/jobs"
	"github.com/QTest-hq/qtest/internal/mutation"
)

// CreateMutationRequest is the request body for creating a mutation test run
type CreateMutationRequest struct {
	SourceFilePath  string `json:"source_file_path"`
	TestFilePath    string `json:"test_file_path"`
	RepositoryID    string `json:"repository_id,omitempty"`
	GenerationRunID string `json:"generation_run_id,omitempty"`
	Mode            string `json:"mode,omitempty"` // "fast" or "thorough"
}

// MutationRunResponse is the API response for a mutation test run
type MutationRunResponse struct {
	ID              uuid.UUID              `json:"id"`
	Status          string                 `json:"status"`
	SourceFile      string                 `json:"source_file"`
	TestFile        string                 `json:"test_file"`
	RepositoryID    *uuid.UUID             `json:"repository_id,omitempty"`
	GenerationRunID *uuid.UUID             `json:"generation_run_id,omitempty"`
	Result          *MutationResultResponse `json:"result,omitempty"`
	CreatedAt       string                 `json:"created_at"`
	CompletedAt     *string                `json:"completed_at,omitempty"`
}

// MutationResultResponse is the API response for mutation test results
type MutationResultResponse struct {
	Total    int              `json:"total"`
	Killed   int              `json:"killed"`
	Survived int              `json:"survived"`
	Timeout  int              `json:"timeout"`
	Score    float64          `json:"score"`
	Quality  string           `json:"quality"` // "good", "acceptable", "poor"
	Mutants  []MutantResponse `json:"mutants,omitempty"`
}

// MutantResponse is the API response for an individual mutant
type MutantResponse struct {
	Line        int    `json:"line"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

// createMutationRun creates a new mutation testing run
func (s *Server) createMutationRun(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	var req CreateMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.SourceFilePath == "" {
		respondError(w, http.StatusBadRequest, "source_file_path is required")
		return
	}
	if req.TestFilePath == "" {
		respondError(w, http.StatusBadRequest, "test_file_path is required")
		return
	}

	// Parse optional IDs
	var repoID, runID *uuid.UUID
	if req.RepositoryID != "" {
		id, err := uuid.Parse(req.RepositoryID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid repository_id")
			return
		}
		repoID = &id
	}
	if req.GenerationRunID != "" {
		id, err := uuid.Parse(req.GenerationRunID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid generation_run_id")
			return
		}
		runID = &id
	}

	// Create mutation job payload
	payload := jobs.MutationPayload{
		SourceFilePath: req.SourceFilePath,
		TestFilePath:   req.TestFilePath,
	}
	if repoID != nil {
		payload.RepositoryID = *repoID
	}
	if runID != nil {
		payload.GenerationRunID = *runID
	}

	// Create the job
	job, err := jobs.NewJob(jobs.JobTypeMutation, payload)
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to create job")
		return
	}
	job.RepositoryID = repoID
	job.GenerationRunID = runID

	if err := s.jobRepo.Create(r.Context(), job); err != nil {
		log.Error().Err(err).Msg("failed to create mutation job")
		respondError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	// Return response
	resp := &MutationRunResponse{
		ID:              job.ID,
		Status:          string(job.Status),
		SourceFile:      req.SourceFilePath,
		TestFile:        req.TestFilePath,
		RepositoryID:    repoID,
		GenerationRunID: runID,
		CreatedAt:       job.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	log.Info().
		Str("job_id", job.ID.String()).
		Str("source", req.SourceFilePath).
		Str("test", req.TestFilePath).
		Msg("created mutation job")

	respondJSON(w, http.StatusCreated, resp)
}

// getMutationRun gets a mutation run by ID
func (s *Server) getMutationRun(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	mutationID, err := uuid.Parse(chi.URLParam(r, "mutationID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid mutation ID")
		return
	}

	job, err := s.jobRepo.GetByID(r.Context(), mutationID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get mutation job")
		respondError(w, http.StatusInternalServerError, "failed to get job")
		return
	}

	if job == nil {
		respondError(w, http.StatusNotFound, "mutation run not found")
		return
	}

	// Verify it's a mutation job
	if job.Type != jobs.JobTypeMutation {
		respondError(w, http.StatusNotFound, "not a mutation job")
		return
	}

	resp := mutationJobToResponse(job)
	respondJSON(w, http.StatusOK, resp)
}

// listMutationRuns lists mutation runs with optional filters
func (s *Server) listMutationRuns(w http.ResponseWriter, r *http.Request) {
	if s.jobRepo == nil {
		respondError(w, http.StatusServiceUnavailable, "job system not available")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	status := r.URL.Query().Get("status")

	var jobList []*jobs.Job
	var err error

	if status != "" {
		// Get mutation jobs with specific status
		allJobs, err := s.jobRepo.ListByStatus(r.Context(), jobs.JobStatus(status), limit*2)
		if err == nil {
			for _, j := range allJobs {
				if j.Type == jobs.JobTypeMutation {
					jobList = append(jobList, j)
					if len(jobList) >= limit {
						break
					}
				}
			}
		}
	} else {
		// Get all mutation jobs
		allJobs, err := s.jobRepo.ListPendingByType(r.Context(), jobs.JobTypeMutation, limit)
		if err == nil {
			jobList = allJobs
		}
	}

	if err != nil {
		log.Error().Err(err).Msg("failed to list mutation jobs")
		respondError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	responses := make([]*MutationRunResponse, len(jobList))
	for i, j := range jobList {
		responses[i] = mutationJobToResponse(j)
	}

	respondJSON(w, http.StatusOK, responses)
}

// listRepoMutationRuns lists mutation runs for a specific repository
func (s *Server) listRepoMutationRuns(w http.ResponseWriter, r *http.Request) {
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

	// Get all jobs for repo and filter to mutation jobs
	allJobs, err := s.jobRepo.ListByRepository(r.Context(), repoID, limit*2)
	if err != nil {
		log.Error().Err(err).Msg("failed to list jobs")
		respondError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	var mutationJobs []*MutationRunResponse
	for _, j := range allJobs {
		if j.Type == jobs.JobTypeMutation {
			mutationJobs = append(mutationJobs, mutationJobToResponse(j))
			if len(mutationJobs) >= limit {
				break
			}
		}
	}

	respondJSON(w, http.StatusOK, mutationJobs)
}

// mutationJobToResponse converts a mutation job to API response
func mutationJobToResponse(job *jobs.Job) *MutationRunResponse {
	if job == nil {
		return nil
	}

	// Parse payload
	var payload jobs.MutationPayload
	job.GetPayload(&payload)

	resp := &MutationRunResponse{
		ID:              job.ID,
		Status:          string(job.Status),
		SourceFile:      payload.SourceFilePath,
		TestFile:        payload.TestFilePath,
		RepositoryID:    job.RepositoryID,
		GenerationRunID: job.GenerationRunID,
		CreatedAt:       job.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if job.CompletedAt != nil {
		s := job.CompletedAt.Format("2006-01-02T15:04:05Z")
		resp.CompletedAt = &s
	}

	// Parse result if completed
	if job.Status == jobs.StatusCompleted && len(job.Result) > 0 {
		var result jobs.MutationResult
		if err := job.GetResult(&result); err == nil {
			resp.Result = &MutationResultResponse{
				Total:    result.MutantsTotal,
				Killed:   result.MutantsKilled,
				Survived: result.MutantsLived,
				Score:    result.MutationScore,
				Quality:  getQualityLabel(result.MutationScore),
			}
		}
	}

	return resp
}

// getQualityLabel returns a quality label based on mutation score
func getQualityLabel(score float64) string {
	if score >= mutation.ThresholdGood {
		return "good"
	}
	if score >= mutation.ThresholdAcceptable {
		return "acceptable"
	}
	return "poor"
}
