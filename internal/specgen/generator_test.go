package specgen

import (
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/pkg/model"
)

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	if gen == nil {
		t.Fatal("NewGenerator() returned nil")
	}
	if gen.router != nil {
		t.Error("router should be nil")
	}
	if gen.tier != llm.Tier1 {
		t.Errorf("tier = %v, want Tier1", gen.tier)
	}
}

func TestNewGenerator_Tiers(t *testing.T) {
	tests := []struct {
		tier llm.Tier
	}{
		{llm.Tier1},
		{llm.Tier2},
		{llm.Tier3},
	}

	for _, tt := range tests {
		gen := NewGenerator(nil, tt.tier)
		if gen.tier != tt.tier {
			t.Errorf("tier = %v, want %v", gen.tier, tt.tier)
		}
	}
}

func TestBuildModelFragment_Endpoint(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	sysModel := &model.SystemModel{
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users", Handler: "ListUsers"},
		},
		Functions: []model.Function{
			{ID: "fn1", Name: "ListUsers"},
		},
		Types: []model.TypeDef{
			{Name: "User"},
		},
	}

	intent := model.TestIntent{
		TargetKind: "endpoint",
		TargetID:   "ep1",
	}

	fragment := gen.buildModelFragment(intent, sysModel)

	if fragment["endpoint"] == nil {
		t.Error("Should include endpoint")
	}
	if fragment["handler"] == nil {
		t.Error("Should include handler function")
	}
}

func TestBuildModelFragment_EndpointWithTypes(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	sysModel := &model.SystemModel{
		Endpoints: []model.Endpoint{
			{
				ID:           "ep1",
				Method:       "POST",
				Path:         "/users",
				Handler:      "CreateUser",
				RequestBody:  "CreateUserRequest",
				ResponseBody: "User",
			},
		},
		Types: []model.TypeDef{
			{Name: "CreateUserRequest"},
			{Name: "User"},
		},
	}

	intent := model.TestIntent{
		TargetKind: "endpoint",
		TargetID:   "ep1",
	}

	fragment := gen.buildModelFragment(intent, sysModel)

	if fragment["endpoint"] == nil {
		t.Error("Should include endpoint")
	}
	if fragment["request_type"] == nil {
		t.Error("Should include request type")
	}
	if fragment["response_type"] == nil {
		t.Error("Should include response type")
	}
}

func TestBuildModelFragment_Function(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	sysModel := &model.SystemModel{
		Functions: []model.Function{
			{
				ID:   "fn1",
				Name: "Calculate",
				Parameters: []model.Parameter{
					{Name: "input", Type: "CalculateInput"},
				},
			},
		},
		Types: []model.TypeDef{
			{Name: "CalculateInput"},
		},
	}

	intent := model.TestIntent{
		TargetKind: "function",
		TargetID:   "fn1",
	}

	fragment := gen.buildModelFragment(intent, sysModel)

	if fragment["function"] == nil {
		t.Error("Should include function")
	}
	if fragment["types"] == nil {
		t.Error("Should include related types")
	}
}

func TestBuildModelFragment_NotFound(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	sysModel := &model.SystemModel{
		Endpoints: []model.Endpoint{},
		Functions: []model.Function{},
	}

	intent := model.TestIntent{
		TargetKind: "endpoint",
		TargetID:   "nonexistent",
	}

	fragment := gen.buildModelFragment(intent, sysModel)

	if len(fragment) != 0 {
		t.Error("Fragment should be empty for non-existent target")
	}
}

func TestBuildPrompt_API(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	intent := model.TestIntent{
		ID:         "test-1",
		Level:      model.LevelAPI,
		TargetKind: "endpoint",
		TargetID:   "ep1",
	}

	fragment := map[string]interface{}{
		"endpoint": model.Endpoint{ID: "ep1", Method: "GET", Path: "/users"},
	}

	prompt := gen.buildPrompt(intent, fragment)

	if !strings.Contains(prompt, "System Model Fragment") {
		t.Error("Prompt should contain system model fragment")
	}
	if !strings.Contains(prompt, "Test Intent") {
		t.Error("Prompt should contain test intent")
	}
	if !strings.Contains(prompt, "API Test Guidelines") {
		t.Error("Should include API test guidance for API level")
	}
}

