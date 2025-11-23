package maintenance

import (
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScheduler(t *testing.T) {
	config := DefaultSchedulerConfig()
	s := NewScheduler(config)
	assert.NotNil(t, s)
	assert.True(t, s.config.AutoCreateTests)
	assert.True(t, s.config.RegenerateOnSignatureChange)
}

func TestDefaultSchedulerConfig(t *testing.T) {
	config := DefaultSchedulerConfig()
	assert.True(t, config.AutoCreateTests)
	assert.True(t, config.RegenerateOnSignatureChange)
	assert.True(t, config.UpdateOnBodyChange)
	assert.True(t, config.RemoveOrphanedTests)
}

func TestOnPush_NilModels(t *testing.T) {
	s := NewScheduler(DefaultSchedulerConfig())

	jobs := s.OnPush(nil, nil)
	assert.Nil(t, jobs)

	jobs = s.OnPush(&model.SystemModel{}, nil)
	assert.Nil(t, jobs)

	jobs = s.OnPush(nil, &model.SystemModel{})
	assert.Nil(t, jobs)
}

func TestOnPush_NoChanges(t *testing.T) {
	s := NewScheduler(DefaultSchedulerConfig())

	oldModel := &model.SystemModel{
		Functions: []model.Function{{ID: "fn1", Name: "Add"}},
	}
	newModel := &model.SystemModel{
		Functions: []model.Function{{ID: "fn1", Name: "Add"}},
	}

	jobs := s.OnPush(oldModel, newModel)
	assert.Empty(t, jobs)
}

func TestOnPush_AddedFunction(t *testing.T) {
	s := NewScheduler(DefaultSchedulerConfig())

	oldModel := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{{ID: "fn1", Name: "Add"}},
	}
	newModel := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add"},
			{ID: "fn2", Name: "Subtract"},
		},
	}

	jobs := s.OnPush(oldModel, newModel)

	require.Len(t, jobs, 1)
	assert.Equal(t, JobTypeCreate, jobs[0].Type)
	assert.Equal(t, "fn2", jobs[0].TargetID)
	assert.Equal(t, "function", jobs[0].TargetType)
	assert.Equal(t, PriorityMedium, jobs[0].Priority)
}

func TestOnPush_RemovedFunction(t *testing.T) {
	s := NewScheduler(DefaultSchedulerConfig())

	oldModel := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add"},
			{ID: "fn2", Name: "Subtract"},
		},
	}
	newModel := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{{ID: "fn1", Name: "Add"}},
	}

	jobs := s.OnPush(oldModel, newModel)

	require.Len(t, jobs, 1)
	assert.Equal(t, JobTypeRemove, jobs[0].Type)
	assert.Equal(t, "fn2", jobs[0].TargetID)
	assert.Equal(t, PriorityHigh, jobs[0].Priority)
}

func TestOnPush_ModifiedFunctionSignature(t *testing.T) {
	s := NewScheduler(DefaultSchedulerConfig())

	oldModel := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Parameters: []model.Parameter{{Name: "a", Type: "int"}}},
		},
	}
	newModel := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Parameters: []model.Parameter{
				{Name: "a", Type: "int"},
				{Name: "b", Type: "int"},
			}},
		},
	}

	jobs := s.OnPush(oldModel, newModel)

	require.Len(t, jobs, 1)
	assert.Equal(t, JobTypeRegenerate, jobs[0].Type)
	assert.Equal(t, "fn1", jobs[0].TargetID)
	assert.Equal(t, PriorityHigh, jobs[0].Priority)
	assert.Contains(t, jobs[0].ChangeTypes, "signature")
}

func TestOnPush_ModifiedFunctionBody(t *testing.T) {
	s := NewScheduler(DefaultSchedulerConfig())

	oldModel := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Body: "return a + b"},
		},
	}
	newModel := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Body: "return a + b + c"},
		},
	}

	jobs := s.OnPush(oldModel, newModel)

	require.Len(t, jobs, 1)
	assert.Equal(t, JobTypeUpdate, jobs[0].Type)
	assert.Equal(t, "fn1", jobs[0].TargetID)
	assert.Equal(t, PriorityMedium, jobs[0].Priority)
}

