package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/QTest-hq/qtest/internal/db"
)

// OrganizationHandlers handles organization-related API endpoints
type OrganizationHandlers struct {
	store *db.Store
}

// NewOrganizationHandlers creates new organization handlers
func NewOrganizationHandlers(store *db.Store) *OrganizationHandlers {
	return &OrganizationHandlers{store: store}
}

// CreateOrganizationRequest is the request body for creating an org
type CreateOrganizationRequest struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description,omitempty"`
}

// UpdateOrganizationRequest is the request body for updating an org
type UpdateOrganizationRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// AddMemberRequest is the request body for adding a member
type AddMemberRequest struct {
	UserID string        `json:"user_id"`
	Role   db.MemberRole `json:"role"`
}

// UpdateMemberRoleRequest is the request body for updating member role
type UpdateMemberRoleRequest struct {
	Role db.MemberRole `json:"role"`
}

// ListOrganizations returns all organizations the user belongs to
// GET /api/v1/organizations
func (h *OrganizationHandlers) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	orgs, err := h.store.ListUserOrganizations(r.Context(), session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to list organizations")
		writeError(w, http.StatusInternalServerError, "failed to list organizations")
		return
	}

	writeJSON(w, http.StatusOK, orgs)
}

// GetOrganization returns a single organization
// GET /api/v1/organizations/{orgID}
func (h *OrganizationHandlers) GetOrganization(w http.ResponseWriter, r *http.Request) {
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

	// Check membership
	isMember, err := h.store.IsMember(r.Context(), orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check membership")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !isMember {
		writeError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	org, err := h.store.GetOrganizationByID(r.Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get organization")
		writeError(w, http.StatusInternalServerError, "failed to get organization")
		return
	}
	if org == nil {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	// Get user's role
	role, _ := h.store.GetMemberRole(r.Context(), orgID, session.UserID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"organization": org,
		"role":         role,
	})
}

// CreateOrganization creates a new organization
// POST /api/v1/organizations
func (h *OrganizationHandlers) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req CreateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "name and slug are required")
		return
	}

	// Check if slug is taken
	existing, err := h.store.GetOrganizationBySlug(r.Context(), req.Slug)
	if err != nil {
		log.Error().Err(err).Msg("failed to check slug")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "slug already taken")
		return
	}

	org := &db.Organization{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		OwnerID:     session.UserID,
		IsPersonal:  false,
	}

	if err := h.store.CreateOrganization(r.Context(), org); err != nil {
		log.Error().Err(err).Msg("failed to create organization")
		writeError(w, http.StatusInternalServerError, "failed to create organization")
		return
	}

	log.Info().
		Str("org_id", org.ID.String()).
		Str("slug", org.Slug).
		Str("owner", session.UserID.String()).
		Msg("organization created")

	writeJSON(w, http.StatusCreated, org)
}

