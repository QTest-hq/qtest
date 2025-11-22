package adapters

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/QTest-hq/qtest/pkg/model"
)

// JestSpecAdapter generates Jest test code from model.TestSpec
type JestSpecAdapter struct{}

func NewJestSpecAdapter() *JestSpecAdapter {
	return &JestSpecAdapter{}
}

func (a *JestSpecAdapter) Framework() Framework {
	return FrameworkJest
}

func (a *JestSpecAdapter) FileExtension() string {
	return ".ts"
}

func (a *JestSpecAdapter) TestFileSuffix() string {
	return ".test"
}

const jestSpecTemplate = `{{range .Imports}}
import {{.}};
{{end}}

{{range .Tests}}
describe('{{.DescribeName}}', () => {
{{range .Cases}}
  test('{{.Name}}', () => {
    // Arrange
{{if .Setup}}{{.Setup}}{{end}}
    // Act
    {{.Action}}

    // Assert
{{range .Assertions}}    {{.}}
{{end}}  });

{{end}}});
{{end}}
`

type jestSpecTemplateData struct {
	Imports []string
	Tests   []jestSpecTestData
}

type jestSpecTestData struct {
	DescribeName string
	Cases        []jestSpecCaseData
}

type jestSpecCaseData struct {
	Name       string
	Setup      string
	Action     string
	Assertions []string
}

