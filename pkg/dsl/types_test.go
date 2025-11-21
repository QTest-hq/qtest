package dsl

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

// =============================================================================
// TestDSL Tests
// =============================================================================

func TestTestDSL_Fields(t *testing.T) {
	dsl := TestDSL{
		Version:     "1.0",
		ID:          "test-001",
		Name:        "Test GetUser",
		Description: "Tests the GetUser function",
		Type:        TestTypeUnit,
		Target: TestTarget{
			File:     "user.go",
			Function: "GetUser",
		},
		Steps: []TestStep{
			{ID: "step-1", Description: "Call function"},
		},
		Metadata: map[string]string{
			"author": "test",
		},
	}

	if dsl.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", dsl.Version)
	}
	if dsl.Type != TestTypeUnit {
		t.Errorf("Type = %s, want unit", dsl.Type)
	}
	if len(dsl.Steps) != 1 {
		t.Errorf("len(Steps) = %d, want 1", len(dsl.Steps))
	}
}

func TestTestDSL_JSON(t *testing.T) {
	dsl := TestDSL{
		Version: "1.0",
		ID:      "test-001",
		Name:    "Test JSON",
		Type:    TestTypeAPI,
		Target:  TestTarget{File: "api.go"},
		Steps:   []TestStep{},
	}

	data, err := json.Marshal(dsl)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed TestDSL
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Version != dsl.Version {
		t.Errorf("Version mismatch after JSON roundtrip")
	}
	if parsed.Type != dsl.Type {
		t.Errorf("Type mismatch after JSON roundtrip")
	}
}

func TestTestDSL_YAML(t *testing.T) {
	dsl := TestDSL{
		Version: "1.0",
		ID:      "test-001",
		Name:    "Test YAML",
		Type:    TestTypeIntegration,
		Target:  TestTarget{File: "service.go"},
		Steps:   []TestStep{},
	}

	data, err := yaml.Marshal(dsl)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed TestDSL
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Version != dsl.Version {
		t.Errorf("Version mismatch after YAML roundtrip")
	}
}

// =============================================================================
// TestType Tests
// =============================================================================

func TestTestType_Constants(t *testing.T) {
	tests := []struct {
		typ  TestType
		want string
	}{
		{TestTypeUnit, "unit"},
		{TestTypeIntegration, "integration"},
		{TestTypeAPI, "api"},
		{TestTypeE2E, "e2e"},
	}

	for _, tt := range tests {
		if string(tt.typ) != tt.want {
			t.Errorf("TestType %v = %s, want %s", tt.typ, string(tt.typ), tt.want)
		}
	}
}

// =============================================================================
// TestTarget Tests
// =============================================================================

func TestTestTarget_Fields(t *testing.T) {
	target := TestTarget{
		File:     "user_controller.go",
		Function: "GetUser",
		Class:    "UserController",
		Method:   "GET",
		Endpoint: "/users/:id",
		Tags:     []string{"user", "api"},
	}

	if target.File != "user_controller.go" {
		t.Errorf("File = %s, want user_controller.go", target.File)
	}
	if len(target.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(target.Tags))
	}
}

// =============================================================================
// Lifecycle Tests
// =============================================================================

func TestLifecycle_Fields(t *testing.T) {
	lifecycle := Lifecycle{
		Scope: ScopeSuite,
		BeforeAll: []Action{
			{Type: "setup_db", Params: map[string]interface{}{"name": "test_db"}},
		},
		BeforeEach: []Action{
			{Type: "begin_transaction"},
		},
		AfterEach: []Action{
			{Type: "rollback_transaction"},
		},
		AfterAll: []Action{
			{Type: "teardown_db"},
		},
	}

	if lifecycle.Scope != ScopeSuite {
		t.Errorf("Scope = %s, want suite", lifecycle.Scope)
	}
	if len(lifecycle.BeforeAll) != 1 {
		t.Errorf("len(BeforeAll) = %d, want 1", len(lifecycle.BeforeAll))
	}
}

func TestLifecycleScope_Constants(t *testing.T) {
	tests := []struct {
		scope LifecycleScope
		want  string
	}{
		{ScopeTest, "test"},
		{ScopeSuite, "suite"},
		{ScopeFile, "file"},
	}

	for _, tt := range tests {
		if string(tt.scope) != tt.want {
			t.Errorf("LifecycleScope %v = %s, want %s", tt.scope, string(tt.scope), tt.want)
		}
	}
}

