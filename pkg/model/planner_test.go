package model

import (
	"testing"
)

// =============================================================================
// PlannerConfig Tests
// =============================================================================

func TestDefaultPlannerConfig(t *testing.T) {
	config := DefaultPlannerConfig()

	if config.HighRiskThreshold != 0.7 {
		t.Errorf("HighRiskThreshold = %f, want 0.7", config.HighRiskThreshold)
	}
	if config.MediumRiskThreshold != 0.4 {
		t.Errorf("MediumRiskThreshold = %f, want 0.4", config.MediumRiskThreshold)
	}
	if config.UnitTestRatio != 0.7 {
		t.Errorf("UnitTestRatio = %f, want 0.7", config.UnitTestRatio)
	}
	if config.APITestRatio != 0.2 {
		t.Errorf("APITestRatio = %f, want 0.2", config.APITestRatio)
	}
	if config.E2ETestRatio != 0.1 {
		t.Errorf("E2ETestRatio = %f, want 0.1", config.E2ETestRatio)
	}
	if config.MaxIntents != 0 {
		t.Errorf("MaxIntents = %d, want 0 (unlimited)", config.MaxIntents)
	}
}

func TestPlannerConfig_Fields(t *testing.T) {
	config := PlannerConfig{
		HighRiskThreshold:   0.8,
		MediumRiskThreshold: 0.5,
		UnitTestRatio:       0.6,
		APITestRatio:        0.3,
		E2ETestRatio:        0.1,
		MaxIntents:          100,
	}

	if config.HighRiskThreshold != 0.8 {
		t.Errorf("HighRiskThreshold = %f, want 0.8", config.HighRiskThreshold)
	}
	if config.MaxIntents != 100 {
		t.Errorf("MaxIntents = %d, want 100", config.MaxIntents)
	}
}

// =============================================================================
// Planner Tests
// =============================================================================

func TestNewPlanner(t *testing.T) {
	config := DefaultPlannerConfig()
	planner := NewPlanner(config)

	if planner == nil {
		t.Fatal("NewPlanner() returned nil")
	}
	if planner.config.HighRiskThreshold != 0.7 {
		t.Errorf("config.HighRiskThreshold = %f, want 0.7", planner.config.HighRiskThreshold)
	}
}

func TestPlanner_Plan_Basic(t *testing.T) {
	config := DefaultPlannerConfig()
	planner := NewPlanner(config)

	model := &SystemModel{
		ID:         "model-1",
		Repository: "test-repo",
		Functions: []Function{
			{ID: "fn1", Name: "GetUser", Exported: true},
			{ID: "fn2", Name: "CreateUser", Exported: true},
			{ID: "fn3", Name: "helper", Exported: false},
		},
		RiskScores: map[string]RiskScore{
			"fn1": {FunctionID: "fn1", Score: 0.8}, // High risk
			"fn2": {FunctionID: "fn2", Score: 0.5}, // Medium risk
			"fn3": {FunctionID: "fn3", Score: 0.1}, // Low risk
		},
	}

	plan, err := planner.Plan(model)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	if plan == nil {
		t.Fatal("plan should not be nil")
	}
	if plan.ModelID != "model-1" {
		t.Errorf("ModelID = %s, want model-1", plan.ModelID)
	}
	if plan.Repository != "test-repo" {
		t.Errorf("Repository = %s, want test-repo", plan.Repository)
	}

	// Only exported functions should have intents
	if plan.UnitTests != 2 {
		t.Errorf("UnitTests = %d, want 2", plan.UnitTests)
	}
}

func TestPlanner_Plan_WithEndpoints(t *testing.T) {
	config := DefaultPlannerConfig()
	planner := NewPlanner(config)

	model := &SystemModel{
		ID:         "model-1",
		Repository: "test-repo",
		Endpoints: []Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users"},
			{ID: "ep2", Method: "POST", Path: "/users"},
		},
		Functions: []Function{
			{ID: "fn1", Name: "GetUser", Exported: true},
		},
		RiskScores: map[string]RiskScore{
			"fn1": {FunctionID: "fn1", Score: 0.5},
		},
	}

	plan, err := planner.Plan(model)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	if plan.APITests != 2 {
		t.Errorf("APITests = %d, want 2", plan.APITests)
	}
	if plan.UnitTests != 1 {
		t.Errorf("UnitTests = %d, want 1", plan.UnitTests)
	}
	if plan.TotalTests != 3 {
		t.Errorf("TotalTests = %d, want 3", plan.TotalTests)
	}
}

