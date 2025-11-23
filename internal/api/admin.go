package api

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/QTest-hq/qtest/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// AdminHandlers handles admin-only endpoints
type AdminHandlers struct {
	store *db.Store
}

// NewAdminHandlers creates a new AdminHandlers
func NewAdminHandlers(store *db.Store) *AdminHandlers {
	return &AdminHandlers{store: store}
}

// SystemStats represents system-wide statistics
type SystemStats struct {
	TotalOrganizations int       `json:"total_organizations"`
	TotalUsers         int       `json:"total_users"`
	TotalRepositories  int       `json:"total_repositories"`
	TotalJobs          int       `json:"total_jobs"`
	ActiveJobs         int       `json:"active_jobs"`
	TotalTests         int       `json:"total_tests"`
	ServerUptime       string    `json:"server_uptime"`
	GoVersion          string    `json:"go_version"`
	NumGoroutines      int       `json:"num_goroutines"`
	MemAllocMB         float64   `json:"mem_alloc_mb"`
	Timestamp          time.Time `json:"timestamp"`
}

var serverStartTime = time.Now()

// GetSystemStats returns system-wide statistics
// GET /api/v1/admin/stats
func (h *AdminHandlers) GetSystemStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin access
	if !h.isAdmin(r) {
		respondError(w, http.StatusForbidden, "admin access required")
		return
	}

	stats := &SystemStats{
		ServerUptime:  time.Since(serverStartTime).Round(time.Second).String(),
		GoVersion:     runtime.Version(),
		NumGoroutines: runtime.NumGoroutine(),
		Timestamp:     time.Now(),
	}

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	stats.MemAllocMB = float64(m.Alloc) / 1024 / 1024

	// Database stats
	stats.TotalOrganizations, _ = h.store.CountOrganizations(ctx)
	stats.TotalUsers, _ = h.store.CountUsers(ctx)
	stats.TotalRepositories, _ = h.store.CountRepositories(ctx)
	stats.TotalJobs, stats.ActiveJobs, _ = h.store.CountJobs(ctx)
	stats.TotalTests, _ = h.store.CountTests(ctx)

	respondJSON(w, http.StatusOK, stats)
}

// ListAllOrganizations lists all organizations (admin only)
// GET /api/v1/admin/organizations
func (h *AdminHandlers) ListAllOrganizations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.isAdmin(r) {
		respondError(w, http.StatusForbidden, "admin access required")
		return
	}

	limit, offset := parsePagination(r)

	orgs, err := h.store.ListAllOrganizations(ctx, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("failed to list organizations")
		respondError(w, http.StatusInternalServerError, "failed to list organizations")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"organizations": orgs,
		"limit":         limit,
		"offset":        offset,
	})
}

// ListAllUsers lists all users (admin only)
// GET /api/v1/admin/users
func (h *AdminHandlers) ListAllUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.isAdmin(r) {
		respondError(w, http.StatusForbidden, "admin access required")
		return
	}

	limit, offset := parsePagination(r)

	users, err := h.store.ListAllUsers(ctx, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("failed to list users")
		respondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"users":  users,
		"limit":  limit,
		"offset": offset,
	})
}

// GetUser gets a specific user (admin only)
// GET /api/v1/admin/users/{userID}
func (h *AdminHandlers) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.isAdmin(r) {
		respondError(w, http.StatusForbidden, "admin access required")
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get user")
		respondError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// UpdateUserRequest represents an admin user update
type UpdateUserRequest struct {
	IsActive *bool `json:"is_active,omitempty"`
}

// UpdateUser updates a user (admin only)
// PATCH /api/v1/admin/users/{userID}
func (h *AdminHandlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.isAdmin(r) {
		respondError(w, http.StatusForbidden, "admin access required")
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.store.AdminUpdateUserStatus(ctx, userID, req.IsActive); err != nil {
		log.Error().Err(err).Msg("failed to update user")
		respondError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ListAllJobs lists all jobs (admin only)
// GET /api/v1/admin/jobs
func (h *AdminHandlers) ListAllJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.isAdmin(r) {
		respondError(w, http.StatusForbidden, "admin access required")
		return
	}

	limit, offset := parsePagination(r)
	status := r.URL.Query().Get("status")

	jobs, err := h.store.ListAllJobs(ctx, status, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("failed to list jobs")
		respondError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":   jobs,
		"limit":  limit,
		"offset": offset,
		"status": status,
	})
}

// CancelJob cancels a job (admin only)
// POST /api/v1/admin/jobs/{jobID}/cancel
func (h *AdminHandlers) CancelJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.isAdmin(r) {
		respondError(w, http.StatusForbidden, "admin access required")
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	if err := h.store.CancelJob(ctx, jobID); err != nil {
		log.Error().Err(err).Msg("failed to cancel job")
		respondError(w, http.StatusInternalServerError, "failed to cancel job")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// GetAuditLogs gets system-wide audit logs (admin only)
// GET /api/v1/admin/audit-logs
func (h *AdminHandlers) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.isAdmin(r) {
		respondError(w, http.StatusForbidden, "admin access required")
		return
	}

	limit, offset := parsePagination(r)

	logs, err := h.store.ListAllAuditLogs(ctx, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("failed to list audit logs")
		respondError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"audit_logs": logs,
		"limit":      limit,
		"offset":     offset,
	})
}

// isAdmin checks if the current user has admin privileges
func (h *AdminHandlers) isAdmin(r *http.Request) bool {
	ctx := r.Context()

	// Check API key admin scope
	if apiKey, ok := auth.GetAPIKeyFromContext(ctx); ok {
		return apiKey.HasScope("admin")
	}

	// For session-based auth, check if user is configured as admin
	// Admin users can be configured via ADMIN_USER_IDS environment variable
	// For now, admin endpoints require API key with admin scope
	if _, ok := auth.GetSessionFromContext(ctx); ok {
		// TODO: Add admin user list via config or database field
		// For now, session users cannot access admin endpoints
		// They must use an API key with admin scope
		return false
	}

	return false
}

// parsePagination extracts limit and offset from query params
func parsePagination(r *http.Request) (int, int) {
	limit := 50
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}