func TestBuildPrompt_Unit(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	intent := model.TestIntent{
		ID:         "test-1",
		Level:      model.LevelUnit,
		TargetKind: "function",
		TargetID:   "fn1",
	}

	fragment := map[string]interface{}{
		"function": model.Function{ID: "fn1", Name: "Calculate"},
	}

	prompt := gen.buildPrompt(intent, fragment)

	if !strings.Contains(prompt, "Unit Test Guidelines") {
		t.Error("Should include Unit test guidance for unit level")
	}
}

func TestParseSpecResponse_Valid(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	response := `{
		"id": "spec-1",
		"level": "unit",
		"target_kind": "function",
		"target_id": "fn1",
		"description": "Test function",
		"function_name": "Calculate",
		"inputs": {"a": 1, "b": 2},
		"expected": {"result": 3},
		"assertions": [{"kind": "equality", "actual": "result", "expected": 3}]
	}`

	intent := model.TestIntent{
		ID:         "test-1",
		Level:      model.LevelUnit,
		TargetKind: "function",
		TargetID:   "fn1",
	}

	spec, err := gen.parseSpecResponse(response, intent)
	if err != nil {
		t.Fatalf("parseSpecResponse() error = %v", err)
	}

	if spec.ID != "spec-1" {
		t.Errorf("ID = %s, want spec-1", spec.ID)
	}
	if spec.Level != "unit" {
		t.Errorf("Level = %s, want unit", spec.Level)
	}
}

func TestParseSpecResponse_Markdown(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	response := "```json\n{\"id\": \"spec-1\", \"level\": \"unit\"}\n```"

	intent := model.TestIntent{ID: "test-1", Level: model.LevelUnit}

	spec, err := gen.parseSpecResponse(response, intent)
	if err != nil {
		t.Fatalf("parseSpecResponse() error = %v", err)
	}

	if spec.ID != "spec-1" {
		t.Errorf("ID = %s, want spec-1", spec.ID)
	}
}

func TestParseSpecResponse_MarkdownNoLang(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	response := "```\n{\"id\": \"spec-2\"}\n```"

	intent := model.TestIntent{ID: "test-1"}

	spec, err := gen.parseSpecResponse(response, intent)
	if err != nil {
		t.Fatalf("parseSpecResponse() error = %v", err)
	}

	if spec.ID != "spec-2" {
		t.Errorf("ID = %s, want spec-2", spec.ID)
	}
}

func TestParseSpecResponse_FillsDefaults(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	response := `{"description": "test"}`

	intent := model.TestIntent{
		ID:         "intent-1",
		Level:      model.LevelAPI,
		TargetKind: "endpoint",
		TargetID:   "ep1",
		Priority:   "high",
	}

	spec, err := gen.parseSpecResponse(response, intent)
	if err != nil {
		t.Fatalf("parseSpecResponse() error = %v", err)
	}

	// Should fill in from intent
	if spec.ID != "intent-1" {
		t.Errorf("ID = %s, want intent-1", spec.ID)
	}
	if spec.Level != model.LevelAPI {
		t.Errorf("Level = %s, want api", spec.Level)
	}
	if spec.TargetKind != "endpoint" {
		t.Errorf("TargetKind = %s, want endpoint", spec.TargetKind)
	}
	if spec.TargetID != "ep1" {
		t.Errorf("TargetID = %s, want ep1", spec.TargetID)
	}
	if spec.Priority != "high" {
		t.Errorf("Priority = %s, want high", spec.Priority)
	}
}

func TestParseSpecResponse_Invalid(t *testing.T) {
	gen := NewGenerator(nil, llm.Tier1)

	response := "not valid json"
	intent := model.TestIntent{}

	_, err := gen.parseSpecResponse(response, intent)
	if err == nil {
		t.Error("Should return error for invalid JSON")
	}
}

func TestSystemPromptExists(t *testing.T) {
	if systemPromptSpecGen == "" {
		t.Error("systemPromptSpecGen should not be empty")
	}
	if !strings.Contains(systemPromptSpecGen, "JSON") {
		t.Error("System prompt should mention JSON")
	}
}

func TestAPITestGuidanceExists(t *testing.T) {
	if apiTestGuidance == "" {
		t.Error("apiTestGuidance should not be empty")
	}
	if !strings.Contains(apiTestGuidance, "API") {
		t.Error("API guidance should mention API")
	}
}

func TestUnitTestGuidanceExists(t *testing.T) {
	if unitTestGuidance == "" {
		t.Error("unitTestGuidance should not be empty")
	}
	if !strings.Contains(unitTestGuidance, "Unit") {
		t.Error("Unit guidance should mention Unit")
	}
}