// UpdateOrganization updates an organization
// PATCH /api/v1/organizations/{orgID}
func (h *OrganizationHandlers) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
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

	// Check admin permission
	canManage, err := h.store.CanManageOrg(r.Context(), orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check permissions")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !canManage {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	var req UpdateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	org, err := h.store.GetOrganizationByID(r.Context(), orgID)
	if err != nil || org == nil {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	if req.Name != nil {
		org.Name = *req.Name
	}
	if req.Description != nil {
		org.Description = req.Description
	}

	if err := h.store.UpdateOrganization(r.Context(), org); err != nil {
		log.Error().Err(err).Msg("failed to update organization")
		writeError(w, http.StatusInternalServerError, "failed to update organization")
		return
	}

	writeJSON(w, http.StatusOK, org)
}

// DeleteOrganization deletes an organization
// DELETE /api/v1/organizations/{orgID}
func (h *OrganizationHandlers) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
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

	org, err := h.store.GetOrganizationByID(r.Context(), orgID)
	if err != nil || org == nil {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	// Only owner can delete
	if org.OwnerID != session.UserID {
		writeError(w, http.StatusForbidden, "only the owner can delete the organization")
		return
	}

	if org.IsPersonal {
		writeError(w, http.StatusBadRequest, "cannot delete personal organization")
		return
	}

	if err := h.store.DeleteOrganization(r.Context(), orgID); err != nil {
		log.Error().Err(err).Msg("failed to delete organization")
		writeError(w, http.StatusInternalServerError, "failed to delete organization")
		return
	}

	log.Info().
		Str("org_id", orgID.String()).
		Msg("organization deleted")

	w.WriteHeader(http.StatusNoContent)
}

// ListMembers returns all members of an organization
// GET /api/v1/organizations/{orgID}/members
func (h *OrganizationHandlers) ListMembers(w http.ResponseWriter, r *http.Request) {
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

	// Check membership
	isMember, err := h.store.IsMember(r.Context(), orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check membership")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !isMember {
		writeError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	members, err := h.store.ListOrganizationMembers(r.Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("failed to list members")
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}

	writeJSON(w, http.StatusOK, members)
}

// AddMember adds a user to an organization
// POST /api/v1/organizations/{orgID}/members
func (h *OrganizationHandlers) AddMember(w http.ResponseWriter, r *http.Request) {
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

	// Check admin permission
	canManage, err := h.store.CanManageOrg(r.Context(), orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check permissions")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !canManage {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Validate role
	if req.Role != db.RoleAdmin && req.Role != db.RoleMember && req.Role != db.RoleViewer {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}

	if err := h.store.AddOrganizationMember(r.Context(), orgID, userID, req.Role, &session.UserID); err != nil {
		log.Error().Err(err).Msg("failed to add member")
		writeError(w, http.StatusInternalServerError, "failed to add member")
		return
	}

	log.Info().
		Str("org_id", orgID.String()).
		Str("user_id", userID.String()).
		Str("role", string(req.Role)).
		Msg("member added")

	w.WriteHeader(http.StatusCreated)
}

// UpdateMemberRole updates a member's role
// PATCH /api/v1/organizations/{orgID}/members/{userID}
func (h *OrganizationHandlers) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
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

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Check admin permission
	canManage, err := h.store.CanManageOrg(r.Context(), orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check permissions")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !canManage {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	var req UpdateMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Can't change owner role
	org, _ := h.store.GetOrganizationByID(r.Context(), orgID)
	if org != nil && org.OwnerID == userID {
		writeError(w, http.StatusBadRequest, "cannot change owner's role")
		return
	}

	if err := h.store.UpdateMemberRole(r.Context(), orgID, userID, req.Role); err != nil {
		log.Error().Err(err).Msg("failed to update role")
		writeError(w, http.StatusInternalServerError, "failed to update role")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// RemoveMember removes a user from an organization
// DELETE /api/v1/organizations/{orgID}/members/{userID}
func (h *OrganizationHandlers) RemoveMember(w http.ResponseWriter, r *http.Request) {
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

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Users can remove themselves, admins can remove others
	if userID != session.UserID {
		canManage, err := h.store.CanManageOrg(r.Context(), orgID, session.UserID)
		if err != nil {
			log.Error().Err(err).Msg("failed to check permissions")
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !canManage {
			writeError(w, http.StatusForbidden, "insufficient permissions")
			return
		}
	}

	// Can't remove owner
	org, _ := h.store.GetOrganizationByID(r.Context(), orgID)
	if org != nil && org.OwnerID == userID {
		writeError(w, http.StatusBadRequest, "cannot remove the owner")
		return
	}

	if err := h.store.RemoveOrganizationMember(r.Context(), orgID, userID); err != nil {
		log.Error().Err(err).Msg("failed to remove member")
		writeError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}

	log.Info().
		Str("org_id", orgID.String()).
		Str("user_id", userID.String()).
		Msg("member removed")

	w.WriteHeader(http.StatusNoContent)
}

// Helper functions
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
