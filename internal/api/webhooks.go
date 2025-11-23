package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/QTest-hq/qtest/internal/db"
	"github.com/QTest-hq/qtest/internal/webhook"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// WebhookHandlers handles webhook API endpoints
type WebhookHandlers struct {
	store   *db.Store
	service *webhook.Service
}

// NewWebhookHandlers creates new webhook handlers
func NewWebhookHandlers(store *db.Store, service *webhook.Service) *WebhookHandlers {
	return &WebhookHandlers{
		store:   store,
		service: service,
	}
}

// CreateWebhookRequest is the request body for creating a webhook
type CreateWebhookRequest struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Events      []string          `json:"events"`
	Description *string           `json:"description,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// UpdateWebhookRequest is the request body for updating a webhook
type UpdateWebhookRequest struct {
	Name        *string           `json:"name,omitempty"`
	URL         *string           `json:"url,omitempty"`
	Events      []string          `json:"events,omitempty"`
	IsActive    *bool             `json:"is_active,omitempty"`
	Description *string           `json:"description,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// CreateWebhook creates a new webhook
func (h *WebhookHandlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from URL
	orgIDStr := chi.URLParam(r, "orgID")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Get user from context
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Verify user has access to org
	canManage, err := h.store.CanManageOrg(ctx, orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check org access")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !canManage {
		respondError(w, http.StatusForbidden, "admin access required to manage webhooks")
		return
	}

	// Parse request body
	var req CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.URL == "" {
		respondError(w, http.StatusBadRequest, "url is required")
		return
	}
	if !strings.HasPrefix(req.URL, "https://") {
		respondError(w, http.StatusBadRequest, "webhook URL must use HTTPS")
		return
	}
	if len(req.Events) == 0 {
		respondError(w, http.StatusBadRequest, "at least one event type is required")
		return
	}

	// Validate event types
	validEvents := webhook.AllEventTypes()
	for _, event := range req.Events {
		valid := false
		for _, ve := range validEvents {
			if event == ve {
				valid = true
				break
			}
		}
		if !valid {
			respondError(w, http.StatusBadRequest, "invalid event type: "+event)
			return
		}
	}

	// Convert headers to JSON
	var headers *json.RawMessage
	if len(req.Headers) > 0 {
		headersJSON, _ := json.Marshal(req.Headers)
		raw := json.RawMessage(headersJSON)
		headers = &raw
	}

	// Create webhook
	wh, err := h.store.CreateWebhook(ctx, orgID, session.UserID, req.Name, req.URL, req.Events, req.Description, headers)
	if err != nil {
		log.Error().Err(err).Msg("failed to create webhook")
		respondError(w, http.StatusInternalServerError, "failed to create webhook")
		return
	}

	// Return response with secret (only time it's visible)
	response := map[string]interface{}{
		"id":         wh.ID,
		"name":       wh.Name,
		"url":        wh.URL,
		"secret":     wh.Secret,
		"events":     wh.Events,
		"is_active":  wh.IsActive,
		"created_at": wh.CreatedAt,
	}

	respondJSON(w, http.StatusCreated, response)
}

// ListWebhooks lists webhooks for an organization
func (h *WebhookHandlers) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from URL
	orgIDStr := chi.URLParam(r, "orgID")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Get user from context
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Verify user has access to org
	isMember, err := h.store.IsMember(ctx, orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check org membership")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !isMember {
		respondError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	// Parse pagination
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// List webhooks
	webhooks, err := h.store.ListWebhooksByOrg(ctx, orgID, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("failed to list webhooks")
		respondError(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}

	respondJSON(w, http.StatusOK, webhooks)
}

// GetWebhook gets a specific webhook
func (h *WebhookHandlers) GetWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from URL
	orgIDStr := chi.URLParam(r, "orgID")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Get webhook ID from URL
	webhookIDStr := chi.URLParam(r, "webhookID")
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid webhook ID")
		return
	}

	// Get user from context
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Verify user has access to org
	isMember, err := h.store.IsMember(ctx, orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check org membership")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !isMember {
		respondError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	// Get webhook
	wh, err := h.store.GetWebhookByID(ctx, webhookID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get webhook")
		respondError(w, http.StatusInternalServerError, "failed to get webhook")
		return
	}
	if wh == nil || wh.OrganizationID != orgID {
		respondError(w, http.StatusNotFound, "webhook not found")
		return
	}

	respondJSON(w, http.StatusOK, wh)
}

