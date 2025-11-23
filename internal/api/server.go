package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QTest-hq/qtest/internal/api/ratelimit"
	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/db"
	gh "github.com/QTest-hq/qtest/internal/github"
	"github.com/QTest-hq/qtest/internal/jobs"
	qtestnats "github.com/QTest-hq/qtest/internal/nats"
	"github.com/QTest-hq/qtest/internal/webhook"
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

	// Rate limiter
	rateLimiter *ratelimit.RateLimiter

	// Organization handlers
	orgHandlers *OrganizationHandlers

	// API key handlers
	apiKeyHandlers *APIKeyHandlers

	// Audit log handlers
	auditHandlers *AuditHandlers

	// Webhook handlers
	webhookHandlers *WebhookHandlers
	webhookService  *webhook.Service
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, database *db.DB) (*Server, error) {
	store := db.NewStore(database)
	s := &Server{
		cfg:            cfg,
		router:         chi.NewRouter(),
		store:          store,
		repoService:    gh.NewRepoService("/tmp/qtest-repos", cfg.GitHubToken),
		orgHandlers:    NewOrganizationHandlers(store),
		apiKeyHandlers: NewAPIKeyHandlers(store),
		auditHandlers:  NewAuditHandlers(store),
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

	// Set up API key validator
	validator := &apiKeyValidatorWrapper{store: s.store}
	middleware.SetAPIKeyValidator(validator)

	log.Info().Msg("auth system configured with API key support")
}

// SetRateLimiter configures the rate limiting system
func (s *Server) SetRateLimiter(rl *ratelimit.RateLimiter) {
	s.rateLimiter = rl
	log.Info().Msg("rate limiter configured")
}

// SetWebhookService configures the webhook system
func (s *Server) SetWebhookService(svc *webhook.Service) {
	s.webhookService = svc
	s.webhookHandlers = NewWebhookHandlers(s.store, svc)
	log.Info().Msg("webhook service configured")
}

// GetWebhookService returns the webhook service for external use
func (s *Server) GetWebhookService() *webhook.Service {
	return s.webhookService
}

// apiKeyValidatorWrapper wraps db.Store to implement auth.APIKeyValidator
type apiKeyValidatorWrapper struct {
	store *db.Store
}

// ValidateAPIKeyForAuth implements auth.APIKeyValidator
func (w *apiKeyValidatorWrapper) ValidateAPIKeyForAuth(ctx context.Context, apiKey string) (*auth.APIKeyInfo, error) {
	key, err := w.store.ValidateAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, nil
	}

	return &auth.APIKeyInfo{
		ID:             key.ID,
		OrganizationID: key.OrganizationID,
		UserID:         key.UserID,
		Scopes:         key.Scopes,
	}, nil
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

	// Rate limiting middleware (added after auth to have access to user context)
	// Note: Actually applied via SetRateLimiter after router setup
}