func TestOnPush_AddedEndpoint(t *testing.T) {
	s := NewScheduler(DefaultSchedulerConfig())

	oldModel := &model.SystemModel{
		CommitSHA: "abc123",
		Endpoints: []model.Endpoint{{ID: "ep1", Method: "GET", Path: "/users"}},
	}
	newModel := &model.SystemModel{
		CommitSHA: "def456",
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users"},
			{ID: "ep2", Method: "POST", Path: "/users"},
		},
	}

	jobs := s.OnPush(oldModel, newModel)

	require.Len(t, jobs, 1)
	assert.Equal(t, JobTypeCreate, jobs[0].Type)
	assert.Equal(t, "ep2", jobs[0].TargetID)
	assert.Equal(t, "endpoint", jobs[0].TargetType)
}

func TestOnPush_ModifiedEndpointPath(t *testing.T) {
	s := NewScheduler(DefaultSchedulerConfig())

	oldModel := &model.SystemModel{
		CommitSHA: "abc123",
		Endpoints: []model.Endpoint{{ID: "ep1", Method: "GET", Path: "/users"}},
	}
	newModel := &model.SystemModel{
		CommitSHA: "def456",
		Endpoints: []model.Endpoint{{ID: "ep1", Method: "GET", Path: "/api/users"}},
	}

	jobs := s.OnPush(oldModel, newModel)

	require.Len(t, jobs, 1)
	assert.Equal(t, JobTypeRegenerate, jobs[0].Type)
	assert.Equal(t, "ep1", jobs[0].TargetID)
	assert.Equal(t, PriorityHigh, jobs[0].Priority)
}

func TestOnPush_ModifiedEndpointHandler(t *testing.T) {
	s := NewScheduler(DefaultSchedulerConfig())

	oldModel := &model.SystemModel{
		CommitSHA: "abc123",
		Endpoints: []model.Endpoint{{ID: "ep1", Method: "GET", Path: "/users", Handler: "getUsers"}},
	}
	newModel := &model.SystemModel{
		CommitSHA: "def456",
		Endpoints: []model.Endpoint{{ID: "ep1", Method: "GET", Path: "/users", Handler: "listUsers"}},
	}

	jobs := s.OnPush(oldModel, newModel)

	require.Len(t, jobs, 1)
	assert.Equal(t, JobTypeUpdate, jobs[0].Type)
	assert.Equal(t, PriorityMedium, jobs[0].Priority)
}

func TestOnPush_JobPrioritySorting(t *testing.T) {
	s := NewScheduler(DefaultSchedulerConfig())

	// Mix of changes: removed (high), signature change (high), body change (medium), new (medium)
	oldModel := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "ToRemove"},
			{ID: "fn2", Name: "SignatureChange", Parameters: []model.Parameter{{Name: "a", Type: "int"}}},
			{ID: "fn3", Name: "BodyChange", Body: "old"},
		},
	}
	newModel := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn2", Name: "SignatureChange", Parameters: []model.Parameter{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}}},
			{ID: "fn3", Name: "BodyChange", Body: "new"},
			{ID: "fn4", Name: "NewFunction"},
		},
	}

	jobs := s.OnPush(oldModel, newModel)

	require.Len(t, jobs, 4)
	// High priority jobs should come first
	assert.Equal(t, PriorityHigh, jobs[0].Priority)
	assert.Equal(t, PriorityHigh, jobs[1].Priority)
	// Medium priority jobs after
	assert.Equal(t, PriorityMedium, jobs[2].Priority)
	assert.Equal(t, PriorityMedium, jobs[3].Priority)
}

func TestOnPush_ConfigDisableAutoCreate(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.AutoCreateTests = false
	s := NewScheduler(config)

	oldModel := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{{ID: "fn1", Name: "Add"}},
	}
	newModel := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add"},
			{ID: "fn2", Name: "New"},
		},
	}

	jobs := s.OnPush(oldModel, newModel)
	assert.Empty(t, jobs) // No create job because AutoCreateTests is disabled
}

