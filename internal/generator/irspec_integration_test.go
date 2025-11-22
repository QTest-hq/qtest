package generator

import (
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/internal/adapters"
	"github.com/QTest-hq/qtest/pkg/model"
)

// TestIRSpecPipeline_FullFlow tests the complete IRSpec pipeline:
// JSON Input -> Parse -> Validate -> Convert -> Generate Code
func TestIRSpecPipeline_FullFlow(t *testing.T) {
	// Sample IRSpec JSON (like what an LLM would produce)
	irspecJSON := `{
		"function_name": "Add",
		"description": "Tests for the Add function",
		"tests": [
			{
				"name": "add_positive_numbers",
				"description": "Adding two positive numbers returns their sum",
				"given": [
					{"name": "a", "value": 5, "type": "int"},
					{"name": "b", "value": 3, "type": "int"}
				],
				"when": {
					"call": "Add($a, $b)",
					"args": ["a", "b"]
				},
				"then": [
					{"type": "equals", "actual": "result", "expected": 8}
				],
				"tags": ["happy_path"]
			},
			{
				"name": "add_negative_numbers",
				"description": "Adding negative numbers works correctly",
				"given": [
					{"name": "a", "value": -2, "type": "int"},
					{"name": "b", "value": -3, "type": "int"}
				],
				"when": {
					"call": "Add($a, $b)",
					"args": ["a", "b"]
				},
				"then": [
					{"type": "equals", "actual": "result", "expected": -5}
				],
				"tags": ["edge_case"]
			}
		]
	}`

	// Step 1: Parse and Validate
	converter := NewIRSpecConverter()
	suite, validationResult, err := converter.ParseAndValidate(irspecJSON)
	if err != nil {
		t.Fatalf("ParseAndValidate failed: %v", err)
	}

	// Verify validation passed
	if !validationResult.Valid {
		t.Errorf("validation failed: %v", validationResult.ErrorMessages())
	}

	// Check no warnings
	if len(validationResult.Warnings) > 0 {
		t.Logf("warnings: %v", validationResult.WarningMessages())
	}

	// Step 2: Verify parsed structure
	if suite.FunctionName != "Add" {
		t.Errorf("expected function_name='Add', got %q", suite.FunctionName)
	}
	if len(suite.Tests) != 2 {
		t.Fatalf("expected 2 tests, got %d", len(suite.Tests))
	}

	// Step 3: Convert to TestSpecs
	specs, err := converter.ConvertToTestSpecs(suite)
	if err != nil {
		t.Fatalf("ConvertToTestSpecs failed: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}

	// Verify specs have correct data
	for _, spec := range specs {
		if spec.FunctionName != "Add" {
			t.Errorf("expected FunctionName='Add', got %q", spec.FunctionName)
		}
		if len(spec.Inputs) != 2 {
			t.Errorf("expected 2 inputs, got %d", len(spec.Inputs))
		}
		if len(spec.ArgOrder) != 2 {
			t.Errorf("expected ArgOrder with 2 elements, got %d", len(spec.ArgOrder))
		}
		if len(spec.Assertions) != 1 {
			t.Errorf("expected 1 assertion, got %d", len(spec.Assertions))
		}
		// Verify type hints are preserved
		if spec.InputTypes["a"] != "int" {
			t.Errorf("expected InputTypes['a']='int', got %q", spec.InputTypes["a"])
		}
	}

	// Step 4: Generate Go test code
	goAdapter := adapters.NewGoSpecAdapter()
	goCode, err := goAdapter.GenerateFromSpecs(specs, "math.go")
	if err != nil {
		t.Fatalf("Go GenerateFromSpecs failed: %v", err)
	}

	// Verify Go code structure
	if !strings.Contains(goCode, "func TestAdd(t *testing.T)") {
		t.Error("Go code missing TestAdd function")
	}
	if !strings.Contains(goCode, "t.Run(") {
		t.Error("Go code missing t.Run subtests")
	}
	if !strings.Contains(goCode, "a := 5") {
		t.Error("Go code missing variable assignment")
	}
	if !strings.Contains(goCode, "result := Add(a, b)") {
		t.Error("Go code missing function call")
	}

	// Step 5: Generate pytest code
	pytestAdapter := adapters.NewPytestSpecAdapter()
	pytestCode, err := pytestAdapter.GenerateFromSpecs(specs, "math.py")
	if err != nil {
		t.Fatalf("pytest GenerateFromSpecs failed: %v", err)
	}

	// Verify pytest code structure
	if !strings.Contains(pytestCode, "class TestAdd:") {
		t.Error("pytest code missing TestAdd class")
	}
	if !strings.Contains(pytestCode, "def test_") {
		t.Error("pytest code missing test methods")
	}
	if !strings.Contains(pytestCode, "assert result ==") {
		t.Error("pytest code missing assertions")
	}

	// Step 6: Generate Jest code
	jestAdapter := adapters.NewJestSpecAdapter()
	jestCode, err := jestAdapter.GenerateFromSpecs(specs, "math.ts")
	if err != nil {
		t.Fatalf("Jest GenerateFromSpecs failed: %v", err)
	}

	// Verify Jest code structure
	if !strings.Contains(jestCode, "describe('Add'") {
		t.Error("Jest code missing describe block")
	}
	if !strings.Contains(jestCode, "test(") {
		t.Error("Jest code missing test blocks")
	}
	if !strings.Contains(jestCode, "expect(result).toBe(") {
		t.Error("Jest code missing expect assertions")
	}
}

// TestIRSpecPipeline_MarkdownCleanup tests that markdown code blocks are cleaned up
func TestIRSpecPipeline_MarkdownCleanup(t *testing.T) {
	// LLMs often wrap JSON in markdown code blocks
	markdownJSON := "```json\n" + `{
		"function_name": "Multiply",
		"tests": [
			{
				"name": "multiply_basics",
				"description": "Basic multiplication",
				"given": [{"name": "x", "value": 4, "type": "int"}],
				"when": {"call": "Multiply($x, 2)", "args": ["x"]},
				"then": [{"type": "equals", "actual": "result", "expected": 8}]
			}
		]
	}` + "\n```"

	converter := NewIRSpecConverter()
	specs, err := converter.ParseAndConvert(markdownJSON)
	if err != nil {
		t.Fatalf("ParseAndConvert with markdown failed: %v", err)
	}

	if len(specs) != 1 {
		t.Errorf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].FunctionName != "Multiply" {
		t.Errorf("expected FunctionName='Multiply', got %q", specs[0].FunctionName)
	}
}

// TestIRSpecPipeline_ValidationErrors tests validation catches errors
func TestIRSpecPipeline_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		expectError bool
	}{
		{
			name:        "missing function_name",
			json:        `{"tests": [{"name": "test1", "given": [], "when": {"call": "F()"}, "then": [{"type": "truthy", "actual": "result"}]}]}`,
			expectError: true,
		},
		{
			name:        "empty tests array",
			json:        `{"function_name": "F", "tests": []}`,
			expectError: true,
		},
		{
			name:        "invalid assertion type",
			json:        `{"function_name": "F", "tests": [{"name": "test1", "given": [], "when": {"call": "F()"}, "then": [{"type": "invalid_type", "actual": "result"}]}]}`,
			expectError: true,
		},
		{
			name:        "undefined variable in args",
			json:        `{"function_name": "F", "tests": [{"name": "test1", "given": [{"name": "a", "value": 1, "type": "int"}], "when": {"call": "F($a, $b)", "args": ["a", "undefined_var"]}, "then": [{"type": "truthy", "actual": "result"}]}]}`,
			expectError: true,
		},
	}

	converter := NewIRSpecConverter()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := converter.ParseAndConvert(tc.json)
			if tc.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestIRSpecPipeline_AllAssertionTypes tests all supported assertion types
func TestIRSpecPipeline_AllAssertionTypes(t *testing.T) {
	json := `{
		"function_name": "Test",
		"tests": [
			{
				"name": "all_assertions",
				"given": [{"name": "x", "value": 10, "type": "int"}],
				"when": {"call": "Test($x)", "args": ["x"]},
				"then": [
					{"type": "equals", "actual": "result", "expected": 10},
					{"type": "not_equals", "actual": "result", "expected": 0},
					{"type": "greater_than", "actual": "result", "expected": 5},
					{"type": "less_than", "actual": "result", "expected": 20},
					{"type": "truthy", "actual": "result"},
					{"type": "not_nil", "actual": "result"}
				]
			}
		]
	}`

	converter := NewIRSpecConverter()
	specs, err := converter.ParseAndConvert(json)
	if err != nil {
		t.Fatalf("ParseAndConvert failed: %v", err)
	}

	// Verify all assertions were converted
	if len(specs[0].Assertions) != 6 {
		t.Errorf("expected 6 assertions, got %d", len(specs[0].Assertions))
	}

	// Generate code for all frameworks
	goAdapter := adapters.NewGoSpecAdapter()
	goCode, err := goAdapter.GenerateFromSpecs(specs, "test.go")
	if err != nil {
		t.Fatalf("Go generation failed: %v", err)
	}

	// Check assertion types in generated code
	assertionChecks := []string{
		"result !=",   // equals
		"result ==",   // not_equals (inverted)
		"result <=",   // greater_than (inverted)
		"result >=",   // less_than (inverted)
		"!result",     // truthy (inverted)
		"result == nil", // not_nil (inverted)
	}
	for _, check := range assertionChecks {
		if !strings.Contains(goCode, check) {
			t.Errorf("Go code missing assertion check: %s", check)
		}
	}
}

// TestIRSpecPipeline_TypePreservation tests that type hints are preserved
func TestIRSpecPipeline_TypePreservation(t *testing.T) {
	json := `{
		"function_name": "Process",
		"tests": [
			{
				"name": "type_test",
				"given": [
					{"name": "intVal", "value": 42, "type": "int"},
					{"name": "floatVal", "value": 3.14, "type": "float"},
					{"name": "strVal", "value": "hello", "type": "string"},
					{"name": "boolVal", "value": true, "type": "bool"}
				],
				"when": {"call": "Process($intVal, $floatVal, $strVal, $boolVal)", "args": ["intVal", "floatVal", "strVal", "boolVal"]},
				"then": [{"type": "truthy", "actual": "result"}]
			}
		]
	}`

	converter := NewIRSpecConverter()
	specs, err := converter.ParseAndConvert(json)
	if err != nil {
		t.Fatalf("ParseAndConvert failed: %v", err)
	}

	spec := specs[0]

	// Verify type hints are preserved
	expectedTypes := map[string]string{
		"intVal":   "int",
		"floatVal": "float",
		"strVal":   "string",
		"boolVal":  "bool",
	}
	for name, expectedType := range expectedTypes {
		if spec.InputTypes[name] != expectedType {
			t.Errorf("InputTypes[%s] = %q, want %q", name, spec.InputTypes[name], expectedType)
		}
	}

	// Verify arg order is preserved
	expectedOrder := []string{"intVal", "floatVal", "strVal", "boolVal"}
	for i, expected := range expectedOrder {
		if spec.ArgOrder[i] != expected {
			t.Errorf("ArgOrder[%d] = %q, want %q", i, spec.ArgOrder[i], expected)
		}
	}
}

// TestIRSpecConverter_DirectValidate tests direct validation method
func TestIRSpecConverter_DirectValidate(t *testing.T) {
	converter := NewIRSpecConverter()

	// Valid suite
	validSuite := &model.IRTestSuite{
		FunctionName: "Add",
		Tests: []model.IRTestCase{
			{
				Name:  "test1",
				Given: []model.IRVariable{{Name: "x", Value: 1, Type: "int"}},
				When:  model.IRAction{Call: "Add($x)", Args: []string{"x"}},
				Then:  []model.IRAssertion{{Type: "equals", Actual: "result", Expected: 1}},
			},
		},
	}

	result := converter.Validate(validSuite)
	if !result.Valid {
		t.Errorf("expected valid suite, got errors: %v", result.ErrorMessages())
	}
}
