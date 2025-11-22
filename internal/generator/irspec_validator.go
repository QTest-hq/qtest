package generator

import (
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
)

// IRSpecValidator validates IRSpec output from LLMs
type IRSpecValidator struct {
	// ValidTypes are the allowed type hints
	ValidTypes map[string]bool
	// ValidAssertions are the allowed assertion types
	ValidAssertions map[string]bool
}

// ValidationError represents a validation failure with context
type ValidationError struct {
	Field   string // JSON path to the problematic field
	Message string // Human-readable error description
	Value   string // The problematic value (if applicable)
}

func (e ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("%s: %s (got: %s)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains all validation errors and warnings
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
	Valid    bool
}

// NewIRSpecValidator creates a validator with default rules
func NewIRSpecValidator() *IRSpecValidator {
	return &IRSpecValidator{
		ValidTypes: map[string]bool{
			"int":      true,
			"float":    true,
			"string":   true,
			"bool":     true,
			"null":     true,
			"array":    true,
			"object":   true,
			"function": true, // for callbacks/mocks
		},
		ValidAssertions: map[string]bool{
			"equals":       true,
			"not_equals":   true,
			"contains":     true,
			"not_contains": true,
			"greater_than": true,
			"less_than":    true,
			"throws":       true,
			"truthy":       true,
			"falsy":        true,
			"nil":          true,
			"not_nil":      true,
			"length":       true,
			"type_is":      true,
		},
	}
}

// Validate performs full validation on an IRTestSuite
func (v *IRSpecValidator) Validate(suite *model.IRTestSuite) *ValidationResult {
	result := &ValidationResult{
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationError, 0),
		Valid:    true,
	}

	// Nil check
	if suite == nil {
		result.addError("", "IRTestSuite is nil")
		return result
	}

	// Validate function name
	v.validateFunctionName(suite, result)

	// Validate tests array
	v.validateTests(suite, result)

	return result
}

// validateFunctionName checks the function_name field
func (v *IRSpecValidator) validateFunctionName(suite *model.IRTestSuite, result *ValidationResult) {
	if suite.FunctionName == "" {
		result.addError("function_name", "function_name is required")
		return
	}

	// Check for invalid characters
	if strings.ContainsAny(suite.FunctionName, " \t\n\r") {
		result.addError("function_name", "function_name contains whitespace", suite.FunctionName)
	}

	// Check for reasonable length
	if len(suite.FunctionName) > 200 {
		result.addWarning("function_name", "function_name is unusually long")
	}
}

// validateTests validates the tests array
func (v *IRSpecValidator) validateTests(suite *model.IRTestSuite, result *ValidationResult) {
	if len(suite.Tests) == 0 {
		result.addError("tests", "at least one test case is required")
		return
	}

	// Track test names for duplicates
	seenNames := make(map[string]bool)

	for i, tc := range suite.Tests {
		prefix := fmt.Sprintf("tests[%d]", i)
		v.validateTestCase(&tc, prefix, result, seenNames)
	}
}

// validateTestCase validates a single test case
func (v *IRSpecValidator) validateTestCase(tc *model.IRTestCase, prefix string, result *ValidationResult, seenNames map[string]bool) {
	// Validate name
	if tc.Name == "" {
		result.addError(prefix+".name", "test name is required")
	} else {
		if seenNames[tc.Name] {
			result.addWarning(prefix+".name", "duplicate test name", tc.Name)
		}
		seenNames[tc.Name] = true
	}

	// Validate Given (setup variables)
	v.validateGiven(tc.Given, prefix, result)

	// Validate When (action)
	v.validateWhen(&tc.When, prefix, result, tc.Given)

	// Validate Then (assertions)
	v.validateThen(tc.Then, prefix, result)
}

// validateGiven validates the given (setup) section
func (v *IRSpecValidator) validateGiven(given []model.IRVariable, prefix string, result *ValidationResult) {
	seenVars := make(map[string]bool)

	for i, variable := range given {
		varPrefix := fmt.Sprintf("%s.given[%d]", prefix, i)

		// Name is required
		if variable.Name == "" {
			result.addError(varPrefix+".name", "variable name is required")
			continue
		}

		// Check for duplicate variable names
		if seenVars[variable.Name] {
			result.addError(varPrefix+".name", "duplicate variable name", variable.Name)
		}
		seenVars[variable.Name] = true

		// Validate variable name format (should be valid identifier)
		if !isValidIdentifier(variable.Name) {
			result.addWarning(varPrefix+".name", "variable name may not be a valid identifier", variable.Name)
		}

		// Validate type hint
		if variable.Type == "" {
			result.addWarning(varPrefix+".type", "type hint is recommended")
		} else if !v.ValidTypes[variable.Type] {
			result.addError(varPrefix+".type", "invalid type hint", variable.Type)
		}

		// Validate value matches type hint
		if variable.Type != "" {
			v.validateValueType(variable.Value, variable.Type, varPrefix, result)
		}
	}
}

// validateWhen validates the when (action) section
func (v *IRSpecValidator) validateWhen(when *model.IRAction, prefix string, result *ValidationResult, given []model.IRVariable) {
	whenPrefix := prefix + ".when"

	// Call is required
	if when.Call == "" {
		result.addError(whenPrefix+".call", "function call is required")
	}

	// Build map of available variables from Given
	availableVars := make(map[string]bool)
	for _, v := range given {
		availableVars[v.Name] = true
	}

	// Validate args reference existing variables
	for i, arg := range when.Args {
		if !availableVars[arg] {
			result.addError(
				fmt.Sprintf("%s.args[%d]", whenPrefix, i),
				"references undefined variable",
				arg,
			)
		}
	}

	// Check if call pattern uses variables that aren't in args
	// This is a semantic check - $varName in call should match args
	v.validateCallPattern(when.Call, when.Args, availableVars, whenPrefix, result)
}

