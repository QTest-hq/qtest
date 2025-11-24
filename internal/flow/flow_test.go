package flow

import (
	"testing"
	"time"
)

func TestDefaultFlowConfig(t *testing.T) {
	config := DefaultFlowConfig()

	if !config.RecordNetwork {
		t.Error("RecordNetwork should be true by default")
	}
	if config.RecordScreenshots {
		t.Error("RecordScreenshots should be false by default")
	}
	if !config.DetectLoginForms {
		t.Error("DetectLoginForms should be true by default")
	}
	if config.PageLoadTimeout != 30*time.Second {
		t.Errorf("PageLoadTimeout = %v, want 30s", config.PageLoadTimeout)
	}
	if len(config.SelectorPreferences) == 0 {
		t.Error("SelectorPreferences should not be empty")
	}
	if config.SelectorPreferences[0] != SelectorTestID {
		t.Errorf("First selector preference = %v, want %v", config.SelectorPreferences[0], SelectorTestID)
	}
}

func TestFlowTypes(t *testing.T) {
	tests := []struct {
		flowType FlowType
		expected string
	}{
		{FlowTypeLogin, "login"},
		{FlowTypeRegistration, "registration"},
		{FlowTypeCheckout, "checkout"},
		{FlowTypeSearch, "search"},
		{FlowTypeFormSubmit, "form_submit"},
		{FlowTypeNavigation, "navigation"},
		{FlowTypeCRUD, "crud"},
		{FlowTypeCustom, "custom"},
	}

	for _, tt := range tests {
		if string(tt.flowType) != tt.expected {
			t.Errorf("FlowType %v = %q, want %q", tt.flowType, string(tt.flowType), tt.expected)
		}
	}
}

func TestActionTypes(t *testing.T) {
	tests := []struct {
		actionType ActionType
		expected   string
	}{
		{ActionTypeClick, "click"},
		{ActionTypeFill, "fill"},
		{ActionTypeSelect, "select"},
		{ActionTypeCheck, "check"},
		{ActionTypeHover, "hover"},
		{ActionTypeNavigate, "navigate"},
		{ActionTypeWait, "wait"},
		{ActionTypeAssert, "assert"},
	}

	for _, tt := range tests {
		if string(tt.actionType) != tt.expected {
			t.Errorf("ActionType %v = %q, want %q", tt.actionType, string(tt.actionType), tt.expected)
		}
	}
}

func TestSelectorStrategies(t *testing.T) {
	tests := []struct {
		strategy SelectorStrategy
		expected string
	}{
		{SelectorTestID, "test-id"},
		{SelectorDataCy, "data-cy"},
		{SelectorID, "id"},
		{SelectorCSS, "css"},
		{SelectorXPath, "xpath"},
		{SelectorText, "text"},
		{SelectorRole, "role"},
		{SelectorPlaceholder, "placeholder"},
		{SelectorLabel, "label"},
	}

	for _, tt := range tests {
		if string(tt.strategy) != tt.expected {
			t.Errorf("SelectorStrategy %v = %q, want %q", tt.strategy, string(tt.strategy), tt.expected)
		}
	}
}