// =============================================================================
// Action Tests
// =============================================================================

func TestAction_Fields(t *testing.T) {
	action := Action{
		Type: "http_request",
		Params: map[string]interface{}{
			"method": "POST",
			"url":    "/api/users",
			"body":   map[string]string{"name": "John"},
		},
	}

	if action.Type != "http_request" {
		t.Errorf("Type = %s, want http_request", action.Type)
	}
	if len(action.Params) != 3 {
		t.Errorf("len(Params) = %d, want 3", len(action.Params))
	}
}

// =============================================================================
// Resource Tests
// =============================================================================

func TestResource_Fields(t *testing.T) {
	resource := Resource{
		Type: ResourceDatabase,
		Name: "test_db",
		Config: map[string]interface{}{
			"driver": "postgres",
			"host":   "localhost",
		},
	}

	if resource.Type != ResourceDatabase {
		t.Errorf("Type = %s, want database", resource.Type)
	}
	if resource.Name != "test_db" {
		t.Errorf("Name = %s, want test_db", resource.Name)
	}
}

func TestResourceType_Constants(t *testing.T) {
	tests := []struct {
		typ  ResourceType
		want string
	}{
		{ResourceDatabase, "database"},
		{ResourceCache, "cache"},
		{ResourceQueue, "queue"},
		{ResourceService, "service"},
		{ResourceFile, "file"},
	}

	for _, tt := range tests {
		if string(tt.typ) != tt.want {
			t.Errorf("ResourceType %v = %s, want %s", tt.typ, string(tt.typ), tt.want)
		}
	}
}

// =============================================================================
// Isolation Tests
// =============================================================================

func TestIsolation_Fields(t *testing.T) {
	isolation := Isolation{
		Level:    IsolationTransaction,
		Parallel: false,
		Timeout:  "30s",
	}

	if isolation.Level != IsolationTransaction {
		t.Errorf("Level = %s, want transaction", isolation.Level)
	}
	if isolation.Timeout != "30s" {
		t.Errorf("Timeout = %s, want 30s", isolation.Timeout)
	}
}

func TestIsolationLevel_Constants(t *testing.T) {
	tests := []struct {
		level IsolationLevel
		want  string
	}{
		{IsolationNone, "none"},
		{IsolationTransaction, "transaction"},
		{IsolationContainer, "container"},
		{IsolationProcess, "process"},
	}

	for _, tt := range tests {
		if string(tt.level) != tt.want {
			t.Errorf("IsolationLevel %v = %s, want %s", tt.level, string(tt.level), tt.want)
		}
	}
}

// =============================================================================
// TestStep Tests
// =============================================================================

func TestTestStep_Fields(t *testing.T) {
	step := TestStep{
		ID:          "step-001",
		Description: "Call GetUser",
		Action: StepAction{
			Type:   ActionCall,
			Target: "UserService",
			Method: "GetUser",
			Args:   []interface{}{1},
		},
		Input: map[string]interface{}{
			"userId": 1,
		},
		Expected: &Expected{
			Value: map[string]interface{}{
				"id":   1,
				"name": "John",
			},
		},
		Store: map[string]string{
			"user": "result",
		},
	}

	if step.ID != "step-001" {
		t.Errorf("ID = %s, want step-001", step.ID)
	}
	if step.Action.Type != ActionCall {
		t.Errorf("Action.Type = %s, want call", step.Action.Type)
	}
	if step.Expected == nil {
		t.Error("Expected should not be nil")
	}
}

// =============================================================================
// StepAction Tests
// =============================================================================

