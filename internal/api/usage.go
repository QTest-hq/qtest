package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/QTest-hq/qtest/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// UsageHandlers handles usage and analytics endpoints
type UsageHandlers struct {
	store *db.Store
}

// NewUsageHandlers creates a new UsageHandlers
func NewUsageHandlers(store *db.Store) *UsageHandlers {
	return &UsageHandlers{store: store}
}

// GetUsageSummary returns usage summary for an organization
// GET /api/v1/organizations/{orgID}/usage/summary
func (h *UsageHandlers) GetUsageSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, err := uuid.Parse(chi.URLParam(r, "orgID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Verify access
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	isMember, err := h.store.IsMember(ctx, orgID, session.UserID)
	if err != nil || !isMember {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "today"
	}

	summary, err := h.store.GetUsageSummary(ctx, orgID, period)
	if err != nil {
		log.Error().Err(err).Msg("failed to get usage summary")
		respondError(w, http.StatusInternalServerError, "failed to get usage summary")
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// GetDailyStats returns daily usage statistics
// GET /api/v1/organizations/{orgID}/usage/daily
func (h *UsageHandlers) GetDailyStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, err := uuid.Parse(chi.URLParam(r, "orgID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Verify access
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	isMember, err := h.store.IsMember(ctx, orgID, session.UserID)
	if err != nil || !isMember {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	// Parse date range (default: last 30 days)
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	if startStr := r.URL.Query().Get("start_date"); startStr != "" {
		if parsed, err := time.Parse("2006-01-02", startStr); err == nil {
			startDate = parsed
		}
	}
	if endStr := r.URL.Query().Get("end_date"); endStr != "" {
		if parsed, err := time.Parse("2006-01-02", endStr); err == nil {
			endDate = parsed
		}
	}

	stats, err := h.store.GetDailyStats(ctx, orgID, startDate, endDate)
	if err != nil {
		log.Error().Err(err).Msg("failed to get daily stats")
		respondError(w, http.StatusInternalServerError, "failed to get daily stats")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
		"stats":      stats,
	})
}

// GetMonthlyStats returns monthly usage statistics
// GET /api/v1/organizations/{orgID}/usage/monthly
func (h *UsageHandlers) GetMonthlyStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, err := uuid.Parse(chi.URLParam(r, "orgID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Verify access
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	isMember, err := h.store.IsMember(ctx, orgID, session.UserID)
	if err != nil || !isMember {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	// Parse year (default: current year)
	year := time.Now().Year()
	if yearStr := r.URL.Query().Get("year"); yearStr != "" {
		if parsed, err := strconv.Atoi(yearStr); err == nil && parsed > 2000 {
			year = parsed
		}
	}

	stats, err := h.store.GetMonthlyStats(ctx, orgID, year)
	if err != nil {
		log.Error().Err(err).Msg("failed to get monthly stats")
		respondError(w, http.StatusInternalServerError, "failed to get monthly stats")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"year":  year,
		"stats": stats,
	})
}

// GetRecentUsage returns recent API usage
// GET /api/v1/organizations/{orgID}/usage/recent
func (h *UsageHandlers) GetRecentUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, err := uuid.Parse(chi.URLParam(r, "orgID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Verify access
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	isMember, err := h.store.IsMember(ctx, orgID, session.UserID)
	if err != nil || !isMember {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	usage, err := h.store.GetRecentAPIUsage(ctx, orgID, limit)
	if err != nil {
		log.Error().Err(err).Msg("failed to get recent usage")
		respondError(w, http.StatusInternalServerError, "failed to get recent usage")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"limit": limit,
		"usage": usage,
	})
}

// GetEndpointStats returns usage stats by endpoint
// GET /api/v1/organizations/{orgID}/usage/endpoints
func (h *UsageHandlers) GetEndpointStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, err := uuid.Parse(chi.URLParam(r, "orgID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Verify access
	session, ok := auth.GetSessionFromContext(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	isMember, err := h.store.IsMember(ctx, orgID, session.UserID)
	if err != nil || !isMember {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	// Default: last 7 days
	startTime := time.Now().AddDate(0, 0, -7)
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if days, err := strconv.Atoi(daysStr); err == nil && days > 0 && days <= 90 {
			startTime = time.Now().AddDate(0, 0, -days)
		}
	}

	stats, err := h.store.GetEndpointStats(ctx, orgID, startTime)
	if err != nil {
		log.Error().Err(err).Msg("failed to get endpoint stats")
		respondError(w, http.StatusInternalServerError, "failed to get endpoint stats")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"start_time": startTime.Format(time.RFC3339),
		"endpoints":  stats,
	})
}
