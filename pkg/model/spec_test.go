package model

import (
	"testing"
)

// =============================================================================
// TestSpec Tests
// =============================================================================

func TestTestSpec_Fields(t *testing.T) {
	spec := TestSpec{
		ID:           "test-001",
		Level:        LevelUnit,
		TargetKind:   "function",
		TargetID:     "fn:GetUser",
		Description:  "Test GetUser returns user by ID",
		FunctionName: "GetUser",
		Inputs: map[string]interface{}{
			"id": 1,
		},
		Assertions: []Assertion{
			{Kind: "equality", Actual: "result.ID", Expected: 1},
		},
		Tags:     []string{"unit", "user"},
		Priority: "high",
	}

	if spec.ID != "test-001" {
		t.Errorf("ID = %s, want test-001", spec.ID)
	}
	if spec.Level != LevelUnit {
		t.Errorf("Level = %s, want unit", spec.Level)
	}
	if spec.FunctionName != "GetUser" {
		t.Errorf("FunctionName = %s, want GetUser", spec.FunctionName)
	}
	if len(spec.Assertions) != 1 {
		t.Errorf("len(Assertions) = %d, want 1", len(spec.Assertions))
	}
}

func TestTestSpec_APIFields(t *testing.T) {
	spec := TestSpec{
		ID:          "api-test-001",
		Level:       LevelAPI,
		TargetKind:  "endpoint",
		TargetID:    "ep:GET:/users/:id",
		Description: "Test GET /users/:id returns user",
		Method:      "GET",
		Path:        "/users/:id",
		PathParams: map[string]interface{}{
			"id": 1,
		},
		QueryParams: map[string]interface{}{
			"include": "orders",
		},
		Headers: map[string]string{
			"Authorization": "Bearer token",
		},
		Expected: map[string]interface{}{
			"status": 200,
		},
		Assertions: []Assertion{
			{Kind: "status_code", Expected: 200},
			{Kind: "contains", Actual: "body.name", Expected: "John"},
		},
	}

	if spec.Method != "GET" {
		t.Errorf("Method = %s, want GET", spec.Method)
	}
	if spec.Path != "/users/:id" {
		t.Errorf("Path = %s, want /users/:id", spec.Path)
	}
	if len(spec.PathParams) != 1 {
		t.Errorf("len(PathParams) = %d, want 1", len(spec.PathParams))
	}
	if len(spec.Headers) != 1 {
		t.Errorf("len(Headers) = %d, want 1", len(spec.Headers))
	}
}

func TestTestSpec_WithBody(t *testing.T) {
	spec := TestSpec{
		ID:         "api-test-002",
		Level:      LevelAPI,
		Method:     "POST",
		Path:       "/users",
		Body: map[string]interface{}{
			"name":  "John",
			"email": "john@example.com",
		},
	}

	body, ok := spec.Body.(map[string]interface{})
	if !ok {
		t.Fatal("Body should be a map")
	}
	if body["name"] != "John" {
		t.Errorf("Body.name = %v, want John", body["name"])
	}
}

// =============================================================================
// Assertion Tests
// =============================================================================

func TestAssertion_Fields(t *testing.T) {
	tests := []struct {
		name      string
		assertion Assertion
	}{
		{
			name: "equality",
			assertion: Assertion{
				Kind:     "equality",
				Actual:   "result",
				Expected: 42,
			},
		},
		{
			name: "contains",
			assertion: Assertion{
				Kind:     "contains",
				Actual:   "body.items",
				Expected: "item1",
			},
		},
		{
			name: "not_null",
			assertion: Assertion{
				Kind:   "not_null",
				Actual: "result.user",
			},
		},
		{
			name: "status_code",
			assertion: Assertion{
				Kind:     "status_code",
				Expected: 200,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.assertion.Kind != tt.name {
				t.Errorf("Kind = %s, want %s", tt.assertion.Kind, tt.name)
			}
		})
	}
}

// =============================================================================
// TestSpecSet Tests
// =============================================================================

func TestTestSpecSet_Stats(t *testing.T) {
	specSet := &TestSpecSet{
		ModelID:    "model-123",
		Repository: "test/repo",
		Language:   "go",
		Framework:  "go-test",
		Specs: []TestSpec{
			{ID: "1", Level: LevelUnit},
			{ID: "2", Level: LevelUnit},
			{ID: "3", Level: LevelAPI},
			{ID: "4", Level: LevelE2E},
		},
	}

	stats := specSet.Stats()

	tests := []struct {
		key  string
		want int
	}{
		{"total", 4},
		{"unit", 2},
		{"api", 1},
		{"e2e", 1},
	}

	for _, tt := range tests {
		got := stats[tt.key]
		if got != tt.want {
			t.Errorf("Stats()[%s] = %d, want %d", tt.key, got, tt.want)
		}
	}
}

func TestTestSpecSet_FilterByLevel(t *testing.T) {
	specSet := &TestSpecSet{
		Specs: []TestSpec{
			{ID: "1", Level: LevelUnit},
			{ID: "2", Level: LevelUnit},
			{ID: "3", Level: LevelAPI},
			{ID: "4", Level: LevelE2E},
			{ID: "5", Level: LevelUnit},
		},
	}

	tests := []struct {
		level TestLevel
		want  int
	}{
		{LevelUnit, 3},
		{LevelAPI, 1},
		{LevelE2E, 1},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			filtered := specSet.FilterByLevel(tt.level)
			if len(filtered) != tt.want {
				t.Errorf("len(FilterByLevel(%s)) = %d, want %d", tt.level, len(filtered), tt.want)
			}
			for _, spec := range filtered {
				if spec.Level != tt.level {
					t.Errorf("spec.Level = %s, want %s", spec.Level, tt.level)
				}
			}
		})
	}
}

func TestTestSpecSet_GetByID(t *testing.T) {
	specSet := &TestSpecSet{
		Specs: []TestSpec{
			{ID: "spec-1", Description: "First"},
			{ID: "spec-2", Description: "Second"},
			{ID: "spec-3", Description: "Third"},
		},
	}

	t.Run("found", func(t *testing.T) {
		spec := specSet.GetByID("spec-2")
		if spec == nil {
			t.Fatal("expected to find spec-2")
		}
		if spec.Description != "Second" {
			t.Errorf("Description = %s, want Second", spec.Description)
		}
	})

	t.Run("not found", func(t *testing.T) {
		spec := specSet.GetByID("nonexistent")
		if spec != nil {
			t.Errorf("expected nil, got %v", spec)
		}
	})
}

func TestTestSpecSet_Empty(t *testing.T) {
	specSet := &TestSpecSet{}

	stats := specSet.Stats()
	for key, val := range stats {
		if val != 0 {
			t.Errorf("empty specSet Stats()[%s] = %d, want 0", key, val)
		}
	}

	filtered := specSet.FilterByLevel(LevelUnit)
	if len(filtered) != 0 {
		t.Error("FilterByLevel on empty specSet should return empty slice")
	}

	spec := specSet.GetByID("any")
	if spec != nil {
		t.Error("GetByID on empty specSet should return nil")
	}
}

// =============================================================================
// TestLevel Tests
// =============================================================================

func TestTestLevel_Constants(t *testing.T) {
	tests := []struct {
		level TestLevel
		want  string
	}{
		{LevelUnit, "unit"},
		{LevelAPI, "api"},
		{LevelE2E, "e2e"},
	}

	for _, tt := range tests {
		if string(tt.level) != tt.want {
			t.Errorf("TestLevel %v = %s, want %s", tt.level, string(tt.level), tt.want)
		}
	}
}