// GenerateFromSpecs generates Jest test code from TestSpec slice
func (a *JestSpecAdapter) GenerateFromSpecs(specs []model.TestSpec, sourceFile string) (string, error) {
	if len(specs) == 0 {
		return "", fmt.Errorf("no test specs provided")
	}

	// Group specs by target function
	specsByFunc := make(map[string][]model.TestSpec)
	for _, spec := range specs {
		funcName := spec.FunctionName
		if funcName == "" {
			funcName = spec.TargetID
		}
		specsByFunc[funcName] = append(specsByFunc[funcName], spec)
	}

	data := jestSpecTemplateData{
		Imports: []string{},
		Tests:   make([]jestSpecTestData, 0),
	}

	// Add import for the module being tested
	moduleName := extractJSModuleName(sourceFile)
	if moduleName != "" {
		// Collect all function names for import
		funcNames := make([]string, 0, len(specsByFunc))
		for funcName := range specsByFunc {
			funcNames = append(funcNames, funcName)
		}
		data.Imports = append(data.Imports, fmt.Sprintf("{ %s } from '%s'", strings.Join(funcNames, ", "), moduleName))
	}

	// Build tests grouped by function
	for funcName, funcSpecs := range specsByFunc {
		testData := jestSpecTestData{
			DescribeName: funcName,
			Cases:        make([]jestSpecCaseData, 0),
		}

		for _, spec := range funcSpecs {
			caseData := jestSpecCaseData{
				Name:       spec.Description,
				Assertions: make([]string, 0),
			}

			// Generate setup from inputs with type hints
			if len(spec.Inputs) > 0 {
				caseData.Setup = a.generateSetup(spec)
			}

			// Generate action (function call)
			caseData.Action = a.generateAction(spec)

			// Generate assertions from spec.Assertions
			for _, assertion := range spec.Assertions {
				assertCode := a.generateAssertion(assertion)
				if assertCode != "" {
					caseData.Assertions = append(caseData.Assertions, assertCode)
				}
			}

			// If no assertions were generated, add a placeholder
			if len(caseData.Assertions) == 0 {
				caseData.Assertions = append(caseData.Assertions, "// TODO: Add assertions")
			}

			testData.Cases = append(testData.Cases, caseData)
		}

		data.Tests = append(data.Tests, testData)
	}

	// Execute template
	tmpl, err := template.New("jestspec").Parse(jestSpecTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// generateSetup generates setup code from inputs with type hints
func (a *JestSpecAdapter) generateSetup(spec model.TestSpec) string {
	var setup strings.Builder

	// Use ArgOrder if available, otherwise sort keys
	var keys []string
	if len(spec.ArgOrder) > 0 {
		keys = spec.ArgOrder
	} else {
		// Separate named args from indexed args
		var namedKeys []string
		var indexedKeys []string
		for key := range spec.Inputs {
			if strings.HasPrefix(key, "arg") {
				indexedKeys = append(indexedKeys, key)
			} else {
				namedKeys = append(namedKeys, key)
			}
		}

		// If we have named args, use those; otherwise use indexed args
		if len(namedKeys) > 0 {
			sort.Strings(namedKeys)
			keys = namedKeys
		} else {
			sort.Slice(indexedKeys, func(i, j int) bool {
				numI, _ := strconv.Atoi(strings.TrimPrefix(indexedKeys[i], "arg"))
				numJ, _ := strconv.Atoi(strings.TrimPrefix(indexedKeys[j], "arg"))
				return numI < numJ
			})
			keys = indexedKeys
		}
	}

	for _, key := range keys {
		value, ok := spec.Inputs[key]
		if !ok {
			continue
		}
		typeHint := ""
		if spec.InputTypes != nil {
			typeHint = spec.InputTypes[key]
		}
		// Sanitize variable names with dots (e.g., "req.body" -> "reqBody")
		varName := sanitizeJSVarName(key)
		setup.WriteString(fmt.Sprintf("    const %s = %s;\n", varName, formatJSValueWithType(value, typeHint)))
	}
	return setup.String()
}

// sanitizeJSVarName converts dotted names to valid JS variable names
func sanitizeJSVarName(name string) string {
	// Replace dots with camelCase
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		return name
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			result += strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return result
}

// generateAction generates the function call
func (a *JestSpecAdapter) generateAction(spec model.TestSpec) string {
	funcName := spec.FunctionName
	if funcName == "" {
		funcName = spec.TargetID
	}

	// Use ArgOrder if available
	var args []string
	if len(spec.ArgOrder) > 0 {
		args = spec.ArgOrder
	} else {
		// Build args from inputs
		var namedArgs []string
		var indexedArgs []string

		for key := range spec.Inputs {
			if strings.HasPrefix(key, "arg") {
				indexedArgs = append(indexedArgs, key)
			} else {
				namedArgs = append(namedArgs, key)
			}
		}

		// Use named args if available
		if len(namedArgs) > 0 {
			sort.Strings(namedArgs)
			args = namedArgs
		} else if len(indexedArgs) > 0 {
			sort.Slice(indexedArgs, func(i, j int) bool {
				numI, _ := strconv.Atoi(strings.TrimPrefix(indexedArgs[i], "arg"))
				numJ, _ := strconv.Atoi(strings.TrimPrefix(indexedArgs[j], "arg"))
				return numI < numJ
			})
			args = indexedArgs
		}
	}

	// Sanitize argument names with dots
	sanitizedArgs := make([]string, len(args))
	for i, arg := range args {
		sanitizedArgs[i] = sanitizeJSVarName(arg)
	}

	return fmt.Sprintf("const result = %s(%s);", funcName, strings.Join(sanitizedArgs, ", "))
}

// formatJSValueWithType formats a value for JavaScript code using type hints
func formatJSValueWithType(val interface{}, typeHint string) string {
	if val == nil {
		return "null"
	}

	switch typeHint {
	case "int":
		switch v := val.(type) {
		case float64:
			return fmt.Sprintf("%d", int(v))
		case int:
			return fmt.Sprintf("%d", v)
		default:
			return fmt.Sprintf("%v", v)
		}
	case "float":
		return fmt.Sprintf("%v", val)
	case "string":
		return fmt.Sprintf("'%v'", val)
	case "bool":
		if b, ok := val.(bool); ok {
			if b {
				return "true"
			}
			return "false"
		}
		return fmt.Sprintf("%v", val)
	case "null":
		return "null"
	case "array":
		if arr, ok := val.([]interface{}); ok {
			elements := make([]string, len(arr))
			for i, elem := range arr {
				elements[i] = formatJSValue(elem)
			}
			return fmt.Sprintf("[%s]", strings.Join(elements, ", "))
		}
		return "[]"
	case "object":
		if m, ok := val.(map[string]interface{}); ok {
			pairs := make([]string, 0, len(m))
			for key, value := range m {
				pairs = append(pairs, fmt.Sprintf("%s: %s", key, formatJSValue(value)))
			}
			return fmt.Sprintf("{ %s }", strings.Join(pairs, ", "))
		}
		return "{}"
	default:
		return formatJSValue(val)
	}
}

// stripDollarPrefix removes $ prefix from variable references like "$res.status" -> "res.status"
func stripDollarPrefix(s string) string {
	if strings.HasPrefix(s, "$") {
		return s[1:]
	}
	return s
}

// generateAssertion generates Jest assertion code from model.Assertion
func (a *JestSpecAdapter) generateAssertion(assertion model.Assertion) string {
	actual := stripDollarPrefix(assertion.Actual)
	if actual == "" {
		actual = "result"
	}

	switch assertion.Kind {
	case "equality", "equals":
		expected := formatJSValue(assertion.Expected)
		return fmt.Sprintf("expect(%s).toBe(%s);", actual, expected)

	case "not_equal", "not_equals":
		expected := formatJSValue(assertion.Expected)
		return fmt.Sprintf("expect(%s).not.toBe(%s);", actual, expected)

	case "not_null", "not_nil", "is_not_nil":
		return fmt.Sprintf("expect(%s).not.toBeNull();", actual)

	case "null", "nil", "is_nil":
		return fmt.Sprintf("expect(%s).toBeNull();", actual)

	case "contains":
		expected := formatJSValue(assertion.Expected)
		return fmt.Sprintf("expect(%s).toContain(%s);", actual, expected)

	case "greater_than":
		expected := formatJSValue(assertion.Expected)
		return fmt.Sprintf("expect(%s).toBeGreaterThan(%s);", actual, expected)

	case "less_than":
		expected := formatJSValue(assertion.Expected)
		return fmt.Sprintf("expect(%s).toBeLessThan(%s);", actual, expected)

	case "truthy":
		return fmt.Sprintf("expect(%s).toBeTruthy();", actual)

	case "falsy":
		return fmt.Sprintf("expect(%s).toBeFalsy();", actual)

	case "throws", "error":
		return "expect(() => result).toThrow();"

	case "type", "type_is":
		expected := assertion.Expected
		return fmt.Sprintf("expect(typeof %s).toBe('%s');", actual, expected)

	case "length":
		expected := formatJSValue(assertion.Expected)
		return fmt.Sprintf("expect(%s).toHaveLength(%s);", actual, expected)

	default:
		if assertion.Expected != nil {
			expected := formatJSValue(assertion.Expected)
			return fmt.Sprintf("expect(%s).toBe(%s);", actual, expected)
		}
		return ""
	}
}

// formatJSValue formats a value for JavaScript code
func formatJSValue(val interface{}) string {
	if val == nil {
		return "null"
	}

	switch v := val.(type) {
	case string:
		// Check if it looks like a number
		if _, err := fmt.Sscanf(v, "%d", new(int)); err == nil {
			return v
		}
		if _, err := fmt.Sscanf(v, "%f", new(float64)); err == nil {
			return v
		}
		if v == "true" || v == "false" {
			return v
		}
		return fmt.Sprintf("'%s'", v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []interface{}:
		elements := make([]string, len(v))
		for i, elem := range v {
			elements[i] = formatJSValue(elem)
		}
		return fmt.Sprintf("[%s]", strings.Join(elements, ", "))
	case map[string]interface{}:
		pairs := make([]string, 0, len(v))
		for key, value := range v {
			pairs = append(pairs, fmt.Sprintf("%s: %s", key, formatJSValue(value)))
		}
		return fmt.Sprintf("{ %s }", strings.Join(pairs, ", "))
	default:
		return fmt.Sprintf("%v", v)
	}
}

// extractJSModuleName extracts the JS module name from a file path
// For test files that are in the same directory, we use just the filename
func extractJSModuleName(filePath string) string {
	if filePath == "" {
		return ""
	}

	// Get just the filename without path
	parts := strings.Split(filePath, "/")
	filename := parts[len(parts)-1]

	// Also handle Windows paths
	parts = strings.Split(filename, "\\")
	filename = parts[len(parts)-1]

	// Remove extensions
	name := strings.TrimSuffix(filename, ".ts")
	name = strings.TrimSuffix(name, ".js")
	name = strings.TrimSuffix(name, ".tsx")
	name = strings.TrimSuffix(name, ".jsx")

	// Make it a relative import
	return "./" + name
}
