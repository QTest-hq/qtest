package adapters

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/QTest-hq/qtest/pkg/model"
)

// GoSpecAdapter generates Go test code from model.TestSpec
type GoSpecAdapter struct{}

func NewGoSpecAdapter() *GoSpecAdapter {
	return &GoSpecAdapter{}
}

func (a *GoSpecAdapter) Framework() Framework {
	return FrameworkGoTest
}

func (a *GoSpecAdapter) FileExtension() string {
	return ".go"
}

func (a *GoSpecAdapter) TestFileSuffix() string {
	return "_test"
}

const goSpecTemplate = `package {{.Package}}

import (
	"testing"
{{range .Imports}}
	"{{.}}"
{{end}}
)

{{range .Tests}}
func Test{{.TestName}}(t *testing.T) {
{{range .Cases}}
	t.Run("{{.Name}}", func(t *testing.T) {
		{{if .Setup}}// Setup
		{{.Setup}}
		{{end}}
		// Act
		{{.Action}}

		// Assert
{{range .Assertions}}
		{{.}}
{{end}}
	})
{{end}}
}
{{end}}
`

type goSpecTemplateData struct {
	Package string
	Imports []string
	Tests   []goSpecTestData
}

type goSpecTestData struct {
	TestName string
	Cases    []goSpecCaseData
}

type goSpecCaseData struct {
	Name       string
	Setup      string
	Action     string
	Assertions []string
}

