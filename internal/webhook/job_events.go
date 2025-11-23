// Package webhook provides job event integration
package webhook

import (
	"context"

	"github.com/QTest-hq/qtest/internal/db"
	"github.com/QTest-hq/qtest/internal/jobs"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// JobEventHandler handles job lifecycle events and triggers webhooks
type JobEventHandler struct {
	service *Service
	store   *db.Store
}

// NewJobEventHandler creates a new job event handler
func NewJobEventHandler(service *Service, store *db.Store) *JobEventHandler {
	return &JobEventHandler{
		service: service,
		store:   store,
	}
}

// HandleEvent is the event hook callback for job events
func (h *JobEventHandler) HandleEvent(ctx context.Context, event *jobs.JobEvent) {
	if h.service == nil || event.RepoID == nil {
		return
	}

	// Look up organization from repository
	repo, err := h.store.GetRepository(ctx, *event.RepoID)
	if err != nil || repo == nil || repo.OrganizationID == nil {
		log.Debug().
			Str("repo_id", event.RepoID.String()).
			Msg("no organization found for repository, skipping webhook")
		return
	}

	orgID := *repo.OrganizationID

	switch event.Type {
	case jobs.EventTypeJobCompleted:
		h.triggerJobCompleted(ctx, orgID, event)
	case jobs.EventTypeJobFailed:
		h.triggerJobFailed(ctx, orgID, event)
	}
}

func (h *JobEventHandler) triggerJobCompleted(ctx context.Context, orgID uuid.UUID, event *jobs.JobEvent) {
	data := &JobCompletedData{
		JobID:   event.JobID,
		JobType: string(event.JobType),
		Status:  event.Status,
	}
	if event.RepoID != nil {
		data.RepositoryID = *event.RepoID
	}
	if event.DurationMs > 0 {
		data.Duration = event.DurationMs
	}

	if err := h.service.TriggerEvent(ctx, orgID, EventJobCompleted, event.JobID, data); err != nil {
		log.Error().Err(err).
			Str("job_id", event.JobID.String()).
			Msg("failed to trigger job.completed webhook")
	}
}

func (h *JobEventHandler) triggerJobFailed(ctx context.Context, orgID uuid.UUID, event *jobs.JobEvent) {
	data := &JobFailedData{
		JobID:   event.JobID,
		JobType: string(event.JobType),
		Error:   event.Error,
		Retries: event.Retries,
	}
	if event.RepoID != nil {
		data.RepositoryID = *event.RepoID
	}

	if err := h.service.TriggerEvent(ctx, orgID, EventJobFailed, event.JobID, data); err != nil {
		log.Error().Err(err).
			Str("job_id", event.JobID.String()).
			Msg("failed to trigger job.failed webhook")
	}
}
