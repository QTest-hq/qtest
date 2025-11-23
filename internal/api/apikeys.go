package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/QTest-hq/qtest/internal/db"
)

// APIKeyHandlers handles API key management endpoints
type APIKeyHandlers struct {
	store *db.Store
}

// NewAPIKeyHandlers creates new API key handlers
func NewAPIKeyHandlers(store *db.Store) *APIKeyHandlers {
	return &APIKeyHandlers{store: store}
}

// CreateAPIKeyRequest is the request body for creating an API key
type CreateAPIKeyRequest struct {
	Name           string     `json:"name"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"` // Uses personal org if not specified
	Scopes         []string   `json:"scopes"`
	ExpiresInDays  *int       `json:"expires_in_days,omitempty"` // Optional expiration
}

// APIKeyResponse is the response for API key operations
type APIKeyResponse struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Name           string     `json:"name"`
	KeyPrefix      string     `json:"key_prefix"`
	Scopes         []string   `json:"scopes"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	Secret         string     `json:"secret,omitempty"` // Only returned on creation
}

// CreateAPIKey creates a new API key
// POST /api/v1/api-keys
func (h *APIKeyHandlers) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if len(req.Scopes) == 0 {
		writeError(w, http.StatusBadRequest, "at least one scope is required")
		return
	}

	// Validate scopes
	validScopes := map[string]bool{
		string(db.ScopeReadRepos):    true,
		string(db.ScopeWriteRepos):   true,
		string(db.ScopeReadRuns):     true,
		string(db.ScopeWriteRuns):    true,
		string(db.ScopeReadTests):    true,
		string(db.ScopeWriteTests):   true,
		string(db.ScopeReadJobs):     true,
		string(db.ScopeWriteJobs):    true,
		string(db.ScopeReadMutation): true,
		string(db.ScopeAdmin):        true,
	}
	for _, scope := range req.Scopes {
		if !validScopes[scope] {
			writeError(w, http.StatusBadRequest, "invalid scope: "+scope)
			return
		}
	}

	// Determine organization ID
	var orgID uuid.UUID
	if req.OrganizationID != nil {
		// Verify user has admin access to the org
		canManage, err := h.store.CanManageOrg(r.Context(), *req.OrganizationID, session.UserID)
		if err != nil {
			log.Error().Err(err).Msg("failed to check permissions")
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !canManage {
			writeError(w, http.StatusForbidden, "admin permission required to create API keys for this organization")
			return
		}
		orgID = *req.OrganizationID
	} else {
		// Use personal organization
		personalOrg, err := h.store.GetPersonalOrganization(r.Context(), session.UserID)
		if err != nil || personalOrg == nil {
			log.Error().Err(err).Msg("failed to get personal organization")
			writeError(w, http.StatusInternalServerError, "failed to get personal organization")
			return
		}
		orgID = personalOrg.ID
	}

	// Calculate expiration
	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		exp := time.Now().AddDate(0, 0, *req.ExpiresInDays)
		expiresAt = &exp
	}

	// Create the API key
	key, err := h.store.CreateAPIKey(r.Context(), orgID, session.UserID, req.Name, req.Scopes, expiresAt)
	if err != nil {
		log.Error().Err(err).Msg("failed to create API key")
		writeError(w, http.StatusInternalServerError, "failed to create API key")
		return
	}

	// Log audit event
	h.store.LogAuditEvent(r.Context(), &orgID, &session.UserID, db.AuditActionAPIKeyCreate,
		db.ResourceTypeAPIKey, &key.ID, map[string]string{"name": req.Name}, "", "")

	log.Info().
		Str("key_id", key.ID.String()).
		Str("name", req.Name).
		Str("org_id", orgID.String()).
		Msg("API key created")

	writeJSON(w, http.StatusCreated, APIKeyResponse{
		ID:             key.ID,
		OrganizationID: key.OrganizationID,
		Name:           key.Name,
		KeyPrefix:      key.KeyPrefix,
		Scopes:         key.Scopes,
		ExpiresAt:      key.ExpiresAt,
		CreatedAt:      key.CreatedAt,
		Secret:         key.Secret, // Only returned on creation
	})
}

// ListAPIKeys lists API keys for the user or organization
// GET /api/v1/api-keys
func (h *APIKeyHandlers) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	orgIDStr := r.URL.Query().Get("organization_id")

	var keys []db.APIKey
	var err error

	if orgIDStr != "" {
		orgID, parseErr := uuid.Parse(orgIDStr)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid organization_id")
			return
		}

		// Verify membership
		isMember, memErr := h.store.IsMember(r.Context(), orgID, session.UserID)
		if memErr != nil {
			log.Error().Err(memErr).Msg("failed to check membership")
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !isMember {
			writeError(w, http.StatusForbidden, "not a member of this organization")
			return
		}

		keys, err = h.store.ListAPIKeysByOrg(r.Context(), orgID)
	} else {
		keys, err = h.store.ListAPIKeysByUser(r.Context(), session.UserID)
	}

	if err != nil {
		log.Error().Err(err).Msg("failed to list API keys")
		writeError(w, http.StatusInternalServerError, "failed to list API keys")
		return
	}

	// Convert to response format (without secrets)
	responses := make([]APIKeyResponse, len(keys))
	for i, key := range keys {
		responses[i] = APIKeyResponse{
			ID:             key.ID,
			OrganizationID: key.OrganizationID,
			Name:           key.Name,
			KeyPrefix:      key.KeyPrefix,
			Scopes:         key.Scopes,
			ExpiresAt:      key.ExpiresAt,
			LastUsedAt:     key.LastUsedAt,
			CreatedAt:      key.CreatedAt,
		}
	}

	writeJSON(w, http.StatusOK, responses)
}

// GetAPIKey gets details of a specific API key
// GET /api/v1/api-keys/{keyID}
func (h *APIKeyHandlers) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	keyID, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid key ID")
		return
	}

	key, err := h.store.GetAPIKeyByID(r.Context(), keyID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get API key")
		writeError(w, http.StatusInternalServerError, "failed to get API key")
		return
	}

	if key == nil {
		writeError(w, http.StatusNotFound, "API key not found")
		return
	}

	// Verify user owns the key or has org admin access
	if key.UserID != session.UserID {
		canManage, manageErr := h.store.CanManageOrg(r.Context(), key.OrganizationID, session.UserID)
		if manageErr != nil || !canManage {
			writeError(w, http.StatusForbidden, "you don't have access to this API key")
			return
		}
	}

	writeJSON(w, http.StatusOK, APIKeyResponse{
		ID:             key.ID,
		OrganizationID: key.OrganizationID,
		Name:           key.Name,
		KeyPrefix:      key.KeyPrefix,
		Scopes:         key.Scopes,
		ExpiresAt:      key.ExpiresAt,
		LastUsedAt:     key.LastUsedAt,
		CreatedAt:      key.CreatedAt,
	})
}

// RevokeAPIKey revokes an API key
// DELETE /api/v1/api-keys/{keyID}
func (h *APIKeyHandlers) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	keyID, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid key ID")
		return
	}

	// Get the key first for audit logging
	key, err := h.store.GetAPIKeyByID(r.Context(), keyID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get API key")
		writeError(w, http.StatusInternalServerError, "failed to get API key")
		return
	}

	if key == nil {
		writeError(w, http.StatusNotFound, "API key not found")
		return
	}

	if err := h.store.RevokeAPIKey(r.Context(), keyID, session.UserID); err != nil {
		log.Error().Err(err).Msg("failed to revoke API key")
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	// Log audit event
	h.store.LogAuditEvent(r.Context(), &key.OrganizationID, &session.UserID, db.AuditActionAPIKeyRevoke,
		db.ResourceTypeAPIKey, &keyID, map[string]string{"name": key.Name}, "", "")

	log.Info().
		Str("key_id", keyID.String()).
		Str("user_id", session.UserID.String()).
		Msg("API key revoked")

	w.WriteHeader(http.StatusNoContent)
}