// GenerateFromSpecs generates Go test code from TestSpec slice
func (a *GoSpecAdapter) GenerateFromSpecs(specs []model.TestSpec, sourceFile string) (string, error) {
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

	data := goSpecTemplateData{
		Package: extractPackageName(sourceFile),
		Imports: []string{},
		Tests:   make([]goSpecTestData, 0),
	}

	// Track if we need strings import
	needsStrings := false
	needsReflect := false

	// Build tests grouped by function
	for funcName, funcSpecs := range specsByFunc {
		testData := goSpecTestData{
			TestName: toGoFunctionName(funcName),
			Cases:    make([]goSpecCaseData, 0),
		}

		for _, spec := range funcSpecs {
			caseData := goSpecCaseData{
				Name:       sanitizeTestName(spec.Description),
				Assertions: make([]string, 0),
			}

			// Generate setup from inputs
			if len(spec.Inputs) > 0 {
				caseData.Setup = a.generateSetup(spec.Inputs)
			}

			// Generate action (function call)
			caseData.Action = a.generateAction(spec)

			// Generate assertions from spec.Assertions
			for _, assertion := range spec.Assertions {
				assertCode, usesStrings, usesReflect := a.generateAssertion(assertion)
				if assertCode != "" {
					caseData.Assertions = append(caseData.Assertions, assertCode)
				}
				if usesStrings {
					needsStrings = true
				}
				if usesReflect {
					needsReflect = true
				}
			}

			// If no assertions were generated, add a placeholder
			if len(caseData.Assertions) == 0 {
				caseData.Assertions = append(caseData.Assertions, `// TODO: Add assertions`)
			}

			testData.Cases = append(testData.Cases, caseData)
		}

		data.Tests = append(data.Tests, testData)
	}

	// Add required imports
	if needsStrings {
		data.Imports = append(data.Imports, "strings")
	}
	if needsReflect {
		data.Imports = append(data.Imports, "reflect")
	}

	// Execute template
	tmpl, err := template.New("gospec").Parse(goSpecTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// generateSetup generates setup code from inputs
func (a *GoSpecAdapter) generateSetup(inputs map[string]interface{}) string {
	var setup strings.Builder

	// Separate named args from indexed args
	var namedKeys []string
	var indexedKeys []string
	for key := range inputs {
		if strings.HasPrefix(key, "arg") {
			indexedKeys = append(indexedKeys, key)
		} else {
			namedKeys = append(namedKeys, key)
		}
	}

	// If we have named args, use those; otherwise use indexed args
	var keys []string
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

	for _, key := range keys {
		setup.WriteString(fmt.Sprintf("%s := %s\n\t\t", key, formatGoValue(inputs[key])))
	}
	return strings.TrimSuffix(setup.String(), "\n\t\t")
}

// generateAction generates the function call
func (a *GoSpecAdapter) generateAction(spec model.TestSpec) string {
	funcName := spec.FunctionName
	if funcName == "" {
		funcName = spec.TargetID
	}

	// Build args from inputs, prioritizing named params (a, b) over arg0, arg1
	var namedArgs []string
	var indexedArgs []string

	for key := range spec.Inputs {
		if strings.HasPrefix(key, "arg") {
			indexedArgs = append(indexedArgs, key)
		} else {
			namedArgs = append(namedArgs, key)
		}
	}

	// Use named args if available (a, b), otherwise use indexed args (arg0, arg1)
	var args []string
	if len(namedArgs) > 0 {
		// Sort named args alphabetically for consistency
		sort.Strings(namedArgs)
		args = namedArgs
	} else if len(indexedArgs) > 0 {
		// Sort indexed args by number
		sort.Slice(indexedArgs, func(i, j int) bool {
			numI, _ := strconv.Atoi(strings.TrimPrefix(indexedArgs[i], "arg"))
			numJ, _ := strconv.Atoi(strings.TrimPrefix(indexedArgs[j], "arg"))
			return numI < numJ
		})
		args = indexedArgs
	}

	return fmt.Sprintf("result := %s(%s)", funcName, strings.Join(args, ", "))
}

// generateAssertion generates Go assertion code from model.Assertion
func (a *GoSpecAdapter) generateAssertion(assertion model.Assertion) (code string, usesStrings, usesReflect bool) {
	actual := assertion.Actual
	if actual == "" {
		actual = "result"
	}

	switch assertion.Kind {
	case "equality":
		expected := formatGoValue(assertion.Expected)
		return fmt.Sprintf(`if result != %s {
			t.Errorf("%s: expected %%v, got %%v", %s, result)
		}`, expected, actual, expected), false, false

	case "not_equal":
		expected := formatGoValue(assertion.Expected)
		return fmt.Sprintf(`if result == %s {
			t.Errorf("%s: expected not %%v, but got %%v", %s, result)
		}`, expected, actual, expected), false, false

	case "not_null":
		return fmt.Sprintf(`if result == nil {
			t.Error("%s: expected non-nil value, got nil")
		}`, actual), false, false

	case "null":
		return fmt.Sprintf(`if result != nil {
			t.Errorf("%s: expected nil, got %%v", result)
		}`, actual), false, false

	case "contains":
		expected := formatGoValue(assertion.Expected)
		// String contains check
		return fmt.Sprintf(`if !strings.Contains(fmt.Sprintf("%%v", result), %s) {
			t.Errorf("%s: expected to contain %%v, got %%v", %s, result)
		}`, expected, actual, expected), true, false

	case "greater_than":
		expected := formatGoValue(assertion.Expected)
		return fmt.Sprintf(`if result <= %s {
			t.Errorf("%s: expected > %%v, got %%v", %s, result)
		}`, expected, actual, expected), false, false

	case "less_than":
		expected := formatGoValue(assertion.Expected)
		return fmt.Sprintf(`if result >= %s {
			t.Errorf("%s: expected < %%v, got %%v", %s, result)
		}`, expected, actual, expected), false, false

	case "type":
		expected := assertion.Expected
		return fmt.Sprintf(`if reflect.TypeOf(result).String() != %q {
			t.Errorf("%s: expected type %%s, got %%s", %q, reflect.TypeOf(result).String())
		}`, expected, actual, expected), false, true

	default:
		// For unknown kinds, generate a generic equality check
		if assertion.Expected != nil {
			expected := formatGoValue(assertion.Expected)
			return fmt.Sprintf(`if result != %s {
			t.Errorf("%s: expected %%v, got %%v", %s, result)
		}`, expected, assertion.Kind, expected), false, false
		}
		return "", false, false
	}
}

// formatGoValue formats a value for Go code
func formatGoValue(val interface{}) string {
	if val == nil {
		return "nil"
	}

	switch v := val.(type) {
	case string:
		// Check if it's a variable reference
		if strings.HasPrefix(v, "${") || strings.HasPrefix(v, "$") {
			varName := strings.TrimPrefix(strings.TrimPrefix(v, "${"), "$")
			varName = strings.TrimSuffix(varName, "}")
			return varName
		}
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
		return fmt.Sprintf("%q", v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case []interface{}:
		elements := make([]string, len(v))
		for i, elem := range v {
			elements[i] = formatGoValue(elem)
		}
		return fmt.Sprintf("[]interface{}{%s}", strings.Join(elements, ", "))
	case map[string]interface{}:
		pairs := make([]string, 0, len(v))
		for key, value := range v {
			pairs = append(pairs, fmt.Sprintf("%q: %s", key, formatGoValue(value)))
		}
		return fmt.Sprintf("map[string]interface{}{%s}", strings.Join(pairs, ", "))
	default:
		return fmt.Sprintf("%v", v)
	}
}

// sanitizeTestName makes a description safe for use as a test name
func sanitizeTestName(name string) string {
	// Replace spaces and special characters
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

// extractPackageNameFromFile reads package name from source file
func extractPackageNameFromFile(filePath string) string {
	if content, err := os.ReadFile(filePath); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "package ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return parts[1]
				}
			}
		}
	}
	return "main"
}
