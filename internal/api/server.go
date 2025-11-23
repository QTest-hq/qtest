package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/db"
	gh "github.com/QTest-hq/qtest/internal/github"
	"github.com/QTest-hq/qtest/internal/jobs"
	qtestnats "github.com/QTest-hq/qtest/internal/nats"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// JobRepository defines the interface for job storage operations
type JobRepository interface {
	Create(ctx context.Context, job *jobs.Job) error
	GetByID(ctx context.Context, id uuid.UUID) (*jobs.Job, error)
	ListByStatus(ctx context.Context, status jobs.JobStatus, limit int) ([]*jobs.Job, error)
	ListPendingByType(ctx context.Context, jobType jobs.JobType, limit int) ([]*jobs.Job, error)
	ListByRepository(ctx context.Context, repoID uuid.UUID, limit int) ([]*jobs.Job, error)
	ListRecent(ctx context.Context, limit int) ([]*jobs.Job, error)
	Cancel(ctx context.Context, jobID uuid.UUID) error
	Retry(ctx context.Context, jobID uuid.UUID) error
}

// Server represents the API server
type Server struct {
	cfg         *config.Config
	router      *chi.Mux
	store       *db.Store
	repoService *gh.RepoService
	nats        *qtestnats.Client
	jobRepo     JobRepository
	pipeline    *jobs.Pipeline

	// Auth components
	authHandlers   *auth.Handlers
	authMiddleware *auth.Middleware

	// Organization handlers
	orgHandlers *OrganizationHandlers
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, database *db.DB) (*Server, error) {
	store := db.NewStore(database)
	s := &Server{
		cfg:         cfg,
		router:      chi.NewRouter(),
		store:       store,
		repoService: gh.NewRepoService("/tmp/qtest-repos", cfg.GitHubToken),
		orgHandlers: NewOrganizationHandlers(store),
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s, nil
}

// SetJobSystem configures the job processing system
func (s *Server) SetJobSystem(jobRepo *jobs.Repository, natsClient *qtestnats.Client) {
	s.nats = natsClient
	s.jobRepo = jobRepo
	if jobRepo != nil {
		s.pipeline = jobs.NewPipeline(jobRepo, natsClient)
	}
}

// SetAuth configures the authentication system
func (s *Server) SetAuth(handlers *auth.Handlers, middleware *auth.Middleware) {
	s.authHandlers = handlers
	s.authMiddleware = middleware
	log.Info().Msg("auth system configured")
}

// Router returns the HTTP router
func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))
	s.router.Use(corsMiddleware)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) setupRoutes() {
	// Health check
	s.router.Get("/health", s.healthCheck)
	s.router.Get("/ready", s.readyCheck)

	// Auth routes (public)
	s.router.Route("/auth", func(r chi.Router) {
		r.Get("/login", s.handleLogin)
		r.Get("/callback", s.handleCallback)
		r.Post("/logout", s.handleLogout)
		r.Get("/logout", s.handleLogout) // Support GET for browser redirects
	})

	// API v1
	s.router.Route("/api/v1", func(r chi.Router) {
		// Auth - user info (requires auth)
		r.Route("/auth", func(r chi.Router) {
			r.Get("/me", s.handleMe)
			r.Post("/refresh", s.handleRefresh)
			r.Get("/repos", s.handleUserRepos)
		})

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
			r.Get("/", s.listTests)
			r.Get("/{testID}", s.getTest)
			r.Put("/{testID}/accept", s.acceptTest)
			r.Put("/{testID}/reject", s.rejectTest)
		})

		// Mutation testing
		r.Route("/mutation", func(r chi.Router) {
			r.Post("/", s.createMutationRun)
			r.Get("/", s.listMutationRuns)
			r.Get("/{mutationID}", s.getMutationRun)
		})

		// Repo-specific mutation runs
		r.Get("/repos/{repoID}/mutation", s.listRepoMutationRuns)

		// Organizations (requires auth)
		r.Route("/organizations", func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/", s.listOrganizations)
			r.Post("/", s.createOrganization)
			r.Get("/{orgID}", s.getOrganization)
			r.Patch("/{orgID}", s.updateOrganization)
			r.Delete("/{orgID}", s.deleteOrganization)

			// Organization members
			r.Get("/{orgID}/members", s.listOrgMembers)
			r.Post("/{orgID}/members", s.addOrgMember)
			r.Patch("/{orgID}/members/{userID}", s.updateMemberRole)
			r.Delete("/{orgID}/members/{userID}", s.removeOrgMember)
		})
	})
}