// UpdateWebhook updates a webhook
func (h *WebhookHandlers) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from URL
	orgIDStr := chi.URLParam(r, "orgID")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Get webhook ID from URL
	webhookIDStr := chi.URLParam(r, "webhookID")
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid webhook ID")
		return
	}

	// Get user from context
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Verify user has admin access to org
	canManage, err := h.store.CanManageOrg(ctx, orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check org access")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !canManage {
		respondError(w, http.StatusForbidden, "admin access required to manage webhooks")
		return
	}

	// Verify webhook belongs to org
	wh, err := h.store.GetWebhookByID(ctx, webhookID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get webhook")
		respondError(w, http.StatusInternalServerError, "failed to get webhook")
		return
	}
	if wh == nil || wh.OrganizationID != orgID {
		respondError(w, http.StatusNotFound, "webhook not found")
		return
	}

	// Parse request body
	var req UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate URL if provided
	if req.URL != nil && !strings.HasPrefix(*req.URL, "https://") {
		respondError(w, http.StatusBadRequest, "webhook URL must use HTTPS")
		return
	}

	// Validate event types if provided
	if len(req.Events) > 0 {
		validEvents := webhook.AllEventTypes()
		for _, event := range req.Events {
			valid := false
			for _, ve := range validEvents {
				if event == ve {
					valid = true
					break
				}
			}
			if !valid {
				respondError(w, http.StatusBadRequest, "invalid event type: "+event)
				return
			}
		}
	}

	// Convert headers to JSON
	var headers *json.RawMessage
	if len(req.Headers) > 0 {
		headersJSON, _ := json.Marshal(req.Headers)
		raw := json.RawMessage(headersJSON)
		headers = &raw
	}

	// Update webhook
	if err := h.store.UpdateWebhook(ctx, webhookID, req.Name, req.URL, req.Events, req.IsActive, req.Description, headers); err != nil {
		log.Error().Err(err).Msg("failed to update webhook")
		respondError(w, http.StatusInternalServerError, "failed to update webhook")
		return
	}

	// Return updated webhook
	wh, _ = h.store.GetWebhookByID(ctx, webhookID)
	respondJSON(w, http.StatusOK, wh)
}

// DeleteWebhook deletes a webhook
func (h *WebhookHandlers) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from URL
	orgIDStr := chi.URLParam(r, "orgID")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Get webhook ID from URL
	webhookIDStr := chi.URLParam(r, "webhookID")
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid webhook ID")
		return
	}

	// Get user from context
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Verify user has admin access to org
	canManage, err := h.store.CanManageOrg(ctx, orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check org access")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !canManage {
		respondError(w, http.StatusForbidden, "admin access required to manage webhooks")
		return
	}

	// Verify webhook belongs to org
	wh, err := h.store.GetWebhookByID(ctx, webhookID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get webhook")
		respondError(w, http.StatusInternalServerError, "failed to get webhook")
		return
	}
	if wh == nil || wh.OrganizationID != orgID {
		respondError(w, http.StatusNotFound, "webhook not found")
		return
	}

	// Delete webhook
	if err := h.store.DeleteWebhook(ctx, webhookID); err != nil {
		log.Error().Err(err).Msg("failed to delete webhook")
		respondError(w, http.StatusInternalServerError, "failed to delete webhook")
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

// ListDeliveries lists deliveries for a webhook
func (h *WebhookHandlers) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from URL
	orgIDStr := chi.URLParam(r, "orgID")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Get webhook ID from URL
	webhookIDStr := chi.URLParam(r, "webhookID")
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid webhook ID")
		return
	}

	// Get user from context
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Verify user has access to org
	isMember, err := h.store.IsMember(ctx, orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check org membership")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !isMember {
		respondError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	// Verify webhook belongs to org
	wh, err := h.store.GetWebhookByID(ctx, webhookID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get webhook")
		respondError(w, http.StatusInternalServerError, "failed to get webhook")
		return
	}
	if wh == nil || wh.OrganizationID != orgID {
		respondError(w, http.StatusNotFound, "webhook not found")
		return
	}

	// Parse pagination
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// List deliveries
	deliveries, err := h.store.ListDeliveriesByWebhook(ctx, webhookID, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("failed to list deliveries")
		respondError(w, http.StatusInternalServerError, "failed to list deliveries")
		return
	}

	respondJSON(w, http.StatusOK, deliveries)
}

// SendTestWebhook sends a test event to a webhook
func (h *WebhookHandlers) SendTestWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from URL
	orgIDStr := chi.URLParam(r, "orgID")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Get webhook ID from URL
	webhookIDStr := chi.URLParam(r, "webhookID")
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid webhook ID")
		return
	}

	// Get user from context
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Verify user has admin access to org
	canManage, err := h.store.CanManageOrg(ctx, orgID, session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to check org access")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !canManage {
		respondError(w, http.StatusForbidden, "admin access required to test webhooks")
		return
	}

	// Get webhook
	wh, err := h.store.GetWebhookByID(ctx, webhookID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get webhook")
		respondError(w, http.StatusInternalServerError, "failed to get webhook")
		return
	}
	if wh == nil || wh.OrganizationID != orgID {
		respondError(w, http.StatusNotFound, "webhook not found")
		return
	}

	// Send test event
	if err := h.service.SendTestEvent(ctx, wh); err != nil {
		log.Error().Err(err).Msg("failed to send test webhook")
		respondError(w, http.StatusBadGateway, "webhook delivery failed: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Test webhook delivered successfully",
	})
}
