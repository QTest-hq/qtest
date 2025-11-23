package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/QTest-hq/qtest/internal/db"
)

// AuditHandlers handles audit log endpoints
type AuditHandlers struct {
	store *db.Store
}

// NewAuditHandlers creates new audit handlers
func NewAuditHandlers(store *db.Store) *AuditHandlers {
	return &AuditHandlers{store: store}
}

// AuditLogResponse is the API response for an audit log entry
type AuditLogResponse struct {
	ID             uuid.UUID   `json:"id"`
	OrganizationID *uuid.UUID  `json:"organization_id,omitempty"`
	UserID         *uuid.UUID  `json:"user_id,omitempty"`
	Action         string      `json:"action"`
	ResourceType   string      `json:"resource_type"`
	ResourceID     *uuid.UUID  `json:"resource_id,omitempty"`
	Details        interface{} `json:"details,omitempty"`
	IPAddress      *string     `json:"ip_address,omitempty"`
	UserAgent      *string     `json:"user_agent,omitempty"`
	CreatedAt      string      `json:"created_at"`
}

// ListAuditLogs lists audit logs for an organization
// GET /api/v1/organizations/{orgID}/audit-logs
func (h *AuditHandlers) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	orgID, err := uuid.Parse(chi.URLParam(r, "orgID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Check if user has admin access to the organization
	canManage, err := h.store.CanManageOrg(r.Context(), orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check permissions")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !canManage {
		writeError(w, http.StatusForbidden, "admin access required to view audit logs")
		return
	}

	// Parse pagination params
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	logs, err := h.store.ListAuditLogs(r.Context(), orgID, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("failed to list audit logs")
		writeError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}

	// Convert to response format
	responses := make([]AuditLogResponse, len(logs))
	for i, log := range logs {
		responses[i] = AuditLogResponse{
			ID:             log.ID,
			OrganizationID: log.OrganizationID,
			UserID:         log.UserID,
			Action:         log.Action,
			ResourceType:   log.ResourceType,
			ResourceID:     log.ResourceID,
			IPAddress:      log.IPAddress,
			UserAgent:      log.UserAgent,
			CreatedAt:      log.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if log.Details != nil {
			responses[i].Details = log.Details
		}
	}

	writeJSON(w, http.StatusOK, responses)
}

// ListUserAuditLogs lists audit logs for the current user
// GET /api/v1/me/audit-logs
func (h *AuditHandlers) ListUserAuditLogs(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Parse pagination params
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	logs, err := h.store.ListAuditLogsByUser(r.Context(), session.UserID, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("failed to list audit logs")
		writeError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}

	// Convert to response format
	responses := make([]AuditLogResponse, len(logs))
	for i, log := range logs {
		responses[i] = AuditLogResponse{
			ID:             log.ID,
			OrganizationID: log.OrganizationID,
			UserID:         log.UserID,
			Action:         log.Action,
			ResourceType:   log.ResourceType,
			ResourceID:     log.ResourceID,
			IPAddress:      log.IPAddress,
			UserAgent:      log.UserAgent,
			CreatedAt:      log.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if log.Details != nil {
			responses[i].Details = log.Details
		}
	}

	writeJSON(w, http.StatusOK, responses)
}