// requireAuth is middleware that requires authentication
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.authMiddleware == nil {
			respondError(w, http.StatusServiceUnavailable, "auth not configured")
			return
		}
		s.authMiddleware.RequireAuth(next).ServeHTTP(w, r)
	})
}

// Response helpers
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// Health check handlers
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checks := make(map[string]string)
	allHealthy := true

	// Check database
	if s.store != nil {
		if err := s.store.Ping(ctx); err != nil {
			checks["database"] = "unhealthy"
			allHealthy = false
		} else {
			checks["database"] = "healthy"
		}
	}

	// Check NATS
	if s.nats != nil {
		if err := s.nats.HealthCheck(); err != nil {
			checks["nats"] = "unhealthy"
			allHealthy = false
		} else {
			checks["nats"] = "healthy"
		}
	}

	if allHealthy {
		checks["status"] = "ready"
		respondJSON(w, http.StatusOK, checks)
	} else {
		checks["status"] = "not_ready"
		respondJSON(w, http.StatusServiceUnavailable, checks)
	}
}

// Repo handlers
type CreateRepoRequest struct {
	URL    string `json:"url"`
	Branch string `json:"branch,omitempty"`
}

func (s *Server) createRepo(w http.ResponseWriter, r *http.Request) {
	var req CreateRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.URL == "" {
		respondError(w, http.StatusBadRequest, "url is required")
		return
	}

	// Parse the GitHub URL
	repoInfo, err := gh.ParseRepoURL(req.URL)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Branch != "" {
		repoInfo.Branch = req.Branch
	}

	// Check if repo already exists
	existing, _ := s.store.GetRepositoryByURL(r.Context(), req.URL)
	if existing != nil {
		respondJSON(w, http.StatusOK, existing)
		return
	}

	// Create repository record
	repo := &db.Repository{
		URL:           req.URL,
		Name:          repoInfo.Name,
		Owner:         repoInfo.Owner,
		DefaultBranch: repoInfo.Branch,
	}

	if err := s.store.CreateRepository(r.Context(), repo); err != nil {
		log.Error().Err(err).Msg("failed to create repository")
		respondError(w, http.StatusInternalServerError, "failed to create repository")
		return
	}

	// Clone the repository asynchronously
	go s.cloneRepository(repo.ID, repoInfo)

	respondJSON(w, http.StatusCreated, repo)
}

func (s *Server) cloneRepository(repoID uuid.UUID, info *gh.RepoInfo) {
	ctx := context.Background()

	// Update status to cloning
	s.store.UpdateRepositoryStatus(ctx, repoID, "cloning", nil)

	// Clone
	result, err := s.repoService.Clone(ctx, info)
	if err != nil {
		log.Error().Err(err).Str("repo", info.URL).Msg("failed to clone repository")
		s.store.UpdateRepositoryStatus(ctx, repoID, "failed", nil)
		return
	}

	// Update status to ready
	s.store.UpdateRepositoryStatus(ctx, repoID, "ready", &result.CommitSHA)
	log.Info().Str("repo", info.Name).Str("commit", result.CommitSHA[:8]).Msg("repository cloned")
}

func (s *Server) listRepos(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	repos, err := s.store.ListRepositories(r.Context(), limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("failed to list repositories")
		respondError(w, http.StatusInternalServerError, "failed to list repositories")
		return
	}

	respondJSON(w, http.StatusOK, repos)
}

func (s *Server) getRepo(w http.ResponseWriter, r *http.Request) {
	repoID, err := uuid.Parse(chi.URLParam(r, "repoID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid repo ID")
		return
	}

	repo, err := s.store.GetRepository(r.Context(), repoID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get repository")
		respondError(w, http.StatusInternalServerError, "failed to get repository")
		return
	}

	if repo == nil {
		respondError(w, http.StatusNotFound, "repository not found")
		return
	}

	respondJSON(w, http.StatusOK, repo)
}

