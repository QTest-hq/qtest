package differ

import (
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	d := New()
	assert.NotNil(t, d)
}

func TestDiffSystemModels_NoChanges(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Parameters: []model.Parameter{{Name: "a", Type: "int"}}},
		},
	}
	new := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Parameters: []model.Parameter{{Name: "a", Type: "int"}}},
		},
	}

	diff := d.DiffSystemModels(old, new)

	assert.False(t, diff.HasChanges())
	assert.Equal(t, 0, diff.Stats.TotalChanges)
}

func TestDiffSystemModels_AddedFunction(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add"},
		},
	}
	new := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add"},
			{ID: "fn2", Name: "Subtract"},
		},
	}

	diff := d.DiffSystemModels(old, new)

	assert.True(t, diff.HasChanges())
	assert.Equal(t, 1, len(diff.AddedFunctions))
	assert.Equal(t, "fn2", diff.AddedFunctions[0].ID)
	assert.Equal(t, 1, diff.Stats.FunctionsAdded)
}

func TestDiffSystemModels_RemovedFunction(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add"},
			{ID: "fn2", Name: "Subtract"},
		},
	}
	new := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add"},
		},
	}

	diff := d.DiffSystemModels(old, new)

	assert.True(t, diff.HasChanges())
	assert.Equal(t, 1, len(diff.RemovedFunctions))
	assert.Equal(t, "fn2", diff.RemovedFunctions[0].ID)
	assert.Equal(t, 1, diff.Stats.FunctionsRemoved)
}

func TestDiffSystemModels_ModifiedFunctionSignature(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Parameters: []model.Parameter{
				{Name: "a", Type: "int"},
			}},
		},
	}
	new := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Parameters: []model.Parameter{
				{Name: "a", Type: "int"},
				{Name: "b", Type: "int"},
			}},
		},
	}

	diff := d.DiffSystemModels(old, new)

	assert.True(t, diff.HasChanges())
	assert.True(t, diff.HasSignatureChanges())
	require.Equal(t, 1, len(diff.ModifiedFunctions))
	assert.Contains(t, diff.ModifiedFunctions[0].ChangeTypes, "signature")
}

func TestDiffSystemModels_ModifiedFunctionBody(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Body: "return a + b"},
		},
	}
	new := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Body: "return a + b + c"},
		},
	}

	diff := d.DiffSystemModels(old, new)

	assert.True(t, diff.HasChanges())
	require.Equal(t, 1, len(diff.ModifiedFunctions))
	assert.Contains(t, diff.ModifiedFunctions[0].ChangeTypes, "body")
}

func TestDiffSystemModels_ModifiedFunctionDecorators(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "handler", Decorators: []string{"@route('/api')"}},
		},
	}
	new := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn1", Name: "handler", Decorators: []string{"@route('/api')", "@auth"}},
		},
	}

	diff := d.DiffSystemModels(old, new)

	assert.True(t, diff.HasChanges())
	require.Equal(t, 1, len(diff.ModifiedFunctions))
	assert.Contains(t, diff.ModifiedFunctions[0].ChangeTypes, "decorators")
}

func TestDiffSystemModels_AddedType(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Types: []model.TypeDef{
			{ID: "t1", Name: "User"},
		},
	}
	new := &model.SystemModel{
		CommitSHA: "def456",
		Types: []model.TypeDef{
			{ID: "t1", Name: "User"},
			{ID: "t2", Name: "Order"},
		},
	}

	diff := d.DiffSystemModels(old, new)

	assert.True(t, diff.HasChanges())
	assert.Equal(t, 1, len(diff.AddedTypes))
	assert.Equal(t, "t2", diff.AddedTypes[0].ID)
	assert.Equal(t, 1, diff.Stats.TypesAdded)
}

func TestDiffSystemModels_ModifiedTypeFields(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Types: []model.TypeDef{
			{ID: "t1", Name: "User", Fields: []model.Field{
				{Name: "ID", Type: "int"},
			}},
		},
	}
	new := &model.SystemModel{
		CommitSHA: "def456",
		Types: []model.TypeDef{
			{ID: "t1", Name: "User", Fields: []model.Field{
				{Name: "ID", Type: "int"},
				{Name: "Email", Type: "string"},
			}},
		},
	}

	diff := d.DiffSystemModels(old, new)

	assert.True(t, diff.HasChanges())
	require.Equal(t, 1, len(diff.ModifiedTypes))
	assert.Contains(t, diff.ModifiedTypes[0].ChangeTypes, "fields")
}

func TestDiffSystemModels_AddedEndpoint(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users"},
		},
	}
	new := &model.SystemModel{
		CommitSHA: "def456",
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users"},
			{ID: "ep2", Method: "POST", Path: "/users"},
		},
	}

	diff := d.DiffSystemModels(old, new)

	assert.True(t, diff.HasChanges())
	assert.True(t, diff.HasEndpointChanges())
	assert.Equal(t, 1, len(diff.AddedEndpoints))
	assert.Equal(t, "ep2", diff.AddedEndpoints[0].ID)
}

func TestDiffSystemModels_ModifiedEndpointPath(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users"},
		},
	}
	new := &model.SystemModel{
		CommitSHA: "def456",
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/api/users"},
		},
	}

	diff := d.DiffSystemModels(old, new)

	assert.True(t, diff.HasChanges())
	require.Equal(t, 1, len(diff.ModifiedEndpoints))
	assert.Contains(t, diff.ModifiedEndpoints[0].ChangeTypes, "path")
}

