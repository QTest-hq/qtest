package model

import (
	"testing"
	"time"
)

// =============================================================================
// SystemModel Tests
// =============================================================================

func TestSystemModel_Stats(t *testing.T) {
	model := &SystemModel{
		Modules:     []Module{{ID: "m1"}, {ID: "m2"}},
		Functions:   []Function{{ID: "f1"}, {ID: "f2"}, {ID: "f3"}},
		Types:       []TypeDef{{ID: "t1"}},
		Endpoints:   []Endpoint{{ID: "e1"}, {ID: "e2"}},
		Events:      []Event{{ID: "ev1"}},
		TestTargets: []TestTarget{{ID: "tt1"}, {ID: "tt2"}},
	}

	stats := model.Stats()

	tests := []struct {
		key  string
		want int
	}{
		{"modules", 2},
		{"functions", 3},
		{"types", 1},
		{"endpoints", 2},
		{"events", 1},
		{"test_targets", 2},
	}

	for _, tt := range tests {
		got := stats[tt.key]
		if got != tt.want {
			t.Errorf("Stats()[%s] = %d, want %d", tt.key, got, tt.want)
		}
	}
}

func TestSystemModel_GetFunction(t *testing.T) {
	model := &SystemModel{
		Functions: []Function{
			{ID: "func1", Name: "GetUser"},
			{ID: "func2", Name: "CreateUser"},
			{ID: "func3", Name: "DeleteUser"},
		},
	}

	t.Run("found", func(t *testing.T) {
		fn := model.GetFunction("func2")
		if fn == nil {
			t.Fatal("expected to find func2")
		}
		if fn.Name != "CreateUser" {
			t.Errorf("Name = %s, want CreateUser", fn.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		fn := model.GetFunction("nonexistent")
		if fn != nil {
			t.Errorf("expected nil, got %v", fn)
		}
	})
}

func TestSystemModel_GetEndpoint(t *testing.T) {
	model := &SystemModel{
		Endpoints: []Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users"},
			{ID: "ep2", Method: "POST", Path: "/users"},
		},
	}

	t.Run("found", func(t *testing.T) {
		ep := model.GetEndpoint("ep2")
		if ep == nil {
			t.Fatal("expected to find ep2")
		}
		if ep.Method != "POST" {
			t.Errorf("Method = %s, want POST", ep.Method)
		}
	})

	t.Run("not found", func(t *testing.T) {
		ep := model.GetEndpoint("nonexistent")
		if ep != nil {
			t.Errorf("expected nil, got %v", ep)
		}
	})
}

func TestSystemModel_GetExportedFunctions(t *testing.T) {
	model := &SystemModel{
		Functions: []Function{
			{ID: "f1", Name: "PublicFunc", Exported: true},
			{ID: "f2", Name: "privateFunc", Exported: false},
			{ID: "f3", Name: "AnotherPublic", Exported: true},
		},
	}

	exported := model.GetExportedFunctions()

	if len(exported) != 2 {
		t.Errorf("len(exported) = %d, want 2", len(exported))
	}

	for _, fn := range exported {
		if !fn.Exported {
			t.Errorf("function %s should be exported", fn.Name)
		}
	}
}

func TestSystemModel_GetFunctionsByModule(t *testing.T) {
	model := &SystemModel{
		Functions: []Function{
			{ID: "f1", Name: "Func1", Module: "mod1"},
			{ID: "f2", Name: "Func2", Module: "mod2"},
			{ID: "f3", Name: "Func3", Module: "mod1"},
			{ID: "f4", Name: "Func4", Module: "mod2"},
		},
	}

	mod1Funcs := model.GetFunctionsByModule("mod1")
	if len(mod1Funcs) != 2 {
		t.Errorf("len(mod1Funcs) = %d, want 2", len(mod1Funcs))
	}

	mod2Funcs := model.GetFunctionsByModule("mod2")
	if len(mod2Funcs) != 2 {
		t.Errorf("len(mod2Funcs) = %d, want 2", len(mod2Funcs))
	}

	emptyFuncs := model.GetFunctionsByModule("nonexistent")
	if len(emptyFuncs) != 0 {
		t.Errorf("len(emptyFuncs) = %d, want 0", len(emptyFuncs))
	}
}

// =============================================================================
// Module Tests
// =============================================================================

func TestModule_Fields(t *testing.T) {
	module := Module{
		ID:       "mod:main",
		Name:     "main",
		Path:     "/src/main",
		Language: "go",
		Files:    []string{"main.go", "utils.go"},
	}

	if module.ID != "mod:main" {
		t.Errorf("ID = %s, want mod:main", module.ID)
	}
	if module.Language != "go" {
		t.Errorf("Language = %s, want go", module.Language)
	}
	if len(module.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(module.Files))
	}
}