func (s *Server) deleteRepo(w http.ResponseWriter, r *http.Request) {
	repoID, err := uuid.Parse(chi.URLParam(r, "repoID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid repo ID")
		return
	}

	// Check if repo exists first
	repo, err := s.store.GetRepository(r.Context(), repoID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get repository")
		respondError(w, http.StatusInternalServerError, "failed to get repository")
		return
	}

	if repo == nil {
		respondError(w, http.StatusNotFound, "repository not found")
		return
	}

	// Delete the repository (cascades to runs and tests)
	if err := s.store.DeleteRepository(r.Context(), repoID); err != nil {
		log.Error().Err(err).Msg("failed to delete repository")
		respondError(w, http.StatusInternalServerError, "failed to delete repository")
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

// Run handlers
type CreateRunRequest struct {
	Tier     int                    `json:"tier,omitempty"`
	MaxTests int                    `json:"max_tests,omitempty"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	repoID, err := uuid.Parse(chi.URLParam(r, "repoID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid repo ID")
		return
	}

	// Verify repo exists and is ready
	repo, err := s.store.GetRepository(r.Context(), repoID)
	if err != nil || repo == nil {
		respondError(w, http.StatusNotFound, "repository not found")
		return
	}

	if repo.Status != "ready" {
		respondError(w, http.StatusBadRequest, "repository not ready")
		return
	}

	var req CreateRunRequest
	json.NewDecoder(r.Body).Decode(&req)

	configJSON, _ := json.Marshal(req.Config)

	run := &db.GenerationRun{
		RepositoryID: repoID,
		Config:       configJSON,
	}

	if err := s.store.CreateGenerationRun(r.Context(), run); err != nil {
		log.Error().Err(err).Msg("failed to create run")
		respondError(w, http.StatusInternalServerError, "failed to create run")
		return
	}

	// TODO: Queue the run for processing via NATS

	respondJSON(w, http.StatusCreated, run)
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	repoID, err := uuid.Parse(chi.URLParam(r, "repoID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid repo ID")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	runs, err := s.store.ListRunsByRepository(r.Context(), repoID, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("failed to list runs")
		respondError(w, http.StatusInternalServerError, "failed to list runs")
		return
	}

	respondJSON(w, http.StatusOK, runs)
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	run, err := s.store.GetGenerationRun(r.Context(), runID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get run")
		respondError(w, http.StatusInternalServerError, "failed to get run")
		return
	}

	if run == nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}

	respondJSON(w, http.StatusOK, run)
}

func (s *Server) getRunTests(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	tests, err := s.store.ListTestsByRun(r.Context(), runID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get tests")
		respondError(w, http.StatusInternalServerError, "failed to get tests")
		return
	}

	respondJSON(w, http.StatusOK, tests)
}

// Test handlers
func (s *Server) listTests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse optional run_id filter
	var runID *uuid.UUID
	if runIDStr := q.Get("run_id"); runIDStr != "" {
		parsed, err := uuid.Parse(runIDStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid run_id")
			return
		}
		runID = &parsed
	}

	// Parse status filter
	status := q.Get("status")

	// Parse limit (default 50)
	limit := 50
	if limitStr := q.Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 200 {
				limit = 200
			}
		}
	}

	tests, err := s.store.ListTests(r.Context(), runID, status, limit)
	if err != nil {
		log.Error().Err(err).Msg("failed to list tests")
		respondError(w, http.StatusInternalServerError, "failed to list tests")
		return
	}

	respondJSON(w, http.StatusOK, tests)
}

func (s *Server) getTest(w http.ResponseWriter, r *http.Request) {
	testID, err := uuid.Parse(chi.URLParam(r, "testID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid test ID")
		return
	}

	test, err := s.store.GetTest(r.Context(), testID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get test")
		respondError(w, http.StatusInternalServerError, "failed to get test")
		return
	}

	if test == nil {
		respondError(w, http.StatusNotFound, "test not found")
		return
	}

	respondJSON(w, http.StatusOK, test)
}

type RejectTestRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) acceptTest(w http.ResponseWriter, r *http.Request) {
	testID, err := uuid.Parse(chi.URLParam(r, "testID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid test ID")
		return
	}

	// Verify test exists
	test, err := s.store.GetTest(r.Context(), testID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get test")
		respondError(w, http.StatusInternalServerError, "failed to get test")
		return
	}

	if test == nil {
		respondError(w, http.StatusNotFound, "test not found")
		return
	}

	// Update status to accepted
	if err := s.store.UpdateTestStatus(r.Context(), testID, "accepted", nil); err != nil {
		log.Error().Err(err).Msg("failed to accept test")
		respondError(w, http.StatusInternalServerError, "failed to accept test")
		return
	}

	// Return updated test
	test.Status = "accepted"
	respondJSON(w, http.StatusOK, test)
}

func (s *Server) rejectTest(w http.ResponseWriter, r *http.Request) {
	testID, err := uuid.Parse(chi.URLParam(r, "testID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid test ID")
		return
	}

	// Verify test exists
	test, err := s.store.GetTest(r.Context(), testID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get test")
		respondError(w, http.StatusInternalServerError, "failed to get test")
		return
	}

	if test == nil {
		respondError(w, http.StatusNotFound, "test not found")
		return
	}

	// Parse rejection reason
	var req RejectTestRequest
	json.NewDecoder(r.Body).Decode(&req)

	var reason *string
	if req.Reason != "" {
		reason = &req.Reason
	}

	// Update status to rejected
	if err := s.store.UpdateTestStatus(r.Context(), testID, "rejected", reason); err != nil {
		log.Error().Err(err).Msg("failed to reject test")
		respondError(w, http.StatusInternalServerError, "failed to reject test")
		return
	}

	// Return updated test
	test.Status = "rejected"
	test.RejectionReason = reason
	respondJSON(w, http.StatusOK, test)
}

// Auth handlers - delegate to auth package

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.authHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "auth not configured")
		return
	}
	s.authHandlers.HandleLogin(w, r)
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	if s.authHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "auth not configured")
		return
	}
	s.authHandlers.HandleCallback(w, r)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.authHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "auth not configured")
		return
	}
	s.authHandlers.HandleLogout(w, r)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if s.authHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "auth not configured")
		return
	}
	// Check auth middleware if configured
	if s.authMiddleware != nil {
		// Get session from request
		session, ok := auth.GetSessionFromContext(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		// Add session to context for handler
		ctx := r.Context()
		ctx = context.WithValue(ctx, auth.SessionKey, session)
		r = r.WithContext(ctx)
	}
	s.authHandlers.HandleMe(w, r)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if s.authHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "auth not configured")
		return
	}
	s.authHandlers.HandleRefresh(w, r)
}

func (s *Server) handleUserRepos(w http.ResponseWriter, r *http.Request) {
	if s.authHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "auth not configured")
		return
	}
	s.authHandlers.HandleListRepos(w, r)
}

// Organization handlers - delegate to orgHandlers

func (s *Server) listOrganizations(w http.ResponseWriter, r *http.Request) {
	s.orgHandlers.ListOrganizations(w, r)
}

func (s *Server) getOrganization(w http.ResponseWriter, r *http.Request) {
	s.orgHandlers.GetOrganization(w, r)
}

func (s *Server) createOrganization(w http.ResponseWriter, r *http.Request) {
	s.orgHandlers.CreateOrganization(w, r)
}

func (s *Server) updateOrganization(w http.ResponseWriter, r *http.Request) {
	s.orgHandlers.UpdateOrganization(w, r)
}

func (s *Server) deleteOrganization(w http.ResponseWriter, r *http.Request) {
	s.orgHandlers.DeleteOrganization(w, r)
}

func (s *Server) listOrgMembers(w http.ResponseWriter, r *http.Request) {
	s.orgHandlers.ListMembers(w, r)
}

func (s *Server) addOrgMember(w http.ResponseWriter, r *http.Request) {
	s.orgHandlers.AddMember(w, r)
}

func (s *Server) updateMemberRole(w http.ResponseWriter, r *http.Request) {
	s.orgHandlers.UpdateMemberRole(w, r)
}

func (s *Server) removeOrgMember(w http.ResponseWriter, r *http.Request) {
	s.orgHandlers.RemoveMember(w, r)
}
