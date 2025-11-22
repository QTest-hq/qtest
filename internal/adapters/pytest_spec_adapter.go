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

// PytestSpecAdapter generates pytest code from model.TestSpec
type PytestSpecAdapter struct{}

func NewPytestSpecAdapter() *PytestSpecAdapter {
	return &PytestSpecAdapter{}
}

func (a *PytestSpecAdapter) Framework() Framework {
	return FrameworkPytest
}

func (a *PytestSpecAdapter) FileExtension() string {
	return ".py"
}

func (a *PytestSpecAdapter) TestFileSuffix() string {
	return "_test"
}

const pytestSpecTemplate = `import pytest
{{if .Imports}}{{range .Imports}}
{{.}}{{end}}
{{end}}

{{range .Tests}}
class Test{{.ClassName}}:
    """Tests for {{.ClassName}}"""
{{range .Cases}}
    def test_{{.Name}}(self):
        """{{.Description}}"""
        # Arrange
{{if .Setup}}{{.Setup}}{{end}}
        # Act
        {{.Action}}

        # Assert
{{range .Assertions}}        {{.}}
{{end}}
{{end}}
{{end}}
`

type pytestSpecTemplateData struct {
	Imports []string
	Tests   []pytestSpecTestData
}

type pytestSpecTestData struct {
	ClassName string
	Cases     []pytestSpecCaseData
}

type pytestSpecCaseData struct {
	Name        string
	Description string
	Setup       string
	Action      string
	Assertions  []string
}

// GenerateFromSpecs generates pytest code from TestSpec slice
func (a *PytestSpecAdapter) GenerateFromSpecs(specs []model.TestSpec, sourceFile string) (string, error) {
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

	data := pytestSpecTemplateData{
		Imports: []string{},
		Tests:   make([]pytestSpecTestData, 0),
	}

	// Add import for the module being tested
	moduleName := extractPythonModuleName(sourceFile)
	if moduleName != "" {
		// Collect all function names for import
		funcNames := make([]string, 0, len(specsByFunc))
		for funcName := range specsByFunc {
			funcNames = append(funcNames, funcName)
		}
		sort.Strings(funcNames) // Deterministic order
		data.Imports = append(data.Imports, fmt.Sprintf("from %s import %s", moduleName, strings.Join(funcNames, ", ")))
	}

	// Build tests grouped by function
	for funcName, funcSpecs := range specsByFunc {
		testData := pytestSpecTestData{
			ClassName: toPythonClassName(funcName),
			Cases:     make([]pytestSpecCaseData, 0),
		}

		for _, spec := range funcSpecs {
			caseData := pytestSpecCaseData{
				Name:        toPythonTestName(spec.Description),
				Description: spec.Description,
				Assertions:  make([]string, 0),
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
				caseData.Assertions = append(caseData.Assertions, "# TODO: Add assertions")
			}

			testData.Cases = append(testData.Cases, caseData)
		}

		data.Tests = append(data.Tests, testData)
	}

	// Execute template
	tmpl, err := template.New("pytestspec").Parse(pytestSpecTemplate)
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
func (a *PytestSpecAdapter) generateSetup(spec model.TestSpec) string {
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
		setup.WriteString(fmt.Sprintf("        %s = %s\n", key, formatPythonValueWithType(value, typeHint)))
	}
	return setup.String()
}

// generateAction generates the function call
func (a *PytestSpecAdapter) generateAction(spec model.TestSpec) string {
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

	return fmt.Sprintf("result = %s(%s)", funcName, strings.Join(args, ", "))
}

// formatPythonValueWithType formats a value for Python code using type hints
func formatPythonValueWithType(val interface{}, typeHint string) string {
	if val == nil {
		return "None"
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
		return fmt.Sprintf("%q", val)
	case "bool":
		if b, ok := val.(bool); ok {
			if b {
				return "True"
			}
			return "False"
		}
		return fmt.Sprintf("%v", val)
	case "null":
		return "None"
	case "array":
		if arr, ok := val.([]interface{}); ok {
			elements := make([]string, len(arr))
			for i, elem := range arr {
				elements[i] = formatPythonValue(elem)
			}
			return fmt.Sprintf("[%s]", strings.Join(elements, ", "))
		}
		return "[]"
	case "object":
		if m, ok := val.(map[string]interface{}); ok {
			pairs := make([]string, 0, len(m))
			for key, value := range m {
				pairs = append(pairs, fmt.Sprintf("%q: %s", key, formatPythonValue(value)))
			}
			return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
		}
		return "{}"
	default:
		return formatPythonValue(val)
	}
}