// =============================================================================
// Function Tests
// =============================================================================

func TestFunction_Fields(t *testing.T) {
	fn := Function{
		ID:        "fn:main:GetUser:10",
		Name:      "GetUser",
		Module:    "main",
		File:      "user.go",
		StartLine: 10,
		EndLine:   25,
		Parameters: []Parameter{
			{Name: "id", Type: "int", Optional: false},
		},
		Returns: []Parameter{
			{Name: "", Type: "*User"},
			{Name: "", Type: "error"},
		},
		Exported:   true,
		Async:      false,
		Pure:       true,
		Complexity: 5,
		LOC:        15,
	}

	if fn.Name != "GetUser" {
		t.Errorf("Name = %s, want GetUser", fn.Name)
	}
	if len(fn.Parameters) != 1 {
		t.Errorf("len(Parameters) = %d, want 1", len(fn.Parameters))
	}
	if len(fn.Returns) != 2 {
		t.Errorf("len(Returns) = %d, want 2", len(fn.Returns))
	}
	if fn.Complexity != 5 {
		t.Errorf("Complexity = %d, want 5", fn.Complexity)
	}
}

// =============================================================================
// Parameter Tests
// =============================================================================

func TestParameter_Fields(t *testing.T) {
	tests := []struct {
		name  string
		param Parameter
	}{
		{
			name: "required parameter",
			param: Parameter{
				Name:     "id",
				Type:     "int",
				Optional: false,
			},
		},
		{
			name: "optional parameter with default",
			param: Parameter{
				Name:     "limit",
				Type:     "int",
				Optional: true,
				Default:  "10",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.param.Name == "" {
				t.Error("Name should not be empty")
			}
		})
	}
}

// =============================================================================
// TypeDef Tests
// =============================================================================

func TestTypeDef_Fields(t *testing.T) {
	typeDef := TypeDef{
		ID:         "type:User",
		Name:       "User",
		Kind:       TypeKindStruct,
		Module:     "models",
		File:       "user.go",
		Line:       5,
		Fields:     []Field{{Name: "ID", Type: "int", Exported: true}},
		Methods:    []string{"fn:GetName", "fn:SetName"},
		Implements: []string{"Stringer"},
		Exported:   true,
	}

	if typeDef.Kind != TypeKindStruct {
		t.Errorf("Kind = %s, want struct", typeDef.Kind)
	}
	if len(typeDef.Fields) != 1 {
		t.Errorf("len(Fields) = %d, want 1", len(typeDef.Fields))
	}
	if len(typeDef.Methods) != 2 {
		t.Errorf("len(Methods) = %d, want 2", len(typeDef.Methods))
	}
}

func TestTypeKind_Constants(t *testing.T) {
	tests := []struct {
		kind TypeKind
		want string
	}{
		{TypeKindStruct, "struct"},
		{TypeKindClass, "class"},
		{TypeKindInterface, "interface"},
		{TypeKindEnum, "enum"},
		{TypeKindAlias, "alias"},
	}

	for _, tt := range tests {
		if string(tt.kind) != tt.want {
			t.Errorf("TypeKind %v = %s, want %s", tt.kind, string(tt.kind), tt.want)
		}
	}
}

// =============================================================================
// Field Tests
// =============================================================================

func TestField_Fields(t *testing.T) {
	field := Field{
		Name:     "ID",
		Type:     "int64",
		Exported: true,
		Tags:     `json:"id" db:"id"`,
	}

	if field.Name != "ID" {
		t.Errorf("Name = %s, want ID", field.Name)
	}
	if field.Tags == "" {
		t.Error("Tags should not be empty")
	}
}

// =============================================================================
// Endpoint Tests
// =============================================================================

func TestEndpoint_Fields(t *testing.T) {
	endpoint := Endpoint{
		ID:          "ep:GET:/users/:id",
		Method:      "GET",
		Path:        "/users/:id",
		Handler:     "GetUserHandler",
		File:        "routes.go",
		Line:        42,
		PathParams:  []string{"id"},
		QueryParams: []string{"include"},
		Framework:   "gin",
		Middleware:  []string{"AuthMiddleware"},
	}

	if endpoint.Method != "GET" {
		t.Errorf("Method = %s, want GET", endpoint.Method)
	}
	if len(endpoint.PathParams) != 1 {
		t.Errorf("len(PathParams) = %d, want 1", len(endpoint.PathParams))
	}
	if endpoint.Framework != "gin" {
		t.Errorf("Framework = %s, want gin", endpoint.Framework)
	}
}