func TestPlanner_Plan_SkipsHandlers(t *testing.T) {
	config := DefaultPlannerConfig()
	planner := NewPlanner(config)

	model := &SystemModel{
		ID:         "model-1",
		Repository: "test-repo",
		Endpoints: []Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users", Handler: "GetUsersHandler"},
		},
		Functions: []Function{
			{ID: "fn1", Name: "GetUsersHandler", Exported: true}, // This is a handler
			{ID: "fn2", Name: "GetUser", Exported: true},          // Regular function
		},
		RiskScores: map[string]RiskScore{
			"fn1": {FunctionID: "fn1", Score: 0.5},
			"fn2": {FunctionID: "fn2", Score: 0.5},
		},
	}

	plan, err := planner.Plan(model)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	// Handler should be skipped for unit tests (already covered by API test)
	if plan.UnitTests != 1 {
		t.Errorf("UnitTests = %d, want 1 (handler should be skipped)", plan.UnitTests)
	}
	if plan.APITests != 1 {
		t.Errorf("APITests = %d, want 1", plan.APITests)
	}
}

func TestPlanner_Plan_RiskPriority(t *testing.T) {
	config := DefaultPlannerConfig()
	planner := NewPlanner(config)

	model := &SystemModel{
		ID:         "model-1",
		Repository: "test-repo",
		Functions: []Function{
			{ID: "fn1", Name: "LowRisk", Exported: true},
			{ID: "fn2", Name: "HighRisk", Exported: true},
			{ID: "fn3", Name: "MediumRisk", Exported: true},
		},
		RiskScores: map[string]RiskScore{
			"fn1": {FunctionID: "fn1", Score: 0.2}, // Low
			"fn2": {FunctionID: "fn2", Score: 0.8}, // High
			"fn3": {FunctionID: "fn3", Score: 0.5}, // Medium
		},
	}

	plan, err := planner.Plan(model)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	// Check priorities are set correctly
	priorities := make(map[string]string)
	for _, intent := range plan.Intents {
		if intent.Level == LevelUnit {
			priorities[intent.TargetID] = intent.Priority
		}
	}

	if priorities["fn2"] != "high" {
		t.Errorf("fn2 priority = %s, want high", priorities["fn2"])
	}
	if priorities["fn3"] != "medium" {
		t.Errorf("fn3 priority = %s, want medium", priorities["fn3"])
	}
	if priorities["fn1"] != "low" {
		t.Errorf("fn1 priority = %s, want low", priorities["fn1"])
	}
}

func TestPlanner_Plan_MaxIntents(t *testing.T) {
	config := DefaultPlannerConfig()
	config.MaxIntents = 2
	planner := NewPlanner(config)

	model := &SystemModel{
		ID:         "model-1",
		Repository: "test-repo",
		Functions: []Function{
			{ID: "fn1", Name: "Func1", Exported: true},
			{ID: "fn2", Name: "Func2", Exported: true},
			{ID: "fn3", Name: "Func3", Exported: true},
			{ID: "fn4", Name: "Func4", Exported: true},
		},
		RiskScores: map[string]RiskScore{
			"fn1": {FunctionID: "fn1", Score: 0.9},
			"fn2": {FunctionID: "fn2", Score: 0.8},
			"fn3": {FunctionID: "fn3", Score: 0.7},
			"fn4": {FunctionID: "fn4", Score: 0.6},
		},
	}

	plan, err := planner.Plan(model)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	if len(plan.Intents) != 2 {
		t.Errorf("len(Intents) = %d, want 2 (max intents)", len(plan.Intents))
	}
	if plan.TotalTests != 2 {
		t.Errorf("TotalTests = %d, want 2", plan.TotalTests)
	}
}

func TestPlanner_Plan_EmptyModel(t *testing.T) {
	config := DefaultPlannerConfig()
	planner := NewPlanner(config)

	model := &SystemModel{
		ID:         "model-1",
		Repository: "test-repo",
	}

	plan, err := planner.Plan(model)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	if plan.TotalTests != 0 {
		t.Errorf("TotalTests = %d, want 0", plan.TotalTests)
	}
	if len(plan.Intents) != 0 {
		t.Errorf("len(Intents) = %d, want 0", len(plan.Intents))
	}
}

func TestPlanner_Plan_MethodReason(t *testing.T) {
	config := DefaultPlannerConfig()
	planner := NewPlanner(config)

	model := &SystemModel{
		ID:         "model-1",
		Repository: "test-repo",
		Functions: []Function{
			{ID: "fn1", Name: "GetUser", Class: "UserService", Exported: true},
		},
		RiskScores: map[string]RiskScore{
			"fn1": {FunctionID: "fn1", Score: 0.5},
		},
	}

	plan, err := planner.Plan(model)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	if len(plan.Intents) != 1 {
		t.Fatalf("len(Intents) = %d, want 1", len(plan.Intents))
	}

	// Reason should mention method format
	intent := plan.Intents[0]
	if intent.Reason == "" {
		t.Error("Reason should not be empty")
	}
}