func TestOnPush_ConfigDisableRemoveOrphaned(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.RemoveOrphanedTests = false
	s := NewScheduler(config)

	oldModel := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add"},
			{ID: "fn2", Name: "Remove"},
		},
	}
	newModel := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{{ID: "fn1", Name: "Add"}},
	}

	jobs := s.OnPush(oldModel, newModel)
	assert.Empty(t, jobs) // No remove job because RemoveOrphanedTests is disabled
}

func TestGetJobsByType(t *testing.T) {
	jobs := []MaintenanceJob{
		{Type: JobTypeCreate, TargetID: "fn1"},
		{Type: JobTypeRemove, TargetID: "fn2"},
		{Type: JobTypeCreate, TargetID: "fn3"},
		{Type: JobTypeRegenerate, TargetID: "fn4"},
	}

	createJobs := GetJobsByType(jobs, JobTypeCreate)
	assert.Len(t, createJobs, 2)

	removeJobs := GetJobsByType(jobs, JobTypeRemove)
	assert.Len(t, removeJobs, 1)

	regenerateJobs := GetJobsByType(jobs, JobTypeRegenerate)
	assert.Len(t, regenerateJobs, 1)
}

func TestGetJobsByPriority(t *testing.T) {
	jobs := []MaintenanceJob{
		{Priority: PriorityHigh, TargetID: "fn1"},
		{Priority: PriorityMedium, TargetID: "fn2"},
		{Priority: PriorityHigh, TargetID: "fn3"},
		{Priority: PriorityLow, TargetID: "fn4"},
	}

	highJobs := GetJobsByPriority(jobs, PriorityHigh)
	assert.Len(t, highJobs, 2)

	mediumJobs := GetJobsByPriority(jobs, PriorityMedium)
	assert.Len(t, mediumJobs, 1)

	lowJobs := GetJobsByPriority(jobs, PriorityLow)
	assert.Len(t, lowJobs, 1)
}

func TestGetHighPriorityJobs(t *testing.T) {
	jobs := []MaintenanceJob{
		{Priority: PriorityHigh, TargetID: "fn1"},
		{Priority: PriorityMedium, TargetID: "fn2"},
		{Priority: PriorityHigh, TargetID: "fn3"},
	}

	highJobs := GetHighPriorityJobs(jobs)
	assert.Len(t, highJobs, 2)
}

func TestOnPush_ComplexScenario(t *testing.T) {
	s := NewScheduler(DefaultSchedulerConfig())

	oldModel := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Keep"},
			{ID: "fn2", Name: "Remove"},
			{ID: "fn3", Name: "ChangeSignature", Parameters: []model.Parameter{{Name: "a", Type: "int"}}},
			{ID: "fn4", Name: "ChangeBody", Body: "old body"},
		},
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/old"},
			{ID: "ep2", Method: "DELETE", Path: "/remove"},
		},
	}
	newModel := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn1", Name: "Keep"},
			{ID: "fn3", Name: "ChangeSignature", Parameters: []model.Parameter{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}}},
			{ID: "fn4", Name: "ChangeBody", Body: "new body"},
			{ID: "fn5", Name: "NewFunction"},
		},
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/new"},
			{ID: "ep3", Method: "POST", Path: "/create"},
		},
	}

	jobs := s.OnPush(oldModel, newModel)

	// Expected jobs:
	// - fn2: remove (high)
	// - fn3: regenerate (high, signature change)
	// - fn4: update (medium, body change)
	// - fn5: create (medium)
	// - ep1: regenerate (high, path change)
	// - ep2: remove (high)
	// - ep3: create (medium)

	assert.Len(t, jobs, 7)

	// Verify high priority jobs are first
	highJobs := GetHighPriorityJobs(jobs)
	assert.Len(t, highJobs, 4) // fn2 remove, fn3 regen, ep1 regen, ep2 remove

	// Verify job types
	createJobs := GetJobsByType(jobs, JobTypeCreate)
	assert.Len(t, createJobs, 2) // fn5, ep3

	removeJobs := GetJobsByType(jobs, JobTypeRemove)
	assert.Len(t, removeJobs, 2) // fn2, ep2

	regenerateJobs := GetJobsByType(jobs, JobTypeRegenerate)
	assert.Len(t, regenerateJobs, 2) // fn3, ep1

	updateJobs := GetJobsByType(jobs, JobTypeUpdate)
	assert.Len(t, updateJobs, 1) // fn4
}