// validateCallPattern checks the call string for variable references
func (v *IRSpecValidator) validateCallPattern(call string, args []string, availableVars map[string]bool, prefix string, result *ValidationResult) {
	// Look for $variable patterns in the call
	// Pattern: $identifier or ${identifier}
	idx := 0
	for idx < len(call) {
		if call[idx] == '$' {
			// Extract variable name
			varName := ""
			if idx+1 < len(call) && call[idx+1] == '{' {
				// ${var} format
				endIdx := strings.Index(call[idx:], "}")
				if endIdx > 0 {
					varName = call[idx+2 : idx+endIdx]
				}
			} else {
				// $var format - read until non-identifier char
				start := idx + 1
				end := start
				for end < len(call) && isIdentifierChar(rune(call[end])) {
					end++
				}
				if end > start {
					varName = call[start:end]
				}
			}

			if varName != "" && !availableVars[varName] {
				result.addWarning(
					prefix+".call",
					fmt.Sprintf("references variable $%s not in given section", varName),
				)
			}
		}
		idx++
	}
}

// validateThen validates the then (assertions) section
func (v *IRSpecValidator) validateThen(then []model.IRAssertion, prefix string, result *ValidationResult) {
	if len(then) == 0 {
		result.addError(prefix+".then", "at least one assertion is required")
		return
	}

	for i, assertion := range then {
		assertPrefix := fmt.Sprintf("%s.then[%d]", prefix, i)

		// Type is required
		if assertion.Type == "" {
			result.addError(assertPrefix+".type", "assertion type is required")
		} else if !v.ValidAssertions[assertion.Type] {
			result.addError(assertPrefix+".type", "invalid assertion type", assertion.Type)
		}

		// Actual is required
		if assertion.Actual == "" {
			result.addError(assertPrefix+".actual", "actual value reference is required")
		}

		// Expected is required for comparison assertions
		if v.requiresExpected(assertion.Type) && assertion.Expected == nil {
			result.addError(assertPrefix+".expected", "expected value is required for "+assertion.Type)
		}
	}
}

// requiresExpected returns true if the assertion type needs an expected value
func (v *IRSpecValidator) requiresExpected(assertionType string) bool {
	switch assertionType {
	case "equals", "not_equals", "contains", "not_contains", "greater_than", "less_than", "length", "type_is":
		return true
	case "throws", "truthy", "falsy", "nil", "not_nil":
		return false
	default:
		return false
	}
}

// validateValueType checks if a value matches its type hint
func (v *IRSpecValidator) validateValueType(value interface{}, typeHint string, prefix string, result *ValidationResult) {
	if value == nil {
		if typeHint != "null" {
			result.addWarning(prefix, "null value with non-null type hint", typeHint)
		}
		return
	}

	switch typeHint {
	case "int":
		switch val := value.(type) {
		case float64:
			// JSON numbers are float64 - check if it's a whole number
			if val != float64(int(val)) {
				result.addWarning(prefix, "float value with int type hint")
			}
		case int, int64, int32:
			// OK
		default:
			result.addWarning(prefix+".value", "value doesn't match int type hint")
		}

	case "float":
		switch value.(type) {
		case float64, float32, int, int64:
			// OK - int can be coerced to float
		default:
			result.addWarning(prefix+".value", "value doesn't match float type hint")
		}

	case "string":
		if _, ok := value.(string); !ok {
			result.addWarning(prefix+".value", "value doesn't match string type hint")
		}

	case "bool":
		if _, ok := value.(bool); !ok {
			result.addWarning(prefix+".value", "value doesn't match bool type hint")
		}

	case "null":
		if value != nil {
			result.addWarning(prefix+".value", "non-null value with null type hint")
		}

	case "array":
		if _, ok := value.([]interface{}); !ok {
			result.addWarning(prefix+".value", "value doesn't match array type hint")
		}

	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			result.addWarning(prefix+".value", "value doesn't match object type hint")
		}
	}
}

// Helper methods

func (r *ValidationResult) addError(field, message string, value ...string) {
	err := ValidationError{Field: field, Message: message}
	if len(value) > 0 {
		err.Value = value[0]
	}
	r.Errors = append(r.Errors, err)
	r.Valid = false
}

func (r *ValidationResult) addWarning(field, message string, value ...string) {
	warn := ValidationError{Field: field, Message: message}
	if len(value) > 0 {
		warn.Value = value[0]
	}
	r.Warnings = append(r.Warnings, warn)
}

// ErrorMessages returns all error messages as a slice
func (r *ValidationResult) ErrorMessages() []string {
	msgs := make([]string, len(r.Errors))
	for i, err := range r.Errors {
		msgs[i] = err.Error()
	}
	return msgs
}

// WarningMessages returns all warning messages as a slice
func (r *ValidationResult) WarningMessages() []string {
	msgs := make([]string, len(r.Warnings))
	for i, warn := range r.Warnings {
		msgs[i] = warn.Error()
	}
	return msgs
}

// Summary returns a human-readable summary
func (r *ValidationResult) Summary() string {
	if r.Valid && len(r.Warnings) == 0 {
		return "validation passed"
	}
	return fmt.Sprintf("validation %s: %d errors, %d warnings",
		map[bool]string{true: "passed with warnings", false: "failed"}[r.Valid],
		len(r.Errors), len(r.Warnings))
}

// isValidIdentifier checks if a string is a valid programming identifier
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
		} else {
			if !isIdentifierChar(r) {
				return false
			}
		}
	}
	return true
}

// isIdentifierChar checks if a rune can be part of an identifier
func isIdentifierChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}