func TestStepAction_Fields(t *testing.T) {
	tests := []struct {
		name   string
		action StepAction
	}{
		{
			name: "call",
			action: StepAction{
				Type:   ActionCall,
				Target: "service",
				Method: "DoSomething",
				Args:   []interface{}{"arg1", 2},
			},
		},
		{
			name: "http",
			action: StepAction{
				Type:   ActionHTTP,
				Method: "POST",
				Params: map[string]interface{}{
					"url":  "/api/users",
					"body": `{"name": "John"}`,
				},
			},
		},
		{
			name: "navigate",
			action: StepAction{
				Type:   ActionNavigate,
				Params: map[string]interface{}{"url": "http://localhost:3000"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.action.Type == "" {
				t.Error("Type should not be empty")
			}
		})
	}
}

func TestActionType_Constants(t *testing.T) {
	tests := []struct {
		typ  ActionType
		want string
	}{
		{ActionCall, "call"},
		{ActionHTTP, "http"},
		{ActionAssert, "assert"},
		{ActionSetup, "setup"},
		{ActionTeardown, "teardown"},
		{ActionNavigate, "navigate"},
		{ActionClick, "click"},
		{ActionType_, "type"},
		{ActionWait, "wait"},
		{ActionScreenshot, "screenshot"},
	}

	for _, tt := range tests {
		if string(tt.typ) != tt.want {
			t.Errorf("ActionType %v = %s, want %s", tt.typ, string(tt.typ), tt.want)
		}
	}
}

// =============================================================================
// Expected Tests
// =============================================================================

func TestExpected_Fields(t *testing.T) {
	expected := Expected{
		Value:    42,
		Type:     "int",
		Contains: "substring",
		Matches:  "^[0-9]+$",
		Properties: map[string]interface{}{
			"name": "John",
			"age":  30,
		},
		Error: &ExpectedError{
			Type:    "NotFoundError",
			Message: "user not found",
		},
	}

	if expected.Value != 42 {
		t.Errorf("Value = %v, want 42", expected.Value)
	}
	if expected.Error == nil {
		t.Error("Error should not be nil")
	}
	if expected.Error.Type != "NotFoundError" {
		t.Errorf("Error.Type = %s, want NotFoundError", expected.Error.Type)
	}
}

// =============================================================================
// ExpectedError Tests
// =============================================================================

func TestExpectedError_Fields(t *testing.T) {
	err := ExpectedError{
		Type:    "ValidationError",
		Message: "invalid email format",
	}

	if err.Type != "ValidationError" {
		t.Errorf("Type = %s, want ValidationError", err.Type)
	}
	if err.Message != "invalid email format" {
		t.Errorf("Message = %s, want 'invalid email format'", err.Message)
	}
}

// =============================================================================
// Complex DSL Test
// =============================================================================

func TestTestDSL_CompleteExample(t *testing.T) {
	dsl := TestDSL{
		Version:     "1.0",
		ID:          "e2e-user-flow",
		Name:        "User Registration Flow",
		Description: "Tests the complete user registration flow",
		Type:        TestTypeE2E,
		Target: TestTarget{
			File:     "app.e2e.ts",
			Endpoint: "/register",
			Tags:     []string{"e2e", "registration"},
		},
		Lifecycle: &Lifecycle{
			Scope:      ScopeTest,
			BeforeEach: []Action{{Type: "clear_session"}},
			AfterEach:  []Action{{Type: "take_screenshot"}},
		},
		Resources: []Resource{
			{Type: ResourceService, Name: "api", Config: map[string]interface{}{"url": "http://localhost:3000"}},
		},
		Isolation: &Isolation{
			Level:    IsolationNone,
			Parallel: false,
			Timeout:  "60s",
		},
		Steps: []TestStep{
			{
				ID:          "navigate",
				Description: "Navigate to registration page",
				Action:      StepAction{Type: ActionNavigate, Params: map[string]interface{}{"url": "/register"}},
			},
			{
				ID:          "fill-form",
				Description: "Fill registration form",
				Action:      StepAction{Type: ActionType_, Target: "[name=email]", Params: map[string]interface{}{"text": "test@example.com"}},
			},
			{
				ID:          "submit",
				Description: "Submit form",
				Action:      StepAction{Type: ActionClick, Target: "button[type=submit]"},
			},
			{
				ID:          "verify",
				Description: "Verify success message",
				Action:      StepAction{Type: ActionAssert},
				Expected:    &Expected{Contains: "Registration successful"},
			},
		},
		Metadata: map[string]string{
			"suite":    "e2e",
			"priority": "high",
		},
	}

	if dsl.Type != TestTypeE2E {
		t.Errorf("Type = %s, want e2e", dsl.Type)
	}
	if len(dsl.Steps) != 4 {
		t.Errorf("len(Steps) = %d, want 4", len(dsl.Steps))
	}
	if dsl.Lifecycle == nil {
		t.Error("Lifecycle should not be nil")
	}
	if len(dsl.Resources) != 1 {
		t.Errorf("len(Resources) = %d, want 1", len(dsl.Resources))
	}
	if dsl.Isolation.Timeout != "60s" {
		t.Errorf("Isolation.Timeout = %s, want 60s", dsl.Isolation.Timeout)
	}
}
