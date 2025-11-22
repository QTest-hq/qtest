package adapters

import (
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
)

func TestPytestSpecAdapter_GenerateFromSpecs(t *testing.T) {
	adapter := NewPytestSpecAdapter()

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

	code, err := adapter.GenerateFromSpecs(specs, "math.py")
	if err != nil {
		t.Fatalf("GenerateFromSpecs failed: %v", err)
	}

	// Verify test structure
	if !strings.Contains(code, "class TestAdd:") {
		t.Error("expected class TestAdd in output")
	}
	if !strings.Contains(code, "def test_adding_two_positive_numbers(self):") {
		t.Error("expected test method in output")
	}
	if !strings.Contains(code, "a = 5") {
		t.Error("expected variable a assignment")
	}
	if !strings.Contains(code, "b = 3") {
		t.Error("expected variable b assignment")
	}
	if !strings.Contains(code, "result = add(a, b)") {
		t.Error("expected function call")
	}
	if !strings.Contains(code, "assert result == 8") {
		t.Error("expected assertion")
	}
}

func TestPytestSpecAdapter_GenerateAssertions(t *testing.T) {
	adapter := NewPytestSpecAdapter()

	tests := []struct {
		name     string
		kind     string
		actual   string
		expected interface{}
		want     string
	}{
		{"equals", "equals", "result", 42, "assert result == 42"},
		{"not_equals", "not_equals", "result", 0, "assert result != 0"},
		{"not_nil", "not_nil", "result", nil, "assert result is not None"},
		{"nil", "nil", "result", nil, "assert result is None"},
		{"contains", "contains", "result", "hello", "assert \"hello\" in result"},
		{"greater_than", "greater_than", "result", 10, "assert result > 10"},
		{"less_than", "less_than", "result", 100, "assert result < 100"},
		{"truthy", "truthy", "result", nil, "assert result"},
		{"falsy", "falsy", "result", nil, "assert not result"},
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

func TestFormatPythonValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{nil, "None"},
		{42, "42"},
		{float64(3.14), "3.14"},
		{true, "True"},
		{false, "False"},
		{"hello", "\"hello\""},
		{[]interface{}{1, 2, 3}, "[1, 2, 3]"},
	}

	for _, tc := range tests {
		result := formatPythonValue(tc.input)
		if result != tc.expected {
			t.Errorf("formatPythonValue(%v) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestToPythonClassName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"add", "Add"},
		{"calculate_sum", "CalculateSum"},
		{"my-function", "MyFunction"},
		{"", "Unknown"},
	}

	for _, tc := range tests {
		result := toPythonClassName(tc.input)
		if result != tc.expected {
			t.Errorf("toPythonClassName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestToPythonTestName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Adding two numbers", "adding_two_numbers"},
		{"Test case with (parentheses)", "test_case_with_parentheses"},
		{"", "test_case"},
	}

	for _, tc := range tests {
		result := toPythonTestName(tc.input)
		if result != tc.expected {
			t.Errorf("toPythonTestName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestPytestSpecAdapter_MultipleTestCases(t *testing.T) {
	adapter := NewPytestSpecAdapter()

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

	code, err := adapter.GenerateFromSpecs(specs, "math.py")
	if err != nil {
		t.Fatalf("GenerateFromSpecs failed: %v", err)
	}

	// Should have both test cases
	if !strings.Contains(code, "def test_divide_positive_numbers") {
		t.Error("expected first test case")
	}
	if !strings.Contains(code, "def test_divide_by_one") {
		t.Error("expected second test case")
	}
}
