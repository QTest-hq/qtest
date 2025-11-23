// Package jobs provides job hooks for event notification
package jobs

import (
	"context"

	"github.com/google/uuid"
)

// EventType represents a job event type
type EventType string

const (
	EventTypeJobCompleted EventType = "job.completed"
	EventTypeJobFailed    EventType = "job.failed"
)

// JobEvent represents a job lifecycle event
type JobEvent struct {
	Type       EventType
	JobID      uuid.UUID
	JobType    JobType
	RepoID     *uuid.UUID
	Status     string
	Error      string
	DurationMs int64
	Retries    int
}

// EventHook is a function that handles job events
type EventHook func(ctx context.Context, event *JobEvent)

// SetEventHook sets the event hook for job lifecycle events
func (r *Repository) SetEventHook(hook EventHook) {
	r.eventHook = hook
}

// notifyEvent calls the event hook if set
func (r *Repository) notifyEvent(ctx context.Context, event *JobEvent) {
	if r.eventHook != nil {
		// Run in goroutine to not block job completion
		go r.eventHook(ctx, event)
	}
}