func TestMapFlowType(t *testing.T) {
	tests := []struct {
		input    string
		expected FlowType
	}{
		{"login", FlowTypeLogin},
		{"Login Flow", FlowTypeLogin},
		{"user registration", FlowTypeRegistration},
		{"signup form", FlowTypeRegistration},
		{"checkout process", FlowTypeCheckout},
		{"payment flow", FlowTypeCheckout},
		{"search functionality", FlowTypeSearch},
		{"form submission", FlowTypeFormSubmit},
		{"page navigation", FlowTypeNavigation},
		{"crud operations", FlowTypeCRUD},
		{"unknown type", FlowTypeCustom},
	}

	for _, tt := range tests {
		result := mapFlowType(tt.input)
		if result != tt.expected {
			t.Errorf("mapFlowType(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestMapActionType(t *testing.T) {
	tests := []struct {
		input    string
		expected ActionType
	}{
		{"click", ActionTypeClick},
		{"Click Button", ActionTypeClick},
		{"fill", ActionTypeFill},
		{"type text", ActionTypeFill},
		{"input value", ActionTypeFill},
		{"select option", ActionTypeSelect},
		{"check box", ActionTypeCheck},
		{"hover over", ActionTypeHover},
		{"scroll down", ActionTypeScroll},
		{"press key", ActionTypeKeypress},
		{"keypress", ActionTypeKeypress},
		{"navigate to", ActionTypeNavigate},
		{"goto page", ActionTypeNavigate},
		{"wait for", ActionTypeWait},
		{"assert that", ActionTypeAssert},
	}

	for _, tt := range tests {
		result := mapActionType(tt.input)
		if result != tt.expected {
			t.Errorf("mapActionType(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s        string
		substrs  []string
		expected bool
	}{
		{"username", []string{"user", "name"}, true},
		{"password", []string{"pass", "pwd"}, true},
		{"email", []string{"mail", "address"}, true},
		{"field", []string{"user", "pass"}, false},
		{"", []string{"test"}, false},
		{"test", []string{}, false},
	}

	for _, tt := range tests {
		result := containsAny(tt.s, tt.substrs)
		if result != tt.expected {
			t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.substrs, result, tt.expected)
		}
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`Here is the JSON: [{"name": "test"}]`, `[{"name": "test"}]`},
		{`Response: {"key": "value"}`, `{"key": "value"}`},
		{`[{"a": 1}, {"b": 2}]`, `[{"a": 1}, {"b": 2}]`},
		{`No JSON here`, ""},
		{`Partial [{"incomplete"`, ""},
		{`Nested: {"outer": {"inner": "value"}}`, `{"outer": {"inner": "value"}}`},
	}

	for _, tt := range tests {
		result := extractJSON(tt.input)
		if result != tt.expected {
			t.Errorf("extractJSON(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"test", 4, "test"},
		{"testing", 4, "test..."},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestValidateHint(t *testing.T) {
	tests := []struct {
		name       string
		hint       FlowHint
		errorCount int
	}{
		{
			name: "valid login hint",
			hint: FlowHint{
				Type: FlowTypeLogin,
				Steps: []HintStep{
					{Action: "fill", Target: "#email", Value: "test@example.com"},
					{Action: "fill", Target: "#password", Value: "password"},
					{Action: "click", Target: "#submit"},
				},
			},
			errorCount: 0,
		},
		{
			name: "missing type",
			hint: FlowHint{
				Steps: []HintStep{
					{Action: "click", Target: "#button"},
				},
			},
			errorCount: 1,
		},
		{
			name: "no steps",
			hint: FlowHint{
				Type:  FlowTypeLogin,
				Steps: []HintStep{},
			},
			errorCount: 1,
		},
		{
			name: "step missing action",
			hint: FlowHint{
				Type: FlowTypeLogin,
				Steps: []HintStep{
					{Target: "#button"},
				},
			},
			errorCount: 1,
		},
		{
			name: "click missing target",
			hint: FlowHint{
				Type: FlowTypeLogin,
				Steps: []HintStep{
					{Action: "click"},
				},
			},
			errorCount: 1,
		},
		{
			name: "navigate missing value",
			hint: FlowHint{
				Type: FlowTypeNavigation,
				Steps: []HintStep{
					{Action: "navigate"},
				},
			},
			errorCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateHint(tt.hint)
			if len(errors) != tt.errorCount {
				t.Errorf("ValidateHint() returned %d errors, want %d: %v", len(errors), tt.errorCount, errors)
			}
		})
	}
}

func TestMergeHints(t *testing.T) {
	hints1 := []FlowHint{
		{Type: FlowTypeLogin, Description: "Login flow"},
		{Type: FlowTypeSearch, Description: "Search flow"},
	}

	hints2 := []FlowHint{
		{Type: FlowTypeLogin, Description: "Login flow"}, // Duplicate
		{Type: FlowTypeCheckout, Description: "Checkout flow"},
	}

	merged := MergeHints(hints1, hints2)

	if len(merged) != 3 {
		t.Errorf("MergeHints() returned %d hints, want 3", len(merged))
	}

	// Verify no duplicates
	seen := make(map[string]bool)
	for _, h := range merged {
		key := string(h.Type) + "-" + h.Description
		if seen[key] {
			t.Errorf("Duplicate hint found: %s", key)
		}
		seen[key] = true
	}
}

func TestGenerateHintTemplate(t *testing.T) {
	hints := GenerateHintTemplate()

	if len(hints) == 0 {
		t.Error("GenerateHintTemplate() returned empty hints")
	}

	// Check that login flow is included
	hasLogin := false
	for _, h := range hints {
		if h.Type == FlowTypeLogin {
			hasLogin = true
			if len(h.Steps) == 0 {
				t.Error("Login hint should have steps")
			}
			if h.Credentials == nil || len(h.Credentials) == 0 {
				t.Error("Login hint should have credentials")
			}
		}
	}

	if !hasLogin {
		t.Error("Template should include login flow")
	}
}

func TestExportHintsToYAML(t *testing.T) {
	hints := []FlowHint{
		{
			Type:        FlowTypeLogin,
			Description: "Test login",
			Steps: []HintStep{
				{Action: "click", Target: "#button"},
			},
		},
	}

	yaml, err := ExportHintsToYAML(hints)
	if err != nil {
		t.Fatalf("ExportHintsToYAML() error: %v", err)
	}

	if yaml == "" {
		t.Error("ExportHintsToYAML() returned empty string")
	}

	// Verify it contains expected content
	if !containsAny(yaml, []string{"login", "Test login", "click"}) {
		t.Error("YAML output missing expected content")
	}
}

func TestExportHintsToJSON(t *testing.T) {
	hints := []FlowHint{
		{
			Type:        FlowTypeSearch,
			Description: "Test search",
			Steps: []HintStep{
				{Action: "fill", Target: "#search", Value: "query"},
			},
		},
	}

	json, err := ExportHintsToJSON(hints)
	if err != nil {
		t.Fatalf("ExportHintsToJSON() error: %v", err)
	}

	if json == "" {
		t.Error("ExportHintsToJSON() returned empty string")
	}

	// Verify it's valid JSON structure
	if json[0] != '[' {
		t.Error("JSON should start with array bracket")
	}
}

func TestHintLoaderLoadHintsFromString(t *testing.T) {
	loader := NewHintLoader("")

	yamlContent := `
- type: login
  description: Test login flow
  steps:
    - action: fill
      target: "#email"
      value: "test@example.com"
    - action: click
      target: "#submit"
`

	hints, err := loader.LoadHintsFromString(yamlContent)
	if err != nil {
		t.Fatalf("LoadHintsFromString() error: %v", err)
	}

	if len(hints) != 1 {
		t.Errorf("LoadHintsFromString() returned %d hints, want 1", len(hints))
	}

	if hints[0].Type != FlowTypeLogin {
		t.Errorf("First hint type = %v, want %v", hints[0].Type, FlowTypeLogin)
	}

	if len(hints[0].Steps) != 2 {
		t.Errorf("First hint has %d steps, want 2", len(hints[0].Steps))
	}
}

func TestHintLoaderLoadHintsFromStringJSON(t *testing.T) {
	loader := NewHintLoader("")

	jsonContent := `[
		{
			"type": "search",
			"description": "Search flow",
			"steps": [
				{"action": "fill", "target": "#query", "value": "test"}
			]
		}
	]`

	hints, err := loader.LoadHintsFromString(jsonContent)
	if err != nil {
		t.Fatalf("LoadHintsFromString() error: %v", err)
	}

	if len(hints) != 1 {
		t.Errorf("LoadHintsFromString() returned %d hints, want 1", len(hints))
	}

	if hints[0].Type != FlowTypeSearch {
		t.Errorf("First hint type = %v, want %v", hints[0].Type, FlowTypeSearch)
	}
}

func TestFlowStepOrdering(t *testing.T) {
	flow := Flow{
		ID:   "test-flow",
		Name: "Test Flow",
		Type: FlowTypeLogin,
		Steps: []Step{
			{ID: "1", Order: 1, Action: Action{Type: ActionTypeFill}},
			{ID: "2", Order: 2, Action: Action{Type: ActionTypeFill}},
			{ID: "3", Order: 3, Action: Action{Type: ActionTypeClick}},
		},
	}

	if len(flow.Steps) != 3 {
		t.Errorf("Flow has %d steps, want 3", len(flow.Steps))
	}

	// Verify order
	for i, step := range flow.Steps {
		if step.Order != i+1 {
			t.Errorf("Step %d has order %d, want %d", i, step.Order, i+1)
		}
	}
}

func TestSelectorWithFallbacks(t *testing.T) {
	selector := Selector{
		Primary:    "[data-testid='submit']",
		Strategy:   SelectorTestID,
		Confidence: 0.9,
		Fallbacks: []Selector{
			{Primary: "#submit-btn", Strategy: SelectorID, Confidence: 0.7},
			{Primary: "button[type='submit']", Strategy: SelectorCSS, Confidence: 0.5},
		},
	}

	if len(selector.Fallbacks) != 2 {
		t.Errorf("Selector has %d fallbacks, want 2", len(selector.Fallbacks))
	}

	if selector.Confidence <= selector.Fallbacks[0].Confidence {
		t.Error("Primary selector should have higher confidence than fallbacks")
	}
}

func TestFormFieldTypes(t *testing.T) {
	form := Form{
		ID:       "login-form",
		FormType: "login",
		Fields: []FormField{
			{Name: "email", Type: "email", Required: true},
			{Name: "password", Type: "password", Required: true},
			{Name: "remember", Type: "checkbox", Required: false},
		},
	}

	if len(form.Fields) != 3 {
		t.Errorf("Form has %d fields, want 3", len(form.Fields))
	}

	// Verify required fields
	requiredCount := 0
	for _, field := range form.Fields {
		if field.Required {
			requiredCount++
		}
	}
	if requiredCount != 2 {
		t.Errorf("Form has %d required fields, want 2", requiredCount)
	}
}