// ApplyRateLimiting adds rate limiting middleware to the API routes
// Called after SetRateLimiter to ensure rate limiter is configured
func (s *Server) ApplyRateLimiting() {
	if s.rateLimiter != nil {
		// Apply rate limiting to /api/v1 routes
		s.router.Use(s.rateLimiter.Middleware())
		log.Info().Msg("rate limiting middleware applied")
	}
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

		// Repositories - require repos scope for API key access
		r.Route("/repos", func(r chi.Router) {
			r.Use(s.optionalAuth)
			r.With(s.requireScope("repos:write")).Post("/", s.createRepo)
			r.With(s.requireScope("repos:read")).Get("/", s.listRepos)
			r.With(s.requireScope("repos:read")).Get("/{repoID}", s.getRepo)
			r.With(s.requireScope("repos:write")).Delete("/{repoID}", s.deleteRepo)
			r.With(s.requireScope("jobs:read")).Get("/{repoID}/jobs", s.listRepoJobs)
		})

		// Generation runs - require runs scope for API key access
		r.Route("/repos/{repoID}/runs", func(r chi.Router) {
			r.Use(s.optionalAuth)
			r.With(s.requireScope("runs:write")).Post("/", s.createRun)
			r.With(s.requireScope("runs:read")).Get("/", s.listRuns)
			r.With(s.requireScope("runs:read")).Get("/{runID}", s.getRun)
			r.With(s.requireScope("tests:read")).Get("/{runID}/tests", s.getRunTests)
		})

		// Jobs - require jobs scope for API key access
		r.Route("/jobs", func(r chi.Router) {
			r.Use(s.optionalAuth)
			r.With(s.requireScope("jobs:write")).Post("/", s.createJob)
			r.With(s.requireScope("jobs:write")).Post("/pipeline", s.startPipeline)
			r.With(s.requireScope("jobs:read")).Get("/", s.listJobs)
			r.With(s.requireScope("jobs:read")).Get("/{jobID}", s.getJob)
			r.With(s.requireScope("jobs:write")).Post("/{jobID}/cancel", s.cancelJob)
			r.With(s.requireScope("jobs:write")).Post("/{jobID}/retry", s.retryJob)
		})

		// Tests - require tests scope for API key access
		r.Route("/tests", func(r chi.Router) {
			r.Use(s.optionalAuth)
			r.With(s.requireScope("tests:read")).Get("/", s.listTests)
			r.With(s.requireScope("tests:read")).Get("/{testID}", s.getTest)
			r.With(s.requireScope("tests:write")).Put("/{testID}/accept", s.acceptTest)
			r.With(s.requireScope("tests:write")).Put("/{testID}/reject", s.rejectTest)
		})

		// Mutation testing - require mutation scope for API key access
		r.Route("/mutation", func(r chi.Router) {
			r.Use(s.optionalAuth)
			r.With(s.requireScope("mutation:read")).Post("/", s.createMutationRun)
			r.With(s.requireScope("mutation:read")).Get("/", s.listMutationRuns)
			r.With(s.requireScope("mutation:read")).Get("/{mutationID}", s.getMutationRun)
		})

		// Repo-specific mutation runs
		r.With(s.optionalAuth, s.requireScope("mutation:read")).Get("/repos/{repoID}/mutation", s.listRepoMutationRuns)

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

			// Organization audit logs
			r.Get("/{orgID}/audit-logs", s.listOrgAuditLogs)

			// Organization webhooks
			r.Route("/{orgID}/webhooks", func(r chi.Router) {
				r.Post("/", s.createWebhook)
				r.Get("/", s.listWebhooks)
				r.Get("/{webhookID}", s.getWebhook)
				r.Patch("/{webhookID}", s.updateWebhook)
				r.Delete("/{webhookID}", s.deleteWebhook)
				r.Get("/{webhookID}/deliveries", s.listWebhookDeliveries)
				r.Post("/{webhookID}/test", s.sendTestWebhook)
			})
		})

		// User endpoints (requires auth)
		r.Route("/me", func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/audit-logs", s.listUserAuditLogs)
		})

		// API Keys (requires auth)
		r.Route("/api-keys", func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/", s.listAPIKeys)
			r.Post("/", s.createAPIKey)
			r.Get("/{keyID}", s.getAPIKey)
			r.Delete("/{keyID}", s.revokeAPIKey)
		})
	})
}

// API key handlers - delegate to apiKeyHandlers

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	s.apiKeyHandlers.ListAPIKeys(w, r)
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	s.apiKeyHandlers.CreateAPIKey(w, r)
}

func (s *Server) getAPIKey(w http.ResponseWriter, r *http.Request) {
	s.apiKeyHandlers.GetAPIKey(w, r)
}

func (s *Server) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	s.apiKeyHandlers.RevokeAPIKey(w, r)
}

// Audit log handlers - delegate to auditHandlers

func (s *Server) listOrgAuditLogs(w http.ResponseWriter, r *http.Request) {
	s.auditHandlers.ListAuditLogs(w, r)
}

func (s *Server) listUserAuditLogs(w http.ResponseWriter, r *http.Request) {
	s.auditHandlers.ListUserAuditLogs(w, r)
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

// optionalAuth is middleware that adds auth info if present but doesn't require it
func (s *Server) optionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.authMiddleware == nil {
			next.ServeHTTP(w, r)
			return
		}
		s.authMiddleware.OptionalAuth(next).ServeHTTP(w, r)
	})
}