// stripPythonDollarPrefix removes $ prefix from variable references like "$res.status" -> "res.status"
func stripPythonDollarPrefix(s string) string {
	if strings.HasPrefix(s, "$") {
		return s[1:]
	}
	return s
}

// generateAssertion generates pytest assertion code from model.Assertion
func (a *PytestSpecAdapter) generateAssertion(assertion model.Assertion) string {
	actual := stripPythonDollarPrefix(assertion.Actual)
	if actual == "" {
		actual = "result"
	}

	switch assertion.Kind {
	case "equality", "equals":
		expected := formatPythonValue(assertion.Expected)
		return fmt.Sprintf("assert %s == %s", actual, expected)

	case "not_equal", "not_equals":
		expected := formatPythonValue(assertion.Expected)
		return fmt.Sprintf("assert %s != %s", actual, expected)

	case "not_null", "not_nil", "is_not_nil":
		return fmt.Sprintf("assert %s is not None", actual)

	case "null", "nil", "is_nil":
		return fmt.Sprintf("assert %s is None", actual)

	case "contains":
		expected := formatPythonValue(assertion.Expected)
		return fmt.Sprintf("assert %s in %s", expected, actual)

	case "greater_than":
		expected := formatPythonValue(assertion.Expected)
		return fmt.Sprintf("assert %s > %s", actual, expected)

	case "less_than":
		expected := formatPythonValue(assertion.Expected)
		return fmt.Sprintf("assert %s < %s", actual, expected)

	case "truthy":
		return fmt.Sprintf("assert %s", actual)

	case "falsy":
		return fmt.Sprintf("assert not %s", actual)

	case "throws", "error":
		return "# Exception is expected - wrap call in pytest.raises()"

	case "type", "type_is":
		expected := assertion.Expected
		return fmt.Sprintf("assert isinstance(%s, %s)", actual, expected)

	case "length":
		expected := formatPythonValue(assertion.Expected)
		return fmt.Sprintf("assert len(%s) == %s", actual, expected)

	default:
		if assertion.Expected != nil {
			expected := formatPythonValue(assertion.Expected)
			return fmt.Sprintf("assert %s == %s", actual, expected)
		}
		return ""
	}
}

// formatPythonValue formats a value for Python code
func formatPythonValue(val interface{}) string {
	if val == nil {
		return "None"
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
		if v == "true" || v == "True" {
			return "True"
		}
		if v == "false" || v == "False" {
			return "False"
		}
		return fmt.Sprintf("%q", v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "True"
		}
		return "False"
	case []interface{}:
		elements := make([]string, len(v))
		for i, elem := range v {
			elements[i] = formatPythonValue(elem)
		}
		return fmt.Sprintf("[%s]", strings.Join(elements, ", "))
	case map[string]interface{}:
		pairs := make([]string, 0, len(v))
		for key, value := range v {
			pairs = append(pairs, fmt.Sprintf("%q: %s", key, formatPythonValue(value)))
		}
		return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
	default:
		return fmt.Sprintf("%v", v)
	}
}

// toPythonClassName converts a function name to a Python class name (PascalCase)
func toPythonClassName(name string) string {
	if name == "" {
		return "Unknown"
	}

	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == ' ' || r == '_' || r == '-' || r == '.'
	})

	var result strings.Builder
	for _, word := range words {
		if len(word) > 0 {
			result.WriteString(strings.ToUpper(word[:1]))
			if len(word) > 1 {
				result.WriteString(word[1:])
			}
		}
	}

	return result.String()
}

// toPythonTestName converts a description to a valid Python test name (snake_case)
func toPythonTestName(name string) string {
	if name == "" {
		return "test_case"
	}
	// Replace spaces and special characters
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.ReplaceAll(name, "(", "")
	name = strings.ReplaceAll(name, ")", "")
	name = strings.ReplaceAll(name, ",", "_")
	return name
}

// extractPythonModuleName extracts the Python module name from a file path
// For test files that are in the same directory, we use just the filename
func extractPythonModuleName(filePath string) string {
	if filePath == "" {
		return ""
	}

	// Get just the filename without path
	parts := strings.Split(filePath, "/")
	filename := parts[len(parts)-1]

	// Also handle Windows paths
	parts = strings.Split(filename, "\\")
	filename = parts[len(parts)-1]

	// Remove .py extension
	name := strings.TrimSuffix(filename, ".py")

	// Handle invalid Python identifiers (e.g., hyphens)
	// Replace hyphens with underscores
	name = strings.ReplaceAll(name, "-", "_")

	return name
}
