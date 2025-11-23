// Package maintenance provides test maintenance scheduling and orchestration.
// It uses drift detection to identify code changes and schedules appropriate
// maintenance jobs (regenerate, remove, update tests).
package maintenance

import (
	"sort"
	"time"

	"github.com/QTest-hq/qtest/internal/differ"
	"github.com/QTest-hq/qtest/pkg/model"
)

// JobType represents the type of maintenance job
type JobType string

const (
	// JobTypeRegenerate indicates tests need to be regenerated due to code changes
	JobTypeRegenerate JobType = "regenerate"
	// JobTypeRemove indicates tests should be removed (code was deleted)
	JobTypeRemove JobType = "remove"
	// JobTypeUpdate indicates tests need minor updates (non-breaking changes)
	JobTypeUpdate JobType = "update"
	// JobTypeCreate indicates new tests should be created (new code added)
	JobTypeCreate JobType = "create"
)

// JobPriority represents the priority of a maintenance job
type JobPriority int

const (
	// PriorityHigh for breaking changes (signature changes, removed code)
	PriorityHigh JobPriority = 1
	// PriorityMedium for functional changes (body changes, new code)
	PriorityMedium JobPriority = 2
	// PriorityLow for non-functional changes (decorators, comments)
	PriorityLow JobPriority = 3
)

// MaintenanceJob represents a single test maintenance task
type MaintenanceJob struct {
	ID         string      `json:"id"`
	Type       JobType     `json:"type"`
	TargetID   string      `json:"target_id"`   // Function/Endpoint ID
	TargetType string      `json:"target_type"` // "function", "endpoint", "type"
	Priority   JobPriority `json:"priority"`
	Reason     string      `json:"reason"`
	CreatedAt  time.Time   `json:"created_at"`

	// Context for the job
	OldCommit string `json:"old_commit"`
	NewCommit string `json:"new_commit"`

	// Change details
	ChangeTypes []string `json:"change_types,omitempty"`

	// For regenerate/update jobs
	OldEntity interface{} `json:"old_entity,omitempty"`
	NewEntity interface{} `json:"new_entity,omitempty"`
}

