package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/QTest-hq/qtest/internal/db"
)

// CoverageHandlers handles coverage-related API endpoints
type CoverageHandlers struct {
	store *db.Store
}

// NewCoverageHandlers creates new coverage handlers
func NewCoverageHandlers(store *db.Store) *CoverageHandlers {
	return &CoverageHandlers{store: store}
}

// GetSummary returns overall coverage statistics
// GET /api/v1/coverage/summary
func (h *CoverageHandlers) GetSummary(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Get org from query param or use user's personal org
	var orgID *uuid.UUID
	if orgStr := r.URL.Query().Get("organization_id"); orgStr != "" {
		id, err := uuid.Parse(orgStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid organization ID")
			return
		}
		// Verify membership
		isMember, _ := h.store.IsMember(r.Context(), id, session.UserID)
		if !isMember {
			writeError(w, http.StatusForbidden, "not a member of this organization")
			return
		}
		orgID = &id
	}

	summary, err := h.store.GetCoverageSummary(r.Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get coverage summary")
		writeError(w, http.StatusInternalServerError, "failed to get coverage summary")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// ListSnapshots returns coverage snapshots
// GET /api/v1/coverage/snapshots
func (h *CoverageHandlers) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var repoID *uuid.UUID
	if repoStr := r.URL.Query().Get("repository_id"); repoStr != "" {
		id, err := uuid.Parse(repoStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid repository ID")
			return
		}
		repoID = &id
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	snapshots, err := h.store.ListCoverageSnapshots(r.Context(), repoID, limit)
	if err != nil {
		log.Error().Err(err).Msg("failed to list coverage snapshots")
		writeError(w, http.StatusInternalServerError, "failed to list snapshots")
		return
	}

	writeJSON(w, http.StatusOK, snapshots)
}

// GetSnapshot returns a single coverage snapshot
// GET /api/v1/coverage/snapshots/{id}
func (h *CoverageHandlers) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid snapshot ID")
		return
	}

	snapshot, err := h.store.GetCoverageSnapshot(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Msg("failed to get snapshot")
		writeError(w, http.StatusInternalServerError, "failed to get snapshot")
		return
	}
	if snapshot == nil {
		writeError(w, http.StatusNotFound, "snapshot not found")
		return
	}

	writeJSON(w, http.StatusOK, snapshot)
}

// GetLatestSnapshot returns the latest snapshot for a repo
// GET /api/v1/coverage/repos/{repoID}/latest
func (h *CoverageHandlers) GetLatestSnapshot(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	repoID, err := uuid.Parse(chi.URLParam(r, "repoID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repository ID")
		return
	}

	snapshot, err := h.store.GetLatestCoverageSnapshot(r.Context(), repoID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get latest snapshot")
		writeError(w, http.StatusInternalServerError, "failed to get snapshot")
		return
	}
	if snapshot == nil {
		writeError(w, http.StatusNotFound, "no snapshots found")
		return
	}

	writeJSON(w, http.StatusOK, snapshot)
}

// GetTrend returns coverage trend over time
// GET /api/v1/coverage/repos/{repoID}/trend
func (h *CoverageHandlers) GetTrend(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	repoID, err := uuid.Parse(chi.URLParam(r, "repoID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repository ID")
		return
	}

	days := 30
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	trend, err := h.store.GetCoverageTrend(r.Context(), repoID, days)
	if err != nil {
		log.Error().Err(err).Msg("failed to get coverage trend")
		writeError(w, http.StatusInternalServerError, "failed to get trend")
		return
	}

	writeJSON(w, http.StatusOK, trend)
}

// CreateSnapshot creates a new coverage snapshot
// POST /api/v1/coverage/snapshots
func (h *CoverageHandlers) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.GetSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var snap db.CoverageSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snap); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if snap.RepositoryID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "repository_id is required")
		return
	}
	if snap.Language == "" {
		writeError(w, http.StatusBadRequest, "language is required")
		return
	}

	// Calculate coverage percent if not provided
	if snap.TotalLines > 0 && snap.CoveragePercent == 0 {
		snap.CoveragePercent = float64(snap.CoveredLines) / float64(snap.TotalLines) * 100
	}

	// Get previous snapshot for delta calculation
	prev, _ := h.store.GetLatestCoverageSnapshot(r.Context(), snap.RepositoryID)
	if prev != nil {
		snap.PreviousSnapshotID = &prev.ID
		snap.LinesDelta = snap.TotalLines - prev.TotalLines
		snap.CoverageDelta = snap.CoveragePercent - prev.CoveragePercent
	}

	if err := h.store.CreateCoverageSnapshot(r.Context(), &snap); err != nil {
		log.Error().Err(err).Msg("failed to create snapshot")
		writeError(w, http.StatusInternalServerError, "failed to create snapshot")
		return
	}

	writeJSON(w, http.StatusCreated, snap)
}