func TestDiffSystemModels_ModifiedEndpointHandler(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users", Handler: "handleGetUsers"},
		},
	}
	new := &model.SystemModel{
		CommitSHA: "def456",
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users", Handler: "handleListUsers"},
		},
	}

	diff := d.DiffSystemModels(old, new)

	assert.True(t, diff.HasChanges())
	require.Equal(t, 1, len(diff.ModifiedEndpoints))
	assert.Contains(t, diff.ModifiedEndpoints[0].ChangeTypes, "handler")
}

func TestGetAffectedFunctionIDs(t *testing.T) {
	diff := &model.ModelDiff{
		AddedFunctions: []model.Function{
			{ID: "fn1"},
		},
		RemovedFunctions: []model.Function{
			{ID: "fn2"},
		},
		ModifiedFunctions: []model.FunctionChange{
			{After: model.Function{ID: "fn3"}},
		},
		Stats: model.DiffStats{TotalChanges: 3},
	}

	ids := diff.GetAffectedFunctionIDs()

	assert.Len(t, ids, 3)
	assert.Contains(t, ids, "fn1")
	assert.Contains(t, ids, "fn2")
	assert.Contains(t, ids, "fn3")
}

func TestGetAffectedEndpointIDs(t *testing.T) {
	diff := &model.ModelDiff{
		AddedEndpoints: []model.Endpoint{
			{ID: "ep1"},
		},
		RemovedEndpoints: []model.Endpoint{
			{ID: "ep2"},
		},
		ModifiedEndpoints: []model.EndpointChange{
			{After: model.Endpoint{ID: "ep3"}},
		},
		Stats: model.DiffStats{TotalChanges: 3},
	}

	ids := diff.GetAffectedEndpointIDs()

	assert.Len(t, ids, 3)
	assert.Contains(t, ids, "ep1")
	assert.Contains(t, ids, "ep2")
	assert.Contains(t, ids, "ep3")
}

func TestGetImpactedTests(t *testing.T) {
	d := New()

	diff := &model.ModelDiff{
		ModifiedFunctions: []model.FunctionChange{
			{
				After:       model.Function{ID: "fn1"},
				ChangeTypes: []string{"signature"},
			},
		},
		RemovedFunctions: []model.Function{
			{ID: "fn2"},
		},
	}

	testMapping := map[string][]string{
		"fn1": {"test1", "test2"},
		"fn2": {"test3"},
		"fn3": {"test4"}, // Not affected
	}

	impacted := d.GetImpactedTests(diff, testMapping)

	assert.Len(t, impacted, 3)
	assert.Contains(t, impacted, "test1")
	assert.Contains(t, impacted, "test2")
	assert.Contains(t, impacted, "test3")
}

func TestDiffSystemModels_ComplexScenario(t *testing.T) {
	d := New()

	old := &model.SystemModel{
		CommitSHA: "abc123",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Parameters: []model.Parameter{{Name: "a", Type: "int"}}},
			{ID: "fn2", Name: "Remove", Body: "old body"},
			{ID: "fn3", Name: "ToBeDeleted"},
		},
		Types: []model.TypeDef{
			{ID: "t1", Name: "User", Fields: []model.Field{{Name: "ID", Type: "int"}}},
		},
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users"},
		},
	}

	new := &model.SystemModel{
		CommitSHA: "def456",
		Functions: []model.Function{
			{ID: "fn1", Name: "Add", Parameters: []model.Parameter{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}}}, // signature change
			{ID: "fn2", Name: "Remove", Body: "new body"}, // body change
			{ID: "fn4", Name: "NewFunction"},              // added
		},
		Types: []model.TypeDef{
			{ID: "t1", Name: "User", Fields: []model.Field{{Name: "ID", Type: "int"}, {Name: "Name", Type: "string"}}}, // field added
			{ID: "t2", Name: "Order"}, // added
		},
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/api/users"}, // path change
			{ID: "ep2", Method: "POST", Path: "/users"},    // added
		},
	}

	diff := d.DiffSystemModels(old, new)

	// Functions
	assert.Equal(t, 1, len(diff.AddedFunctions))
	assert.Equal(t, 1, len(diff.RemovedFunctions))
	assert.Equal(t, 2, len(diff.ModifiedFunctions))

	// Types
	assert.Equal(t, 1, len(diff.AddedTypes))
	assert.Equal(t, 0, len(diff.RemovedTypes))
	assert.Equal(t, 1, len(diff.ModifiedTypes))

	// Endpoints
	assert.Equal(t, 1, len(diff.AddedEndpoints))
	assert.Equal(t, 0, len(diff.RemovedEndpoints))
	assert.Equal(t, 1, len(diff.ModifiedEndpoints))

	// Stats
	assert.Equal(t, 1, diff.Stats.FunctionsAdded)
	assert.Equal(t, 1, diff.Stats.FunctionsRemoved)
	assert.Equal(t, 2, diff.Stats.FunctionsModified)
	assert.Equal(t, 1, diff.Stats.TypesAdded)
	assert.Equal(t, 1, diff.Stats.TypesModified)
	assert.Equal(t, 1, diff.Stats.EndpointsAdded)
	assert.Equal(t, 1, diff.Stats.EndpointsModified)
	assert.Equal(t, 8, diff.Stats.TotalChanges)
}
