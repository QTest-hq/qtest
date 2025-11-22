package generator

import (
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/pkg/dsl"
	"gopkg.in/yaml.v3"
)

// SimpleTest represents the simpler format that LLMs naturally generate
type SimpleTest struct {
	Name       string                 `yaml:"name"`
	Setup      map[string]interface{} `yaml:"setup,omitempty"`
	Action     interface{}            `yaml:"action"`     // Can be string or map
	Assertions interface{}            `yaml:"assertions"` // Can be map or list
	Assertion  interface{}            `yaml:"assertion"`  // Singular form
	Assert     interface{}            `yaml:"assert"`     // Alternative name for assertions
	Expected   interface{}            `yaml:"expected"`   // Another alternative
	Expect     interface{}            `yaml:"expect"`     // Another alternative
}

// GetAssertions returns assertions from whichever field is populated
func (t *SimpleTest) GetAssertions() interface{} {
	if t.Assertions != nil {
		return t.Assertions
	}
	if t.Assertion != nil {
		return t.Assertion
	}
	if t.Assert != nil {
		return t.Assert
	}
	if t.Expected != nil {
		return t.Expected
	}
	if t.Expect != nil {
		return t.Expect
	}
	return nil
}

// SimpleTestList is a list of simple tests (what LLM typically returns)
type SimpleTestList []SimpleTest

// ConvertToDSL converts a simple test format to our full DSL
func ConvertToDSL(yamlContent string, funcName, filePath, language string) (*dsl.TestDSL, error) {
	// First, try parsing as our full DSL format
	var fullDSL dsl.TestDSL
	if err := yaml.Unmarshal([]byte(yamlContent), &fullDSL); err == nil && fullDSL.Name != "" {
		return &fullDSL, nil
	}

	// Try parsing as a list of simple tests
	var simpleTests SimpleTestList
	if err := yaml.Unmarshal([]byte(yamlContent), &simpleTests); err == nil && len(simpleTests) > 0 {
		return convertSimpleListToDSL(simpleTests, funcName, filePath, language)
	}

	// Try parsing as a single simple test
	var singleTest SimpleTest
	if err := yaml.Unmarshal([]byte(yamlContent), &singleTest); err == nil && singleTest.Name != "" {
		return convertSimpleListToDSL([]SimpleTest{singleTest}, funcName, filePath, language)
	}

	// Try parsing as a wrapper with "tests" key
	var wrapper struct {
		Tests []SimpleTest `yaml:"tests"`
	}
	if err := yaml.Unmarshal([]byte(yamlContent), &wrapper); err == nil && len(wrapper.Tests) > 0 {
		return convertSimpleListToDSL(wrapper.Tests, funcName, filePath, language)
	}

	return nil, fmt.Errorf("unable to parse test format")
}

// convertSimpleListToDSL converts a list of simple tests to our DSL
func convertSimpleListToDSL(tests []SimpleTest, funcName, filePath, language string) (*dsl.TestDSL, error) {
	result := &dsl.TestDSL{
		Version: "1.0",
		Name:    fmt.Sprintf("Test_%s", funcName),
		Type:    dsl.TestTypeUnit,
		Target: dsl.TestTarget{
			File:     filePath,
			Function: funcName,
		},
		Steps: make([]dsl.TestStep, 0),
	}

	for i, test := range tests {
		step := dsl.TestStep{
			ID:          fmt.Sprintf("step_%d", i+1),
			Description: test.Name,
			Action: dsl.StepAction{
				Type:   dsl.ActionCall,
				Target: funcName,
			},
		}

		// Parse action - can be string or map
		args := parseAction(test.Action, funcName)
		if len(args) > 0 {
			// Resolve variable references from setup
			step.Action.Args = resolveArgs(args, test.Setup)
		}

		// Use setup values as arguments if action didn't have args
		if len(step.Action.Args) == 0 && len(test.Setup) > 0 {
			for _, v := range test.Setup {
				step.Action.Args = append(step.Action.Args, v)
			}
		}

		// Parse setup as input
		if len(test.Setup) > 0 {
			step.Input = test.Setup
		}

		// Parse assertions as expected
		step.Expected = parseAssertions(test.GetAssertions())

		result.Steps = append(result.Steps, step)
	}

	return result, nil
}

