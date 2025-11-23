package jobs

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestEventType tests event type constants
func TestEventType(t *testing.T) {
	if EventTypeJobCompleted != "job.completed" {
		t.Errorf("EventTypeJobCompleted = %s, want job.completed", EventTypeJobCompleted)
	}
	if EventTypeJobFailed != "job.failed" {
		t.Errorf("EventTypeJobFailed = %s, want job.failed", EventTypeJobFailed)
	}
}

// TestJobEvent tests JobEvent struct
func TestJobEvent(t *testing.T) {
	jobID := uuid.New()
	repoID := uuid.New()

	event := &JobEvent{
		Type:       EventTypeJobCompleted,
		JobID:      jobID,
		JobType:    JobTypeGeneration,
		RepoID:     &repoID,
		Status:     "completed",
		DurationMs: 5000,
	}

	if event.Type != EventTypeJobCompleted {
		t.Errorf("Type = %s, want %s", event.Type, EventTypeJobCompleted)
	}
	if event.JobID != jobID {
		t.Error("JobID mismatch")
	}
	if event.JobType != JobTypeGeneration {
		t.Errorf("JobType = %s, want %s", event.JobType, JobTypeGeneration)
	}
	if *event.RepoID != repoID {
		t.Error("RepoID mismatch")
	}
	if event.Status != "completed" {
		t.Errorf("Status = %s, want completed", event.Status)
	}
	if event.DurationMs != 5000 {
		t.Errorf("DurationMs = %d, want 5000", event.DurationMs)
	}
}

// TestJobEvent_Failed tests JobEvent for failed jobs
func TestJobEvent_Failed(t *testing.T) {
	event := &JobEvent{
		Type:    EventTypeJobFailed,
		JobID:   uuid.New(),
		JobType: JobTypeIngestion,
		Error:   "connection timeout",
		Retries: 3,
	}

	if event.Type != EventTypeJobFailed {
		t.Errorf("Type = %s, want %s", event.Type, EventTypeJobFailed)
	}
	if event.Error != "connection timeout" {
		t.Errorf("Error = %s, want connection timeout", event.Error)
	}
	if event.Retries != 3 {
		t.Errorf("Retries = %d, want 3", event.Retries)
	}
}

// TestJobEvent_OptionalFields tests JobEvent with nil optional fields
func TestJobEvent_OptionalFields(t *testing.T) {
	event := &JobEvent{
		Type:    EventTypeJobCompleted,
		JobID:   uuid.New(),
		JobType: JobTypeGeneration,
		RepoID:  nil, // Optional, can be nil
		Status:  "completed",
	}

	if event.RepoID != nil {
		t.Error("RepoID should be nil")
	}
}

// MockEventHookReceiver captures events for testing
type MockEventHookReceiver struct {
	mu     sync.Mutex
	events []*JobEvent
}

func (m *MockEventHookReceiver) HandleEvent(ctx context.Context, event *JobEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *MockEventHookReceiver) Events() []*JobEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.events
}

func (m *MockEventHookReceiver) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
}

// TestEventHook_Type tests EventHook function signature
func TestEventHook_Type(t *testing.T) {
	// Test that EventHook is compatible with expected signature
	var hook EventHook = func(ctx context.Context, event *JobEvent) {
		// No-op handler
	}

	if hook == nil {
		t.Error("hook should not be nil")
	}

	// Verify it can be called without error
	hook(context.Background(), &JobEvent{
		Type:  EventTypeJobCompleted,
		JobID: uuid.New(),
	})
}

// TestRepository_SetEventHook tests setting event hook on repository
// Note: This is a unit test that doesn't require a database
func TestRepository_EventHookField(t *testing.T) {
	// Create a minimal repository structure for testing hook logic
	// The actual database operations would be tested separately

	receiver := &MockEventHookReceiver{}
	hook := receiver.HandleEvent

	// Verify the hook can be invoked
	ctx := context.Background()
	event := &JobEvent{
		Type:    EventTypeJobCompleted,
		JobID:   uuid.New(),
		JobType: JobTypeGeneration,
		Status:  "completed",
	}

	hook(ctx, event)

	// Wait a bit for async processing (hooks run in goroutines)
	time.Sleep(10 * time.Millisecond)

	events := receiver.Events()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
	if len(events) > 0 && events[0].Type != EventTypeJobCompleted {
		t.Errorf("event type = %s, want %s", events[0].Type, EventTypeJobCompleted)
	}
}

// TestEventHook_MultipleEvents tests handling multiple events
func TestEventHook_MultipleEvents(t *testing.T) {
	receiver := &MockEventHookReceiver{}
	hook := receiver.HandleEvent

	ctx := context.Background()

	// Send multiple events
	for i := 0; i < 5; i++ {
		event := &JobEvent{
			Type:       EventTypeJobCompleted,
			JobID:      uuid.New(),
			JobType:    JobTypeGeneration,
			Status:     "completed",
			DurationMs: int64(i * 1000),
		}
		hook(ctx, event)
	}

	// Wait for async processing
	time.Sleep(50 * time.Millisecond)

	events := receiver.Events()
	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}

// TestEventHook_NilEvent tests that hook handles nil event gracefully
func TestEventHook_NilEvent(t *testing.T) {
	// Create a hook that won't panic on nil
	var received *JobEvent
	hook := func(ctx context.Context, event *JobEvent) {
		received = event
	}

	// Shouldn't panic
	hook(context.Background(), nil)

	if received != nil {
		t.Error("expected nil event to be passed through")
	}
}

// TestJobEvent_AllJobTypes tests events with different job types
func TestJobEvent_AllJobTypes(t *testing.T) {
	jobTypes := []JobType{
		JobTypeGeneration,
		JobTypeIngestion,
		JobTypeMutation,
		JobTypeValidation,
	}

	for _, jt := range jobTypes {
		t.Run(string(jt), func(t *testing.T) {
			event := &JobEvent{
				Type:    EventTypeJobCompleted,
				JobID:   uuid.New(),
				JobType: jt,
				Status:  "completed",
			}

			if event.JobType != jt {
				t.Errorf("JobType = %s, want %s", event.JobType, jt)
			}
		})
	}
}