// requireScope returns middleware that checks API key scopes
// Session-based auth passes through (full access), API keys must have the required scope
func (s *Server) requireScope(scopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get API key info from context
			apiKeyInfo, isAPIKey := auth.GetAPIKeyFromContext(r.Context())

			// If not API key auth, allow through (session-based auth has full access)
			if !isAPIKey {
				next.ServeHTTP(w, r)
				return
			}

			// Check if API key has any of the required scopes
			if !apiKeyInfo.HasAnyScope(scopes...) {
				respondError(w, http.StatusForbidden,
					fmt.Sprintf("insufficient scope: requires one of [%s]", strings.Join(scopes, ", ")))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
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
	URL            string     `json:"url"`
	Branch         string     `json:"branch,omitempty"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"` // If not provided, uses personal org
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

	// Get user session (optional for backwards compatibility)
	session, hasSession := auth.GetSessionFromContext(r.Context())

	// Determine organization ID
	var orgID uuid.UUID
	var userID uuid.UUID

	if hasSession {
		userID = session.UserID

		if req.OrganizationID != nil {
			// Verify user has member access to the org
			isMember, err := s.store.IsMember(r.Context(), *req.OrganizationID, userID)
			if err != nil {
				log.Error().Err(err).Msg("failed to check membership")
				respondError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if !isMember {
				respondError(w, http.StatusForbidden, "not a member of this organization")
				return
			}
			orgID = *req.OrganizationID
		} else {
			// Use personal organization
			personalOrg, err := s.store.GetPersonalOrganization(r.Context(), userID)
			if err != nil || personalOrg == nil {
				log.Error().Err(err).Msg("failed to get personal organization")
				respondError(w, http.StatusInternalServerError, "failed to get personal organization")
				return
			}
			orgID = personalOrg.ID
		}
	}

	// Check if repo already exists for this org
	if hasSession {
		existing, _ := s.store.GetRepositoryByURLAndOrg(r.Context(), req.URL, orgID)
		if existing != nil {
			respondJSON(w, http.StatusOK, existing)
			return
		}
	} else {
		// Legacy: check global
		existing, _ := s.store.GetRepositoryByURL(r.Context(), req.URL)
		if existing != nil {
			respondJSON(w, http.StatusOK, existing)
			return
		}
	}

	// Create repository record
	repo := &db.Repository{
		URL:           req.URL,
		Name:          repoInfo.Name,
		Owner:         repoInfo.Owner,
		DefaultBranch: repoInfo.Branch,
	}

	if hasSession {
		// Use tenant-aware creation
		if err := s.store.CreateRepositoryForOrg(r.Context(), repo, orgID, userID); err != nil {
			log.Error().Err(err).Msg("failed to create repository")
			respondError(w, http.StatusInternalServerError, "failed to create repository")
			return
		}
	} else {
		// Legacy: create without tenant context
		if err := s.store.CreateRepository(r.Context(), repo); err != nil {
			log.Error().Err(err).Msg("failed to create repository")
			respondError(w, http.StatusInternalServerError, "failed to create repository")
			return
		}
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
	orgIDStr := r.URL.Query().Get("organization_id")

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Check for authenticated user
	session, hasSession := auth.GetSessionFromContext(r.Context())

	var repos []db.Repository
	var err error

	if hasSession {
		if orgIDStr != "" {
			// Filter by specific organization
			orgID, parseErr := uuid.Parse(orgIDStr)
			if parseErr != nil {
				respondError(w, http.StatusBadRequest, "invalid organization_id")
				return
			}

			// Verify user has access to this org
			isMember, memErr := s.store.IsMember(r.Context(), orgID, session.UserID)
			if memErr != nil {
				log.Error().Err(memErr).Msg("failed to check membership")
				respondError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if !isMember {
				respondError(w, http.StatusForbidden, "not a member of this organization")
				return
			}

			repos, err = s.store.ListRepositoriesByOrg(r.Context(), orgID, limit, offset)
		} else {
			// List all repos across user's organizations
			repos, err = s.store.ListRepositoriesForUser(r.Context(), session.UserID, limit, offset)
		}
	} else {
		// Legacy: list all repos (for backwards compatibility)
		repos, err = s.store.ListRepositories(r.Context(), limit, offset)
	}

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

	// Check for authenticated user
	session, hasSession := auth.GetSessionFromContext(r.Context())

	if hasSession {
		// Verify user can access this repository
		canAccess, accessErr := s.store.CanAccessRepository(r.Context(), session.UserID, repoID)
		if accessErr != nil {
			log.Error().Err(accessErr).Msg("failed to check repository access")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !canAccess {
			respondError(w, http.StatusForbidden, "you don't have access to this repository")
			return
		}
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

	// Check for authenticated user - require admin access for deletion
	session, hasSession := auth.GetSessionFromContext(r.Context())

	if hasSession && repo.OrganizationID != nil {
		// Require admin/owner permission to delete
		canManage, manageErr := s.store.CanManageOrg(r.Context(), *repo.OrganizationID, session.UserID)
		if manageErr != nil {
			log.Error().Err(manageErr).Msg("failed to check permissions")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !canManage {
			respondError(w, http.StatusForbidden, "you need admin permissions to delete this repository")
			return
		}
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

	// Check for authenticated user
	session, hasSession := auth.GetSessionFromContext(r.Context())
	if hasSession {
		canAccess, accessErr := s.store.CanAccessRepository(r.Context(), session.UserID, repoID)
		if accessErr != nil {
			log.Error().Err(accessErr).Msg("failed to check repository access")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !canAccess {
			respondError(w, http.StatusForbidden, "you don't have access to this repository")
			return
		}
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

	// Check for authenticated user
	session, hasSession := auth.GetSessionFromContext(r.Context())
	if hasSession {
		canAccess, accessErr := s.store.CanAccessRepository(r.Context(), session.UserID, repoID)
		if accessErr != nil {
			log.Error().Err(accessErr).Msg("failed to check repository access")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !canAccess {
			respondError(w, http.StatusForbidden, "you don't have access to this repository")
			return
		}
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

	// Check for authenticated user - verify access to the repository
	session, hasSession := auth.GetSessionFromContext(r.Context())
	if hasSession {
		canAccess, accessErr := s.store.CanAccessRepository(r.Context(), session.UserID, run.RepositoryID)
		if accessErr != nil {
			log.Error().Err(accessErr).Msg("failed to check repository access")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !canAccess {
			respondError(w, http.StatusForbidden, "you don't have access to this run")
			return
		}
	}

	respondJSON(w, http.StatusOK, run)
}

func (s *Server) getRunTests(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	// Get the run first to check access
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

	// Check for authenticated user
	session, hasSession := auth.GetSessionFromContext(r.Context())
	if hasSession {
		canAccess, accessErr := s.store.CanAccessRepository(r.Context(), session.UserID, run.RepositoryID)
		if accessErr != nil {
			log.Error().Err(accessErr).Msg("failed to check repository access")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !canAccess {
			respondError(w, http.StatusForbidden, "you don't have access to this run")
			return
		}
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

// Webhook handlers - delegate to webhookHandlers

func (s *Server) createWebhook(w http.ResponseWriter, r *http.Request) {
	if s.webhookHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "webhooks not configured")
		return
	}
	s.webhookHandlers.CreateWebhook(w, r)
}

func (s *Server) listWebhooks(w http.ResponseWriter, r *http.Request) {
	if s.webhookHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "webhooks not configured")
		return
	}
	s.webhookHandlers.ListWebhooks(w, r)
}

func (s *Server) getWebhook(w http.ResponseWriter, r *http.Request) {
	if s.webhookHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "webhooks not configured")
		return
	}
	s.webhookHandlers.GetWebhook(w, r)
}

func (s *Server) updateWebhook(w http.ResponseWriter, r *http.Request) {
	if s.webhookHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "webhooks not configured")
		return
	}
	s.webhookHandlers.UpdateWebhook(w, r)
}

func (s *Server) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	if s.webhookHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "webhooks not configured")
		return
	}
	s.webhookHandlers.DeleteWebhook(w, r)
}

func (s *Server) listWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	if s.webhookHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "webhooks not configured")
		return
	}
	s.webhookHandlers.ListDeliveries(w, r)
}

func (s *Server) sendTestWebhook(w http.ResponseWriter, r *http.Request) {
	if s.webhookHandlers == nil {
		respondError(w, http.StatusServiceUnavailable, "webhooks not configured")
		return
	}
	s.webhookHandlers.SendTestWebhook(w, r)
}
