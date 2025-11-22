package generator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/google/uuid"
)

// IRSpecConverter converts IRSpec (LLM output) to TestSpec (internal representation)
type IRSpecConverter struct {
	validator *IRSpecValidator
}

// NewIRSpecConverter creates a new IRSpec converter
func NewIRSpecConverter() *IRSpecConverter {
	return &IRSpecConverter{
		validator: NewIRSpecValidator(),
	}
}

// ParseIRSpec parses JSON output from LLM into IRTestSuite
func (c *IRSpecConverter) ParseIRSpec(jsonData string) (*model.IRTestSuite, error) {
	// Clean up potential markdown code blocks
	jsonData = strings.TrimSpace(jsonData)
	if strings.HasPrefix(jsonData, "```json") {
		jsonData = strings.TrimPrefix(jsonData, "```json")
	} else if strings.HasPrefix(jsonData, "```") {
		jsonData = strings.TrimPrefix(jsonData, "```")
	}
	if strings.HasSuffix(jsonData, "```") {
		jsonData = strings.TrimSuffix(jsonData, "```")
	}
	jsonData = strings.TrimSpace(jsonData)

	var suite model.IRTestSuite
	if err := json.Unmarshal([]byte(jsonData), &suite); err != nil {
		return nil, fmt.Errorf("failed to parse IRSpec JSON: %w", err)
	}

	return &suite, nil
}

// ParseAndValidate parses JSON and validates the IRTestSuite structure
func (c *IRSpecConverter) ParseAndValidate(jsonData string) (*model.IRTestSuite, *ValidationResult, error) {
	suite, err := c.ParseIRSpec(jsonData)
	if err != nil {
		return nil, nil, err
	}

	result := c.validator.Validate(suite)
	if !result.Valid {
		return suite, result, fmt.Errorf("IRSpec validation failed: %s", strings.Join(result.ErrorMessages(), "; "))
	}

	return suite, result, nil
}

// ConvertToTestSpecs converts IRTestSuite to a slice of TestSpec
func (c *IRSpecConverter) ConvertToTestSpecs(suite *model.IRTestSuite) ([]model.TestSpec, error) {
	if suite == nil || len(suite.Tests) == 0 {
		return nil, fmt.Errorf("empty IRSpec suite")
	}

	specs := make([]model.TestSpec, 0, len(suite.Tests))

	for _, tc := range suite.Tests {
		spec, err := c.convertTestCase(suite.FunctionName, tc)
		if err != nil {
			// Log warning but continue with other tests
			continue
		}
		specs = append(specs, spec)
	}

	if len(specs) == 0 {
		return nil, fmt.Errorf("no valid test cases converted")
	}

	return specs, nil
}

// convertTestCase converts a single IRTestCase to TestSpec
func (c *IRSpecConverter) convertTestCase(functionName string, tc model.IRTestCase) (model.TestSpec, error) {
	spec := model.TestSpec{
		ID:           uuid.New().String(),
		Level:        model.LevelUnit,
		TargetKind:   "function",
		TargetID:     functionName,
		FunctionName: functionName,
		Description:  tc.Description,
		Tags:         tc.Tags,
	}

	// Convert Given section to Inputs with type hints
	spec.Inputs = make(map[string]interface{})
	spec.InputTypes = make(map[string]string)
	for _, v := range tc.Given {
		spec.Inputs[v.Name] = v.Value
		if v.Type != "" {
			spec.InputTypes[v.Name] = v.Type
		}
	}

	// Preserve argument order from When.Args
	if len(tc.When.Args) > 0 {
		spec.ArgOrder = tc.When.Args
	}

	// Convert Then section to Assertions
	spec.Assertions = make([]model.Assertion, 0, len(tc.Then))
	for _, a := range tc.Then {
		assertion := c.convertAssertion(a)
		spec.Assertions = append(spec.Assertions, assertion)
	}

	// Set description from name if not provided
	if spec.Description == "" {
		spec.Description = formatTestName(tc.Name)
	}

	return spec, nil
}

// convertAssertion converts IRAssertion to model.Assertion
func (c *IRSpecConverter) convertAssertion(ir model.IRAssertion) model.Assertion {
	// Map IRSpec assertion types to model.Assertion kinds
	kindMap := map[string]string{
		"equals":       "equality",
		"not_equals":   "not_equal",
		"contains":     "contains",
		"not_contains": "not_contains",
		"greater_than": "greater_than",
		"less_than":    "less_than",
		"throws":       "throws",
		"truthy":       "truthy",
		"falsy":        "falsy",
		"nil":          "nil",
		"not_nil":      "not_nil",
		"length":       "length",
		"type_is":      "type_is",
	}

	kind := ir.Type
	if mapped, ok := kindMap[ir.Type]; ok {
		kind = mapped
	}

	return model.Assertion{
		Kind:     kind,
		Actual:   ir.Actual,
		Expected: ir.Expected,
	}
}

// formatTestName converts snake_case to human-readable format
func formatTestName(name string) string {
	// Replace underscores with spaces
	result := strings.ReplaceAll(name, "_", " ")
	// Capitalize first letter
	if len(result) > 0 {
		result = strings.ToUpper(string(result[0])) + result[1:]
	}
	return result
}

// ParseAndConvert is a convenience method that parses, validates, and converts in one step
func (c *IRSpecConverter) ParseAndConvert(jsonData string) ([]model.TestSpec, error) {
	suite, validationResult, err := c.ParseAndValidate(jsonData)
	if err != nil {
		// Log warnings even on error for debugging
		if validationResult != nil && len(validationResult.Warnings) > 0 {
			// In production, this would be logged
			_ = validationResult.WarningMessages()
		}
		return nil, err
	}

	// Log warnings for debugging (in production, use proper logging)
	if len(validationResult.Warnings) > 0 {
		_ = validationResult.WarningMessages()
	}

	return c.ConvertToTestSpecs(suite)
}

// Validate validates an already-parsed IRTestSuite
func (c *IRSpecConverter) Validate(suite *model.IRTestSuite) *ValidationResult {
	return c.validator.Validate(suite)
}
