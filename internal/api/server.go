package api

import (
	"net/http"
	"time"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server represents the API server
type Server struct {
	cfg    *config.Config
	router *chi.Mux
}

// NewServer creates a new API server
func NewServer(cfg *config.Config) (*Server, error) {
	s := &Server{
		cfg:    cfg,
		router: chi.NewRouter(),
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

		// System model
		r.Get("/repos/{repoID}/model", s.getSystemModel)
	})
}

// Health check handlers
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) readyCheck(w http.ResponseWriter, r *http.Request) {
	// TODO: Check database, redis, nats connections
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

// Repo handlers (stubs)
func (s *Server) createRepo(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) listRepos(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) getRepo(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) deleteRepo(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}

// Run handlers (stubs)
func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) getRunTests(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}

// Test handlers (stubs)
func (s *Server) getTest(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) acceptTest(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) rejectTest(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}

// System model handler (stub)
func (s *Server) getSystemModel(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	w.WriteHeader(http.StatusNotImplemented)
}
