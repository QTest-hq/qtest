package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// setupAuthTestRouter creates a router with auth routes for testing
func setupAuthTestRouter(s *Server) *chi.Mux {
	router := chi.NewRouter()

	// Auth routes
	router.Route("/auth", func(r chi.Router) {
		r.Get("/login", s.handleLogin)
		r.Get("/callback", s.handleCallback)
		r.Post("/logout", s.handleLogout)
		r.Get("/logout", s.handleLogout)
	})

	router.Route("/api/v1/auth", func(r chi.Router) {
		r.Get("/me", s.handleMe)
		r.Post("/refresh", s.handleRefresh)
		r.Get("/repos", s.handleUserRepos)
	})

	return router
}

// TestHandleLogin_NoAuth tests login without auth configured
func TestHandleLogin_NoAuth(t *testing.T) {
	server := &Server{}
	server.router = setupAuthTestRouter(server)

	req := httptest.NewRequest("GET", "/auth/login", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("handleLogin returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "auth not configured" {
		t.Errorf("error = %s, want 'auth not configured'", resp["error"])
	}
}

// TestHandleCallback_NoAuth tests callback without auth configured
func TestHandleCallback_NoAuth(t *testing.T) {
	server := &Server{}
	server.router = setupAuthTestRouter(server)

	req := httptest.NewRequest("GET", "/auth/callback?code=test&state=test", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("handleCallback returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestHandleLogout_NoAuth tests logout without auth configured (POST)
func TestHandleLogout_NoAuth_POST(t *testing.T) {
	server := &Server{}
	server.router = setupAuthTestRouter(server)

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("handleLogout returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestHandleLogout_NoAuth tests logout without auth configured (GET)
func TestHandleLogout_NoAuth_GET(t *testing.T) {
	server := &Server{}
	server.router = setupAuthTestRouter(server)

	req := httptest.NewRequest("GET", "/auth/logout", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("handleLogout returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestHandleMe_NoAuth tests /me without auth configured
func TestHandleMe_NoAuth(t *testing.T) {
	server := &Server{}
	server.router = setupAuthTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("handleMe returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestHandleRefresh_NoAuth tests refresh without auth configured
func TestHandleRefresh_NoAuth(t *testing.T) {
	server := &Server{}
	server.router = setupAuthTestRouter(server)

	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("handleRefresh returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestHandleUserRepos_NoAuth tests user repos without auth configured
func TestHandleUserRepos_NoAuth(t *testing.T) {
	server := &Server{}
	server.router = setupAuthTestRouter(server)

	req := httptest.NewRequest("GET", "/api/v1/auth/repos", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("handleUserRepos returned status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestAuthEndpoints_RouteExists verifies auth routes are registered
func TestAuthEndpoints_RouteExists(t *testing.T) {
	server := &Server{}
	server.router = setupAuthTestRouter(server)

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/auth/login"},
		{"GET", "/auth/callback"},
		{"POST", "/auth/logout"},
		{"GET", "/auth/logout"},
		{"GET", "/api/v1/auth/me"},
		{"POST", "/api/v1/auth/refresh"},
		{"GET", "/api/v1/auth/repos"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			rr := httptest.NewRecorder()

			server.router.ServeHTTP(rr, req)

			// Should get 503 (auth not configured) not 404 (route not found)
			if rr.Code == http.StatusNotFound {
				t.Errorf("route %s %s returned 404, route not registered", route.method, route.path)
			}
		})
	}
}

// TestSetAuth_ConfiguresHandlers tests that SetAuth properly configures handlers
func TestSetAuth_ConfiguresHandlers(t *testing.T) {
	server := &Server{}

	if server.authHandlers != nil {
		t.Error("authHandlers should be nil before SetAuth")
	}

	// SetAuth with nil (should not panic)
	server.SetAuth(nil, nil)

	// authHandlers is still nil since we passed nil
	if server.authHandlers != nil {
		t.Error("authHandlers should still be nil after SetAuth(nil, nil)")
	}
}
