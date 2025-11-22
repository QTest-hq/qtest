package generator

import (
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
)

func TestIRSpecValidator_ValidSuite(t *testing.T) {
	validator := NewIRSpecValidator()

	suite := &model.IRTestSuite{
		FunctionName: "Add",
		Description:  "Tests for Add function",
		Tests: []model.IRTestCase{
			{
				Name:        "add_positive_numbers",
				Description: "Adding two positive numbers",
				Given: []model.IRVariable{
					{Name: "a", Value: float64(5), Type: "int"},
					{Name: "b", Value: float64(3), Type: "int"},
				},
				When: model.IRAction{
					Call: "Add($a, $b)",
					Args: []string{"a", "b"},
				},
				Then: []model.IRAssertion{
					{Type: "equals", Actual: "result", Expected: float64(8)},
				},
				Tags: []string{"happy_path"},
			},
		},
	}

	result := validator.Validate(suite)

	if !result.Valid {
		t.Errorf("expected valid suite, got errors: %v", result.ErrorMessages())
	}
	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got: %v", result.ErrorMessages())
	}
}

func TestIRSpecValidator_NilSuite(t *testing.T) {
	validator := NewIRSpecValidator()
	result := validator.Validate(nil)

	if result.Valid {
		t.Error("expected invalid result for nil suite")
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
}

func TestIRSpecValidator_MissingFunctionName(t *testing.T) {
	validator := NewIRSpecValidator()

	suite := &model.IRTestSuite{
		FunctionName: "",
		Tests: []model.IRTestCase{
			{
				Name: "test1",
				Given: []model.IRVariable{
					{Name: "x", Value: 1, Type: "int"},
				},
				When: model.IRAction{Call: "Func($x)", Args: []string{"x"}},
				Then: []model.IRAssertion{
					{Type: "equals", Actual: "result", Expected: 1},
				},
			},
		},
	}

	result := validator.Validate(suite)

	if result.Valid {
		t.Error("expected invalid result for missing function name")
	}

	foundError := false
	for _, err := range result.Errors {
		if err.Field == "function_name" {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected error about function_name")
	}
}

func TestIRSpecValidator_EmptyTests(t *testing.T) {
	validator := NewIRSpecValidator()

	suite := &model.IRTestSuite{
		FunctionName: "MyFunc",
		Tests:        []model.IRTestCase{},
	}

	result := validator.Validate(suite)

	if result.Valid {
		t.Error("expected invalid result for empty tests")
	}

	foundError := false
	for _, err := range result.Errors {
		if err.Field == "tests" {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected error about tests array")
	}
}

func TestIRSpecValidator_InvalidAssertionType(t *testing.T) {
	validator := NewIRSpecValidator()

	suite := &model.IRTestSuite{
		FunctionName: "MyFunc",
		Tests: []model.IRTestCase{
			{
				Name: "test1",
				Given: []model.IRVariable{
					{Name: "x", Value: 1, Type: "int"},
				},
				When: model.IRAction{Call: "MyFunc($x)", Args: []string{"x"}},
				Then: []model.IRAssertion{
					{Type: "invalid_assertion_type", Actual: "result", Expected: 1},
				},
			},
		},
	}

	result := validator.Validate(suite)

	if result.Valid {
		t.Error("expected invalid result for bad assertion type")
	}

	foundError := false
	for _, err := range result.Errors {
		if err.Value == "invalid_assertion_type" {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected error about invalid assertion type")
	}
}

func TestIRSpecValidator_InvalidTypeHint(t *testing.T) {
	validator := NewIRSpecValidator()

	suite := &model.IRTestSuite{
		FunctionName: "MyFunc",
		Tests: []model.IRTestCase{
			{
				Name: "test1",
				Given: []model.IRVariable{
					{Name: "x", Value: 1, Type: "invalid_type"},
				},
				When: model.IRAction{Call: "MyFunc($x)", Args: []string{"x"}},
				Then: []model.IRAssertion{
					{Type: "equals", Actual: "result", Expected: 1},
				},
			},
		},
	}

	result := validator.Validate(suite)

	if result.Valid {
		t.Error("expected invalid result for bad type hint")
	}

	foundError := false
	for _, err := range result.Errors {
		if err.Value == "invalid_type" {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected error about invalid type hint")
	}
}

func TestIRSpecValidator_UndefinedVariableInArgs(t *testing.T) {
	validator := NewIRSpecValidator()

	suite := &model.IRTestSuite{
		FunctionName: "MyFunc",
		Tests: []model.IRTestCase{
			{
				Name: "test1",
				Given: []model.IRVariable{
					{Name: "a", Value: 1, Type: "int"},
				},
				When: model.IRAction{
					Call: "MyFunc($a, $b)",
					Args: []string{"a", "undefined_var"}, // undefined_var not in given
				},
				Then: []model.IRAssertion{
					{Type: "equals", Actual: "result", Expected: 1},
				},
			},
		},
	}

	result := validator.Validate(suite)

	if result.Valid {
		t.Error("expected invalid result for undefined variable")
	}

	foundError := false
	for _, err := range result.Errors {
		if err.Value == "undefined_var" {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected error about undefined variable")
	}
}

func TestIRSpecValidator_DuplicateVariableNames(t *testing.T) {
	validator := NewIRSpecValidator()

	suite := &model.IRTestSuite{
		FunctionName: "MyFunc",
		Tests: []model.IRTestCase{
			{
				Name: "test1",
				Given: []model.IRVariable{
					{Name: "x", Value: 1, Type: "int"},
					{Name: "x", Value: 2, Type: "int"}, // duplicate
				},
				When: model.IRAction{Call: "MyFunc($x)", Args: []string{"x"}},
				Then: []model.IRAssertion{
					{Type: "equals", Actual: "result", Expected: 1},
				},
			},
		},
	}

	result := validator.Validate(suite)

	if result.Valid {
		t.Error("expected invalid result for duplicate variable")
	}
}

func TestIRSpecValidator_MissingExpectedForEquals(t *testing.T) {
	validator := NewIRSpecValidator()

	suite := &model.IRTestSuite{
		FunctionName: "MyFunc",
		Tests: []model.IRTestCase{
			{
				Name: "test1",
				Given: []model.IRVariable{
					{Name: "x", Value: 1, Type: "int"},
				},
				When: model.IRAction{Call: "MyFunc($x)", Args: []string{"x"}},
				Then: []model.IRAssertion{
					{Type: "equals", Actual: "result"}, // missing Expected
				},
			},
		},
	}

	result := validator.Validate(suite)

	if result.Valid {
		t.Error("expected invalid result for missing expected value")
	}
}

func TestIRSpecValidator_TruthyDoesNotRequireExpected(t *testing.T) {
	validator := NewIRSpecValidator()

	suite := &model.IRTestSuite{
		FunctionName: "IsValid",
		Tests: []model.IRTestCase{
			{
				Name: "test1",
				Given: []model.IRVariable{
					{Name: "x", Value: "test", Type: "string"},
				},
				When: model.IRAction{Call: "IsValid($x)", Args: []string{"x"}},
				Then: []model.IRAssertion{
					{Type: "truthy", Actual: "result"}, // no expected needed
				},
			},
		},
	}

	result := validator.Validate(suite)

	if !result.Valid {
		t.Errorf("truthy assertion should not require expected value, got errors: %v", result.ErrorMessages())
	}
}

func TestIRSpecValidator_TypeMismatchWarning(t *testing.T) {
	validator := NewIRSpecValidator()

	suite := &model.IRTestSuite{
		FunctionName: "MyFunc",
		Tests: []model.IRTestCase{
			{
				Name: "test1",
				Given: []model.IRVariable{
					{Name: "x", Value: "not_an_int", Type: "int"}, // type mismatch
				},
				When: model.IRAction{Call: "MyFunc($x)", Args: []string{"x"}},
				Then: []model.IRAssertion{
					{Type: "equals", Actual: "result", Expected: 1},
				},
			},
		},
	}

	result := validator.Validate(suite)

	// Should be valid (warning, not error) but have warnings
	if len(result.Warnings) == 0 {
		t.Error("expected warning about type mismatch")
	}
}

func TestIRSpecValidator_Summary(t *testing.T) {
	validator := NewIRSpecValidator()

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

	result := validator.Validate(validSuite)
	if result.Summary() != "validation passed" {
		t.Errorf("expected 'validation passed', got: %s", result.Summary())
	}
}

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"validName", true},
		{"_valid", true},
		{"valid123", true},
		{"123invalid", false},
		{"invalid-name", false},
		{"invalid.name", false},
		{"", false},
		{"a", true},
		{"_", true},
	}

	for _, tc := range tests {
		result := isValidIdentifier(tc.input)
		if result != tc.expected {
			t.Errorf("isValidIdentifier(%q) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}
