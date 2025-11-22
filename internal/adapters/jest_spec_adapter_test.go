package adapters

import (
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
)

func TestJestSpecAdapter_GenerateFromSpecs(t *testing.T) {
	adapter := NewJestSpecAdapter()

	specs := []model.TestSpec{
		{
			FunctionName: "add",
			Description:  "Adding two positive numbers",
			Inputs: map[string]interface{}{
				"a": float64(5),
				"b": float64(3),
			},
			InputTypes: map[string]string{
				"a": "int",
				"b": "int",
			},
			ArgOrder: []string{"a", "b"},
			Assertions: []model.Assertion{
				{Kind: "equals", Actual: "result", Expected: float64(8)},
			},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "math.ts")
	if err != nil {
		t.Fatalf("GenerateFromSpecs failed: %v", err)
	}

	// Verify test structure
	if !strings.Contains(code, "describe('add'") {
		t.Error("expected describe block in output")
	}
	if !strings.Contains(code, "test('Adding two positive numbers'") {
		t.Error("expected test block in output")
	}
	if !strings.Contains(code, "const a = 5;") {
		t.Error("expected variable a assignment")
	}
	if !strings.Contains(code, "const b = 3;") {
		t.Error("expected variable b assignment")
	}
	if !strings.Contains(code, "const result = add(a, b);") {
		t.Error("expected function call")
	}
	if !strings.Contains(code, "expect(result).toBe(8);") {
		t.Error("expected assertion")
	}
}

func TestJestSpecAdapter_GenerateAssertions(t *testing.T) {
	adapter := NewJestSpecAdapter()

	tests := []struct {
		name     string
		kind     string
		actual   string
		expected interface{}
		want     string
	}{
		{"equals", "equals", "result", 42, "expect(result).toBe(42);"},
		{"not_equals", "not_equals", "result", 0, "expect(result).not.toBe(0);"},
		{"not_nil", "not_nil", "result", nil, "expect(result).not.toBeNull();"},
		{"nil", "nil", "result", nil, "expect(result).toBeNull();"},
		{"contains", "contains", "result", "hello", "expect(result).toContain('hello');"},
		{"greater_than", "greater_than", "result", 10, "expect(result).toBeGreaterThan(10);"},
		{"less_than", "less_than", "result", 100, "expect(result).toBeLessThan(100);"},
		{"truthy", "truthy", "result", nil, "expect(result).toBeTruthy();"},
		{"falsy", "falsy", "result", nil, "expect(result).toBeFalsy();"},
		{"length", "length", "result", 5, "expect(result).toHaveLength(5);"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertion := model.Assertion{
				Kind:     tc.kind,
				Actual:   tc.actual,
				Expected: tc.expected,
			}
			got := adapter.generateAssertion(assertion)
			if got != tc.want {
				t.Errorf("generateAssertion(%s) = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestFormatJSValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{nil, "null"},
		{42, "42"},
		{float64(3.14), "3.14"},
		{true, "true"},
		{false, "false"},
		{"hello", "'hello'"},
		{[]interface{}{1, 2, 3}, "[1, 2, 3]"},
	}

	for _, tc := range tests {
		result := formatJSValue(tc.input)
		if result != tc.expected {
			t.Errorf("formatJSValue(%v) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestExtractJSModuleName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"math.ts", "./math"},
		{"utils/helper.js", "./helper"},          // Only extracts filename
		{"./src/utils.tsx", "./utils"},           // Only extracts filename
		{"/full/path/to/app.ts", "./app"},        // Full path extracts filename
		{"/home/user/project/src/module.js", "./module"},
		{"", ""},
	}

	for _, tc := range tests {
		result := extractJSModuleName(tc.input)
		if result != tc.expected {
			t.Errorf("extractJSModuleName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestJestSpecAdapter_MultipleTestCases(t *testing.T) {
	adapter := NewJestSpecAdapter()

	specs := []model.TestSpec{
		{
			FunctionName: "divide",
			Description:  "Divide positive numbers",
			Inputs: map[string]interface{}{
				"a": float64(10),
				"b": float64(2),
			},
			ArgOrder: []string{"a", "b"},
			Assertions: []model.Assertion{
				{Kind: "equals", Actual: "result", Expected: float64(5)},
			},
		},
		{
			FunctionName: "divide",
			Description:  "Divide by one",
			Inputs: map[string]interface{}{
				"a": float64(7),
				"b": float64(1),
			},
			ArgOrder: []string{"a", "b"},
			Assertions: []model.Assertion{
				{Kind: "equals", Actual: "result", Expected: float64(7)},
			},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "math.ts")
	if err != nil {
		t.Fatalf("GenerateFromSpecs failed: %v", err)
	}

	// Should have both test cases
	if !strings.Contains(code, "test('Divide positive numbers'") {
		t.Error("expected first test case")
	}
	if !strings.Contains(code, "test('Divide by one'") {
		t.Error("expected second test case")
	}
}

func TestJestSpecAdapter_StringValues(t *testing.T) {
	adapter := NewJestSpecAdapter()

	specs := []model.TestSpec{
		{
			FunctionName: "greet",
			Description:  "Greeting with name",
			Inputs: map[string]interface{}{
				"name": "Alice",
			},
			InputTypes: map[string]string{
				"name": "string",
			},
			ArgOrder: []string{"name"},
			Assertions: []model.Assertion{
				{Kind: "equals", Actual: "result", Expected: "Hello, Alice!"},
			},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "greeting.ts")
	if err != nil {
		t.Fatalf("GenerateFromSpecs failed: %v", err)
	}

	// Check string values are properly quoted
	if !strings.Contains(code, "const name = 'Alice';") {
		t.Error("expected string value to be quoted")
	}
	if !strings.Contains(code, "expect(result).toBe('Hello, Alice!');") {
		t.Error("expected assertion with quoted string")
	}
}