// =============================================================================
// PlanWithPyramid Tests
// =============================================================================

func TestPlanner_PlanWithPyramid(t *testing.T) {
	config := DefaultPlannerConfig()
	planner := NewPlanner(config)

	model := &SystemModel{
		ID:         "model-1",
		Repository: "test-repo",
		Endpoints: []Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users"},
			{ID: "ep2", Method: "POST", Path: "/users"},
			{ID: "ep3", Method: "DELETE", Path: "/users"},
		},
		Functions: []Function{
			{ID: "fn1", Name: "Func1", Exported: true},
			{ID: "fn2", Name: "Func2", Exported: true},
			{ID: "fn3", Name: "Func3", Exported: true},
			{ID: "fn4", Name: "Func4", Exported: true},
			{ID: "fn5", Name: "Func5", Exported: true},
		},
		RiskScores: map[string]RiskScore{
			"fn1": {FunctionID: "fn1", Score: 0.5},
			"fn2": {FunctionID: "fn2", Score: 0.5},
			"fn3": {FunctionID: "fn3", Score: 0.5},
			"fn4": {FunctionID: "fn4", Score: 0.5},
			"fn5": {FunctionID: "fn5", Score: 0.5},
		},
	}

	// Request 10 total tests
	plan, err := planner.PlanWithPyramid(model, 10)
	if err != nil {
		t.Fatalf("PlanWithPyramid() error: %v", err)
	}

	// With default ratios (70% unit, 20% API, 10% E2E)
	// For 10 tests: 7 unit, 2 API, 1 E2E (but E2E not implemented)
	if plan.APITests > 2 {
		t.Errorf("APITests = %d, want <= 2", plan.APITests)
	}

	// Total should not exceed requested
	if plan.TotalTests > 10 {
		t.Errorf("TotalTests = %d, should not exceed 10", plan.TotalTests)
	}
}

func TestPlanner_PlanWithPyramid_LimitedEndpoints(t *testing.T) {
	config := DefaultPlannerConfig()
	planner := NewPlanner(config)

	model := &SystemModel{
		ID:         "model-1",
		Repository: "test-repo",
		Endpoints: []Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users"},
		},
		Functions: []Function{
			{ID: "fn1", Name: "Func1", Exported: true},
			{ID: "fn2", Name: "Func2", Exported: true},
		},
		RiskScores: map[string]RiskScore{
			"fn1": {FunctionID: "fn1", Score: 0.5},
			"fn2": {FunctionID: "fn2", Score: 0.5},
		},
	}

	// Request 10 tests but only 1 endpoint available
	plan, err := planner.PlanWithPyramid(model, 10)
	if err != nil {
		t.Fatalf("PlanWithPyramid() error: %v", err)
	}

	// Should only have 1 API test (limited by available endpoints)
	if plan.APITests != 1 {
		t.Errorf("APITests = %d, want 1", plan.APITests)
	}
}

func TestPlanner_PlanWithPyramid_SkipsHandlers(t *testing.T) {
	config := DefaultPlannerConfig()
	planner := NewPlanner(config)

	model := &SystemModel{
		ID:         "model-1",
		Repository: "test-repo",
		Endpoints: []Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users", Handler: "GetUsersHandler"},
		},
		Functions: []Function{
			{ID: "fn1", Name: "GetUsersHandler", Exported: true}, // Handler
			{ID: "fn2", Name: "GetUser", Exported: true},
		},
		RiskScores: map[string]RiskScore{
			"fn1": {FunctionID: "fn1", Score: 0.5},
			"fn2": {FunctionID: "fn2", Score: 0.5},
		},
	}

	plan, err := planner.PlanWithPyramid(model, 10)
	if err != nil {
		t.Fatalf("PlanWithPyramid() error: %v", err)
	}

	// Handler should be skipped
	if plan.UnitTests > 1 {
		t.Errorf("UnitTests = %d, handler should be skipped", plan.UnitTests)
	}
}

func TestPlanner_PlanWithPyramid_EmptyModel(t *testing.T) {
	config := DefaultPlannerConfig()
	planner := NewPlanner(config)

	model := &SystemModel{
		ID:         "model-1",
		Repository: "test-repo",
	}

	plan, err := planner.PlanWithPyramid(model, 10)
	if err != nil {
		t.Fatalf("PlanWithPyramid() error: %v", err)
	}

	if plan.TotalTests != 0 {
		t.Errorf("TotalTests = %d, want 0", plan.TotalTests)
	}
}

// Note: TestIntent_Fields and TestLevel_Constants are defined in intent_test.go
