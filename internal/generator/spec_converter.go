package generator

import (
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// ConvertToTestSpec converts LLM YAML output to model.TestSpec with proper Assertions
func ConvertToTestSpec(yamlContent string, funcName, filePath, language string) ([]model.TestSpec, error) {
	// Try parsing as a list of simple tests
	var simpleTests SimpleTestList
	if err := yaml.Unmarshal([]byte(yamlContent), &simpleTests); err == nil && len(simpleTests) > 0 {
		return convertSimpleTestsToSpecs(simpleTests, funcName, filePath, language)
	}

	// Try parsing as a single simple test
	var singleTest SimpleTest
	if err := yaml.Unmarshal([]byte(yamlContent), &singleTest); err == nil && singleTest.Name != "" {
		return convertSimpleTestsToSpecs([]SimpleTest{singleTest}, funcName, filePath, language)
	}

	// Try parsing as a wrapper with "tests" key
	var wrapper struct {
		Tests []SimpleTest `yaml:"tests"`
	}
	if err := yaml.Unmarshal([]byte(yamlContent), &wrapper); err == nil && len(wrapper.Tests) > 0 {
		return convertSimpleTestsToSpecs(wrapper.Tests, funcName, filePath, language)
	}

	return nil, fmt.Errorf("unable to parse test format into TestSpec")
}

// convertSimpleTestsToSpecs converts simple LLM tests to model.TestSpec with rich Assertions
func convertSimpleTestsToSpecs(tests []SimpleTest, funcName, filePath, language string) ([]model.TestSpec, error) {
	specs := make([]model.TestSpec, 0, len(tests))

	for _, test := range tests {
		spec := model.TestSpec{
			ID:           uuid.New().String(),
			Level:        model.LevelUnit,
			TargetKind:   "function",
			TargetID:     funcName,
			Description:  test.Name,
			FunctionName: funcName,
			Inputs:       make(map[string]interface{}),
			Assertions:   make([]model.Assertion, 0),
			Priority:     "normal",
		}

		// Parse action to get function args
		args := parseAction(test.Action, funcName)

		// Use setup values as inputs
		if len(test.Setup) > 0 {
			spec.Inputs = test.Setup
		}

		// Resolve variable references in args and add to inputs
		if len(args) > 0 {
			resolvedArgs := resolveArgs(args, test.Setup)
			for i, arg := range resolvedArgs {
				key := fmt.Sprintf("arg%d", i)
				// If we have a setup key that maps to this position, use it
				for k := range test.Setup {
					if i == 0 || strings.HasSuffix(k, fmt.Sprintf("%d", i+1)) {
						key = k
						break
					}
				}
				spec.Inputs[key] = arg
			}
		}

		// Convert assertions to model.Assertion format
		assertions := extractAssertions(test.GetAssertions(), funcName)
		spec.Assertions = assertions

		// If no assertions were extracted, create a default one
		if len(spec.Assertions) == 0 && len(args) > 0 {
			// At minimum, we should have an assertion that result is not nil
			spec.Assertions = append(spec.Assertions, model.Assertion{
				Kind:   "not_null",
				Actual: "result",
			})
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

// extractAssertions extracts model.Assertion from various assertion formats
func extractAssertions(assertions interface{}, funcName string) []model.Assertion {
	result := make([]model.Assertion, 0)

	if assertions == nil {
		return result
	}

	switch v := assertions.(type) {
	case string:
		// String assertion like "result == 15"
		if assertion := parseExpectAssertion(v); assertion != nil {
			result = append(result, *assertion)
		} else {
			// If we can't parse it, treat as direct value
			result = append(result, model.Assertion{
				Kind:     "equality",
				Actual:   "result",
				Expected: v,
			})
		}

	case map[string]interface{}:
		// Direct map: {result: 5} or {expect: "result == 5"}
		extractAssertionsFromMap(v, &result)

	case []interface{}:
		// List of assertions
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				extractAssertionsFromMap(m, &result)
			} else if s, ok := item.(string); ok {
				// String assertion in list
				if assertion := parseExpectAssertion(s); assertion != nil {
					result = append(result, *assertion)
				}
			}
		}

	default:
		// Direct value assertion
		result = append(result, model.Assertion{
			Kind:     "equality",
			Actual:   "result",
			Expected: v,
		})
	}

	return result
}

// extractAssertionsFromMap extracts assertions from a map
func extractAssertionsFromMap(m map[string]interface{}, result *[]model.Assertion) {
	// Handle "result: value" format - this is the expected return value
	if resultVal, ok := m["result"]; ok {
		*result = append(*result, model.Assertion{
			Kind:     "equality",
			Actual:   "result",
			Expected: resultVal,
		})
	}

	// Handle "expect: expression" format like "result == 5"
	if expect, ok := m["expect"]; ok {
		if expr, ok := expect.(string); ok {
			if assertion := parseExpectAssertion(expr); assertion != nil {
				*result = append(*result, *assertion)
			}
		}
	}

	// Handle "error: true/message" format
	if errVal, ok := m["error"]; ok {
		switch e := errVal.(type) {
		case bool:
			if e {
				*result = append(*result, model.Assertion{
					Kind:   "not_null",
					Actual: "error",
				})
			}
		case string:
			*result = append(*result, model.Assertion{
				Kind:     "contains",
				Actual:   "error",
				Expected: e,
			})
		}
	}

	// Handle "contains: value" format
	if contains, ok := m["contains"]; ok {
		*result = append(*result, model.Assertion{
			Kind:     "contains",
			Actual:   "result",
			Expected: contains,
		})
	}

	// Handle "type: typename" format
	if typeVal, ok := m["type"]; ok {
		*result = append(*result, model.Assertion{
			Kind:     "type",
			Actual:   "result",
			Expected: typeVal,
		})
	}

	// Handle property assertions like "length: 5" or "status: 200"
	for key, val := range m {
		if key == "result" || key == "expect" || key == "error" || key == "contains" || key == "type" {
			continue
		}
		// This is a property assertion
		*result = append(*result, model.Assertion{
			Kind:     "equality",
			Actual:   key,
			Expected: val,
		})
	}
}

// parseExpectAssertion parses expressions like "result == 5" into Assertion
func parseExpectAssertion(expr string) *model.Assertion {
	expr = strings.TrimSpace(expr)

	// Handle "result == value" format
	if strings.Contains(expr, "==") {
		parts := strings.SplitN(expr, "==", 2)
		if len(parts) == 2 {
			actual := strings.TrimSpace(parts[0])
			expected := parseValue(strings.TrimSpace(parts[1]))
			return &model.Assertion{
				Kind:     "equality",
				Actual:   actual,
				Expected: expected,
			}
		}
	}

	// Handle "result != value" format
	if strings.Contains(expr, "!=") {
		parts := strings.SplitN(expr, "!=", 2)
		if len(parts) == 2 {
			actual := strings.TrimSpace(parts[0])
			expected := parseValue(strings.TrimSpace(parts[1]))
			return &model.Assertion{
				Kind:     "not_equal",
				Actual:   actual,
				Expected: expected,
			}
		}
	}

	// Handle "result > value" format
	if strings.Contains(expr, ">") && !strings.Contains(expr, ">=") {
		parts := strings.SplitN(expr, ">", 2)
		if len(parts) == 2 {
			actual := strings.TrimSpace(parts[0])
			expected := parseValue(strings.TrimSpace(parts[1]))
			return &model.Assertion{
				Kind:     "greater_than",
				Actual:   actual,
				Expected: expected,
			}
		}
	}

	// Handle "result < value" format
	if strings.Contains(expr, "<") && !strings.Contains(expr, "<=") {
		parts := strings.SplitN(expr, "<", 2)
		if len(parts) == 2 {
			actual := strings.TrimSpace(parts[0])
			expected := parseValue(strings.TrimSpace(parts[1]))
			return &model.Assertion{
				Kind:     "less_than",
				Actual:   actual,
				Expected: expected,
			}
		}
	}

	// Handle "result contains value" format
	if strings.Contains(expr, " contains ") {
		parts := strings.SplitN(expr, " contains ", 2)
		if len(parts) == 2 {
			actual := strings.TrimSpace(parts[0])
			expected := parseValue(strings.TrimSpace(parts[1]))
			return &model.Assertion{
				Kind:     "contains",
				Actual:   actual,
				Expected: expected,
			}
		}
	}

	// Handle "error is nil" format
	if strings.Contains(expr, "is nil") {
		actual := strings.TrimSpace(strings.Replace(expr, "is nil", "", 1))
		return &model.Assertion{
			Kind:     "null",
			Actual:   actual,
			Expected: nil,
		}
	}

	// Handle "error is not nil" format
	if strings.Contains(expr, "is not nil") {
		actual := strings.TrimSpace(strings.Replace(expr, "is not nil", "", 1))
		return &model.Assertion{
			Kind:   "not_null",
			Actual: actual,
		}
	}

	return nil
}

// parseValue parses a string value into an appropriate type
func parseValue(s string) interface{} {
	s = strings.TrimSpace(s)

	// Remove quotes if present
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		return s[1 : len(s)-1]
	}

	// Try to parse as int
	var intVal int
	if _, err := fmt.Sscanf(s, "%d", &intVal); err == nil {
		return intVal
	}

	// Try to parse as float
	var floatVal float64
	if _, err := fmt.Sscanf(s, "%f", &floatVal); err == nil {
		return floatVal
	}

	// Check for boolean
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Check for nil
	if s == "nil" || s == "null" {
		return nil
	}

	// Return as string
	return s
}