// resolveArgs replaces variable references in args with actual values from setup
func resolveArgs(args []interface{}, setup map[string]interface{}) []interface{} {
	if len(setup) == 0 {
		return args
	}

	resolved := make([]interface{}, len(args))
	for i, arg := range args {
		if strArg, ok := arg.(string); ok {
			// Try to resolve variable reference
			varName := strArg

			// Handle ${var}, $var, or bare var patterns
			if strings.HasPrefix(strArg, "${") && strings.HasSuffix(strArg, "}") {
				varName = strArg[2 : len(strArg)-1]
			} else if strings.HasPrefix(strArg, "$") {
				varName = strArg[1:]
			}

			// Look up in setup map
			if val, exists := setup[varName]; exists {
				resolved[i] = val
			} else {
				resolved[i] = arg
			}
		} else {
			resolved[i] = arg
		}
	}
	return resolved
}

// parseAction extracts arguments from various action formats
func parseAction(action interface{}, funcName string) []interface{} {
	if action == nil {
		return nil
	}

	switch v := action.(type) {
	case string:
		return parseActionArgs(v)
	case map[string]interface{}:
		// Format: {function: "Add", args: [...]} or {function: "Add", args: {a: 1, b: 2}}
		if args, ok := v["args"]; ok {
			switch a := args.(type) {
			case []interface{}:
				return a
			case map[string]interface{}:
				var result []interface{}
				for _, val := range a {
					result = append(result, val)
				}
				return result
			}
		}
	}
	return nil
}

// parseActionArgs extracts arguments from an action string like "Add(2, 3)"
func parseActionArgs(action string) []interface{} {
	// Find parentheses
	start := strings.Index(action, "(")
	end := strings.LastIndex(action, ")")

	if start == -1 || end == -1 || end <= start {
		return nil
	}

	argsStr := action[start+1 : end]
	if argsStr == "" {
		return nil
	}

	// Split by comma and trim
	parts := strings.Split(argsStr, ",")
	args := make([]interface{}, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			args = append(args, p)
		}
	}

	return args
}

// parseAssertions converts various assertion formats to Expected
func parseAssertions(assertions interface{}) *dsl.Expected {
	if assertions == nil {
		return nil
	}

	expected := &dsl.Expected{
		Properties: make(map[string]interface{}),
	}

	switch v := assertions.(type) {
	case map[string]interface{}:
		// Direct map: {result: 5} or {expect: "result == 5"}
		if result, ok := v["result"]; ok {
			expected.Value = result
		}
		// Handle expect: "result == value" format
		if expect, ok := v["expect"]; ok {
			if val := parseExpectExpression(expect); val != nil {
				expected.Value = val
			}
		}
		expected.Properties = v

	case []interface{}:
		// List of assertions
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if result, ok := m["result"]; ok {
					expected.Value = result
				}
				// Handle expect: "result == value" format
				if expect, ok := m["expect"]; ok {
					if val := parseExpectExpression(expect); val != nil {
						expected.Value = val
					}
				}
				for k, val := range m {
					expected.Properties[k] = val
				}
			}
		}

	default:
		expected.Value = v
	}

	return expected
}

// parseExpectExpression extracts value from expressions like "result == 8"
func parseExpectExpression(expr interface{}) interface{} {
	str, ok := expr.(string)
	if !ok {
		return nil
	}

	// Handle "result == value" format
	if strings.Contains(str, "==") {
		parts := strings.Split(str, "==")
		if len(parts) == 2 {
			valueStr := strings.TrimSpace(parts[1])
			// Try to parse as number
			var intVal int
			if _, err := fmt.Sscanf(valueStr, "%d", &intVal); err == nil {
				return intVal
			}
			var floatVal float64
			if _, err := fmt.Sscanf(valueStr, "%f", &floatVal); err == nil {
				return floatVal
			}
			// Return as string if not a number
			return valueStr
		}
	}

	return nil
}