// =============================================================================
// Event Tests
// =============================================================================

func TestEvent_Fields(t *testing.T) {
	event := Event{
		ID:      "event:user.created",
		Name:    "user.created",
		Kind:    "queue",
		Handler: "HandleUserCreated",
		File:    "events.go",
		Line:    100,
	}

	if event.Kind != "queue" {
		t.Errorf("Kind = %s, want queue", event.Kind)
	}
	if event.Handler != "HandleUserCreated" {
		t.Errorf("Handler = %s, want HandleUserCreated", event.Handler)
	}
}

// =============================================================================
// CallEdge Tests
// =============================================================================

func TestCallEdge_Fields(t *testing.T) {
	edge := CallEdge{
		Caller: "fn:main",
		Callee: "fn:helper",
		File:   "main.go",
		Line:   25,
	}

	if edge.Caller != "fn:main" {
		t.Errorf("Caller = %s, want fn:main", edge.Caller)
	}
	if edge.Callee != "fn:helper" {
		t.Errorf("Callee = %s, want fn:helper", edge.Callee)
	}
}

// =============================================================================
// RiskScore Tests
// =============================================================================

func TestRiskScore_Fields(t *testing.T) {
	score := RiskScore{
		FunctionID: "fn:CriticalFunction",
		Score:      0.85,
		Complexity: 0.7,
		Centrality: 0.9,
		Churn:      0.8,
		HasTests:   false,
	}

	if score.Score != 0.85 {
		t.Errorf("Score = %f, want 0.85", score.Score)
	}
	if score.HasTests {
		t.Error("HasTests should be false")
	}
}

// =============================================================================
// TestTarget Tests
// =============================================================================

func TestTestTarget_Fields(t *testing.T) {
	target := TestTarget{
		ID:         "tt:1",
		Kind:       TargetKindUnit,
		FunctionID: "fn:GetUser",
		Priority:   1,
		RiskScore:  0.9,
		Reason:     "High complexity, no tests",
	}

	if target.Kind != TargetKindUnit {
		t.Errorf("Kind = %s, want unit", target.Kind)
	}
	if target.Priority != 1 {
		t.Errorf("Priority = %d, want 1", target.Priority)
	}
}

func TestTargetKind_Constants(t *testing.T) {
	tests := []struct {
		kind TargetKind
		want string
	}{
		{TargetKindUnit, "unit"},
		{TargetKindIntegration, "integration"},
		{TargetKindAPI, "api"},
		{TargetKindE2E, "e2e"},
	}

	for _, tt := range tests {
		if string(tt.kind) != tt.want {
			t.Errorf("TargetKind %v = %s, want %s", tt.kind, string(tt.kind), tt.want)
		}
	}
}

// =============================================================================
// Empty Model Tests
// =============================================================================

func TestSystemModel_Empty(t *testing.T) {
	model := &SystemModel{}

	stats := model.Stats()
	for key, val := range stats {
		if val != 0 {
			t.Errorf("empty model Stats()[%s] = %d, want 0", key, val)
		}
	}

	if fn := model.GetFunction("any"); fn != nil {
		t.Error("GetFunction on empty model should return nil")
	}

	if ep := model.GetEndpoint("any"); ep != nil {
		t.Error("GetEndpoint on empty model should return nil")
	}

	exported := model.GetExportedFunctions()
	if len(exported) != 0 {
		t.Error("GetExportedFunctions on empty model should return empty slice")
	}
}

// =============================================================================
// Metadata Tests
// =============================================================================

func TestSystemModel_Metadata(t *testing.T) {
	now := time.Now()
	model := &SystemModel{
		ID:         "model-123",
		Repository: "github.com/test/repo",
		Branch:     "main",
		CommitSHA:  "abc123",
		CreatedAt:  now,
		Languages:  []string{"go", "python"},
	}

	if model.ID != "model-123" {
		t.Errorf("ID = %s, want model-123", model.ID)
	}
	if model.Branch != "main" {
		t.Errorf("Branch = %s, want main", model.Branch)
	}
	if len(model.Languages) != 2 {
		t.Errorf("len(Languages) = %d, want 2", len(model.Languages))
	}
	if model.CreatedAt != now {
		t.Error("CreatedAt mismatch")
	}
}
