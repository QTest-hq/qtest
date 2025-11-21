package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/db"
	gh "github.com/QTest-hq/qtest/internal/github"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Server represents the API server
type Server struct {
	cfg         *config.Config
	router      *chi.Mux
	store       *db.Store
	repoService *gh.RepoService
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, database *db.DB) (*Server, error) {
	s := &Server{
		cfg:         cfg,
		router:      chi.NewRouter(),
		store:       db.NewStore(database),
		repoService: gh.NewRepoService("/tmp/qtest-repos", cfg.GitHubToken),
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s, nil
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

	// API v1
	s.router.Route("/api/v1", func(r chi.Router) {
		// Repositories
		r.Route("/repos", func(r chi.Router) {
			r.Post("/", s.createRepo)
			r.Get("/", s.listRepos)
			r.Get("/{repoID}", s.getRepo)
			r.Delete("/{repoID}", s.deleteRepo)
		})

		// Generation runs
		r.Route("/repos/{repoID}/runs", func(r chi.Router) {
			r.Post("/", s.createRun)
			r.Get("/", s.listRuns)
			r.Get("/{runID}", s.getRun)
			r.Get("/{runID}/tests", s.getRunTests)
		})

		// Tests
		r.Route("/tests", func(r chi.Router) {
			r.Get("/{testID}", s.getTest)
			r.Put("/{testID}/accept", s.acceptTest)
			r.Put("/{testID}/reject", s.rejectTest)
		})
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
	// TODO: Check database, redis, nats connections
	respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
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
	// TODO: Implement delete
	respondError(w, http.StatusNotImplemented, "not implemented")
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
	// TODO: Implement list runs
	respondJSON(w, http.StatusOK, []interface{}{})
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
func (s *Server) getTest(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement get test
	respondError(w, http.StatusNotImplemented, "not implemented")
}

func (s *Server) acceptTest(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement accept test
	respondError(w, http.StatusNotImplemented, "not implemented")
}

func (s *Server) rejectTest(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement reject test
	respondError(w, http.StatusNotImplemented, "not implemented")
}