// MaintenanceResult represents the result of processing a maintenance job
type MaintenanceResult struct {
	JobID      string    `json:"job_id"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	TestsAdded int       `json:"tests_added"`
	TestsRemoved int     `json:"tests_removed"`
	TestsUpdated int     `json:"tests_updated"`
	ProcessedAt time.Time `json:"processed_at"`
}

// SchedulerConfig configures the maintenance scheduler
type SchedulerConfig struct {
	// Whether to create tests for new functions automatically
	AutoCreateTests bool
	// Whether to regenerate tests on signature changes
	RegenerateOnSignatureChange bool
	// Whether to update tests on body-only changes
	UpdateOnBodyChange bool
	// Whether to remove tests when code is deleted
	RemoveOrphanedTests bool
}

// DefaultSchedulerConfig returns a sensible default configuration
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		AutoCreateTests:             true,
		RegenerateOnSignatureChange: true,
		UpdateOnBodyChange:          true,
		RemoveOrphanedTests:         true,
	}
}

// Scheduler orchestrates test maintenance based on code changes
type Scheduler struct {
	config SchedulerConfig
	differ *differ.Differ
}

// NewScheduler creates a new maintenance scheduler
func NewScheduler(config SchedulerConfig) *Scheduler {
	return &Scheduler{
		config: config,
		differ: differ.New(),
	}
}

// OnPush processes a push event and returns maintenance jobs
func (s *Scheduler) OnPush(oldModel, newModel *model.SystemModel) []MaintenanceJob {
	if oldModel == nil || newModel == nil {
		return nil
	}

	// Compute diff
	diff := s.differ.DiffSystemModels(oldModel, newModel)
	if !diff.HasChanges() {
		return nil
	}

	// Generate jobs from diff
	jobs := s.generateJobs(diff)

	// Sort by priority
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Priority != jobs[j].Priority {
			return jobs[i].Priority < jobs[j].Priority
		}
		return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
	})

	return jobs
}

// generateJobs creates maintenance jobs from a diff
func (s *Scheduler) generateJobs(diff *model.ModelDiff) []MaintenanceJob {
	var jobs []MaintenanceJob
	now := time.Now()

	// Handle added functions
	if s.config.AutoCreateTests {
		for _, fn := range diff.AddedFunctions {
			jobs = append(jobs, MaintenanceJob{
				ID:         generateJobID("create", fn.ID),
				Type:       JobTypeCreate,
				TargetID:   fn.ID,
				TargetType: "function",
				Priority:   PriorityMedium,
				Reason:     "New function added",
				CreatedAt:  now,
				OldCommit:  diff.OldCommit,
				NewCommit:  diff.NewCommit,
				NewEntity:  fn,
			})
		}
	}

	// Handle removed functions
	if s.config.RemoveOrphanedTests {
		for _, fn := range diff.RemovedFunctions {
			jobs = append(jobs, MaintenanceJob{
				ID:         generateJobID("remove", fn.ID),
				Type:       JobTypeRemove,
				TargetID:   fn.ID,
				TargetType: "function",
				Priority:   PriorityHigh,
				Reason:     "Function deleted",
				CreatedAt:  now,
				OldCommit:  diff.OldCommit,
				NewCommit:  diff.NewCommit,
				OldEntity:  fn,
			})
		}
	}

	// Handle modified functions
	for _, change := range diff.ModifiedFunctions {
		job := s.createFunctionChangeJob(change, diff.OldCommit, diff.NewCommit, now)
		if job != nil {
			jobs = append(jobs, *job)
		}
	}

	// Handle added endpoints
	if s.config.AutoCreateTests {
		for _, ep := range diff.AddedEndpoints {
			jobs = append(jobs, MaintenanceJob{
				ID:         generateJobID("create", ep.ID),
				Type:       JobTypeCreate,
				TargetID:   ep.ID,
				TargetType: "endpoint",
				Priority:   PriorityMedium,
				Reason:     "New endpoint added",
				CreatedAt:  now,
				OldCommit:  diff.OldCommit,
				NewCommit:  diff.NewCommit,
				NewEntity:  ep,
			})
		}
	}

	// Handle removed endpoints
	if s.config.RemoveOrphanedTests {
		for _, ep := range diff.RemovedEndpoints {
			jobs = append(jobs, MaintenanceJob{
				ID:         generateJobID("remove", ep.ID),
				Type:       JobTypeRemove,
				TargetID:   ep.ID,
				TargetType: "endpoint",
				Priority:   PriorityHigh,
				Reason:     "Endpoint deleted",
				CreatedAt:  now,
				OldCommit:  diff.OldCommit,
				NewCommit:  diff.NewCommit,
				OldEntity:  ep,
			})
		}
	}

	// Handle modified endpoints
	for _, change := range diff.ModifiedEndpoints {
		job := s.createEndpointChangeJob(change, diff.OldCommit, diff.NewCommit, now)
		if job != nil {
			jobs = append(jobs, *job)
		}
	}

	return jobs
}

// createFunctionChangeJob creates a job for a function change
func (s *Scheduler) createFunctionChangeJob(change model.FunctionChange, oldCommit, newCommit string, now time.Time) *MaintenanceJob {
	hasSignatureChange := containsString(change.ChangeTypes, "signature")
	hasBodyChange := containsString(change.ChangeTypes, "body")
	hasDecoratorChange := containsString(change.ChangeTypes, "decorators")

	// Signature changes require regeneration
	if hasSignatureChange && s.config.RegenerateOnSignatureChange {
		return &MaintenanceJob{
			ID:          generateJobID("regenerate", change.After.ID),
			Type:        JobTypeRegenerate,
			TargetID:    change.After.ID,
			TargetType:  "function",
			Priority:    PriorityHigh,
			Reason:      "Function signature changed",
			CreatedAt:   now,
			OldCommit:   oldCommit,
			NewCommit:   newCommit,
			ChangeTypes: change.ChangeTypes,
			OldEntity:   change.Before,
			NewEntity:   change.After,
		}
	}

	// Body-only changes may need update
	if hasBodyChange && s.config.UpdateOnBodyChange {
		return &MaintenanceJob{
			ID:          generateJobID("update", change.After.ID),
			Type:        JobTypeUpdate,
			TargetID:    change.After.ID,
			TargetType:  "function",
			Priority:    PriorityMedium,
			Reason:      "Function implementation changed",
			CreatedAt:   now,
			OldCommit:   oldCommit,
			NewCommit:   newCommit,
			ChangeTypes: change.ChangeTypes,
			OldEntity:   change.Before,
			NewEntity:   change.After,
		}
	}

	// Decorator changes are low priority
	if hasDecoratorChange {
		return &MaintenanceJob{
			ID:          generateJobID("update", change.After.ID),
			Type:        JobTypeUpdate,
			TargetID:    change.After.ID,
			TargetType:  "function",
			Priority:    PriorityLow,
			Reason:      "Function decorators changed",
			CreatedAt:   now,
			OldCommit:   oldCommit,
			NewCommit:   newCommit,
			ChangeTypes: change.ChangeTypes,
			OldEntity:   change.Before,
			NewEntity:   change.After,
		}
	}

	return nil
}

// createEndpointChangeJob creates a job for an endpoint change
func (s *Scheduler) createEndpointChangeJob(change model.EndpointChange, oldCommit, newCommit string, now time.Time) *MaintenanceJob {
	hasPathChange := containsString(change.ChangeTypes, "path")
	hasMethodChange := containsString(change.ChangeTypes, "method")
	hasHandlerChange := containsString(change.ChangeTypes, "handler")
	hasParamsChange := containsString(change.ChangeTypes, "params")

	// Path or method changes are breaking - require regeneration
	if hasPathChange || hasMethodChange {
		return &MaintenanceJob{
			ID:          generateJobID("regenerate", change.After.ID),
			Type:        JobTypeRegenerate,
			TargetID:    change.After.ID,
			TargetType:  "endpoint",
			Priority:    PriorityHigh,
			Reason:      "Endpoint path or method changed",
			CreatedAt:   now,
			OldCommit:   oldCommit,
			NewCommit:   newCommit,
			ChangeTypes: change.ChangeTypes,
			OldEntity:   change.Before,
			NewEntity:   change.After,
		}
	}

	// Handler or params changes need update
	if hasHandlerChange || hasParamsChange {
		return &MaintenanceJob{
			ID:          generateJobID("update", change.After.ID),
			Type:        JobTypeUpdate,
			TargetID:    change.After.ID,
			TargetType:  "endpoint",
			Priority:    PriorityMedium,
			Reason:      "Endpoint handler or params changed",
			CreatedAt:   now,
			OldCommit:   oldCommit,
			NewCommit:   newCommit,
			ChangeTypes: change.ChangeTypes,
			OldEntity:   change.Before,
			NewEntity:   change.After,
		}
	}

	return nil
}

// GetJobsByType filters jobs by type
func GetJobsByType(jobs []MaintenanceJob, jobType JobType) []MaintenanceJob {
	var filtered []MaintenanceJob
	for _, job := range jobs {
		if job.Type == jobType {
			filtered = append(filtered, job)
		}
	}
	return filtered
}

// GetJobsByPriority filters jobs by priority
func GetJobsByPriority(jobs []MaintenanceJob, priority JobPriority) []MaintenanceJob {
	var filtered []MaintenanceJob
	for _, job := range jobs {
		if job.Priority == priority {
			filtered = append(filtered, job)
		}
	}
	return filtered
}

// GetHighPriorityJobs returns all high priority jobs
func GetHighPriorityJobs(jobs []MaintenanceJob) []MaintenanceJob {
	return GetJobsByPriority(jobs, PriorityHigh)
}

// containsString checks if a slice contains a string
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// generateJobID creates a unique job ID
func generateJobID(prefix, targetID string) string {
	return prefix + "-" + targetID + "-" + time.Now().Format("20060102150405")
}
