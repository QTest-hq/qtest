package adapters

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/QTest-hq/qtest/pkg/dsl"
	"github.com/QTest-hq/qtest/pkg/model"
)

// GoAdapter generates Go test code
type GoAdapter struct{}

func NewGoAdapter() *GoAdapter {
	return &GoAdapter{}
}

func (a *GoAdapter) Framework() Framework {
	return FrameworkGoTest
}

func (a *GoAdapter) FileExtension() string {
	return ".go"
}

func (a *GoAdapter) TestFileSuffix() string {
	return "_test"
}

const goTestTemplate = `package {{.Package}}

import (
	"testing"
{{range .Imports}}
	"{{.}}"
{{end}}
)

{{range .Tests}}
func Test{{.FunctionName}}(t *testing.T) {
	{{if .Setup}}// Setup
	{{.Setup}}
	{{end}}
	var result interface{}
	_ = result
	{{range .Steps}}
	// {{.Description}}
	{{if .Action}}result = {{.Action}}{{end}}
	{{range .Assertions}}
	{{.}}
	{{end}}
	{{end}}
}
{{end}}
`

type goTemplateData struct {
	Package string
	Imports []string
	Tests   []goTestData
}

type goTestData struct {
	FunctionName string
	Setup        string
	Steps        []goStepData
}

type goStepData struct {
	Description string
	Action      string
	Assertions  []string
}

func (a *GoAdapter) Generate(test *dsl.TestDSL) (string, error) {
	// Build template data
	data := goTemplateData{
		Package: extractPackageName(test.Target.File),
		Imports: []string{},
		Tests:   make([]goTestData, 0),
	}

	// Convert test name to function name
	funcName := toGoFunctionName(test.Name)
	if funcName == "" {
		funcName = toGoFunctionName(test.Target.Function)
	}

	testData := goTestData{
		FunctionName: funcName,
		Steps:        make([]goStepData, 0),
	}

	// Process lifecycle setup
	if test.Lifecycle != nil {
		for _, action := range test.Lifecycle.BeforeEach {
			testData.Setup += generateGoAction(action) + "\n\t"
		}
	}

	// Process steps
	for _, step := range test.Steps {
		stepData := goStepData{
			Description: step.Description,
			Assertions:  make([]string, 0),
		}

		// Generate action
		if step.Action.Type != "" {
			stepData.Action = generateGoStepAction(step)
		}

		// Generate assertions
		if step.Expected != nil {
			assertions := generateGoAssertions(step)
			stepData.Assertions = append(stepData.Assertions, assertions...)
		}

		testData.Steps = append(testData.Steps, stepData)
	}

	data.Tests = append(data.Tests, testData)

	// Execute template
	tmpl, err := template.New("gotest").Parse(goTestTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func extractPackageName(filePath string) string {
	// Try to read the actual package name from the source file
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

	// Fallback: extract from directory name
	parts := strings.Split(filePath, "/")
	if len(parts) > 0 {
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "" && !strings.Contains(parts[i], ".") {
				return parts[i]
			}
		}
	}
	return "main"
}

func toGoFunctionName(name string) string {
	if name == "" {
		return ""
	}

	// Remove special characters and convert to PascalCase
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

func generateGoAction(action dsl.Action) string {
	switch action.Type {
	case "db_setup":
		return "// TODO: Setup database"
	case "mock":
		return "// TODO: Setup mocks"
	default:
		return fmt.Sprintf("// %s", action.Type)
	}
}

func generateGoStepAction(step dsl.TestStep) string {
	switch step.Action.Type {
	case dsl.ActionCall:
		args := ""
		if len(step.Action.Args) > 0 {
			argStrs := make([]string, len(step.Action.Args))
			for i, arg := range step.Action.Args {
				// Format argument based on type
				argStrs[i] = formatGoArg(arg)
			}
			args = strings.Join(argStrs, ", ")
		}
		return fmt.Sprintf("%s(%s)", step.Action.Target, args)

	case dsl.ActionHTTP:
		method := step.Action.Method
		if method == "" {
			method = "GET"
		}
		return fmt.Sprintf(`req := httptest.NewRequest("%s", "%s", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)`, method, step.Action.Target)

	default:
		return fmt.Sprintf("// %s: %s", step.Action.Type, step.Action.Target)
	}
}

func generateGoAssertions(step dsl.TestStep) []string {
	assertions := make([]string, 0)

	if step.Expected == nil {
		return assertions
	}

	if step.Expected.Value != nil {
		assertions = append(assertions, fmt.Sprintf(
			`if result != %v {
		t.Errorf("expected %%v, got %%v", %v, result)
	}`, step.Expected.Value, step.Expected.Value))
	}

	if step.Expected.Error != nil {
		assertions = append(assertions, fmt.Sprintf(
			`if err == nil {
		t.Error("expected error, got nil")
	}`))
	}

	if step.Expected.Contains != nil {
		assertions = append(assertions, fmt.Sprintf(
			`if !strings.Contains(result, %q) {
		t.Errorf("expected result to contain %%q", %q)
	}`, step.Expected.Contains, step.Expected.Contains))
	}

	return assertions
}

// formatGoArg formats an argument for Go code, handling various types
func formatGoArg(arg interface{}) string {
	if arg == nil {
		return "nil"
	}

	switch v := arg.(type) {
	case string:
		// Check if it's a variable reference that wasn't resolved
		if isUnresolvedVariable(v) {
			return getDefaultForVariable(v)
		}
		// Check if it's a number stored as string
		if _, err := fmt.Sscanf(v, "%d", new(int)); err == nil {
			return v
		}
		if _, err := fmt.Sscanf(v, "%f", new(float64)); err == nil {
			return v
		}
		// Check for boolean strings
		if v == "true" || v == "false" {
			return v
		}
		// It's a string literal
		return fmt.Sprintf("%q", v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case []interface{}:
		// Handle slice arguments
		elements := make([]string, len(v))
		for i, elem := range v {
			elements[i] = formatGoArg(elem)
		}
		return fmt.Sprintf("[]interface{}{%s}", strings.Join(elements, ", "))
	case map[string]interface{}:
		// Handle map arguments
		return "map[string]interface{}{}" // Empty map as safe default
	default:
		return fmt.Sprintf("%v", v)
	}
}

// isUnresolvedVariable checks if a string looks like an unresolved variable reference
func isUnresolvedVariable(s string) bool {
	return strings.HasPrefix(s, "${") ||
		strings.HasPrefix(s, "$") ||
		strings.HasPrefix(s, "*") ||
		strings.HasPrefix(s, "&")
}

// getDefaultForVariable returns a type-appropriate default for an unresolved variable
func getDefaultForVariable(v string) string {
	// Extract variable name for type hints
	varName := strings.ToLower(v)

	// Remove variable markers
	varName = strings.TrimPrefix(varName, "${")
	varName = strings.TrimSuffix(varName, "}")
	varName = strings.TrimPrefix(varName, "$")
	varName = strings.TrimPrefix(varName, "*")
	varName = strings.TrimPrefix(varName, "&")

	// Pointer patterns - return nil
	if strings.HasPrefix(v, "*") || strings.HasPrefix(v, "&") {
		return "nil"
	}

	// String-like variable names
	stringHints := []string{"str", "string", "name", "text", "msg", "message", "path", "url", "key", "value", "id", "err"}
	for _, hint := range stringHints {
		if strings.Contains(varName, hint) {
			return `""`
		}
	}

	// Boolean-like variable names
	boolHints := []string{"is", "has", "can", "should", "enable", "disable", "flag", "bool", "ok", "valid"}
	for _, hint := range boolHints {
		if strings.HasPrefix(varName, hint) || strings.Contains(varName, hint) {
			return "false"
		}
	}

	// Slice/array-like variable names
	sliceHints := []string{"list", "array", "slice", "items", "elements", "values"}
	for _, hint := range sliceHints {
		if strings.Contains(varName, hint) {
			return "nil"
		}
	}

	// Default to 0 for numeric-looking or unknown variables
	return "0"
}

// Template for IRSpec-based test generation (cleaner, table-driven)
const goTestSpecTemplate = `package {{.Package}}

import (
	"testing"
)

func Test{{.FunctionName}}(t *testing.T) {
{{range .SubTests}}
	t.Run("{{.Name}}", func(t *testing.T) {
		// Given
{{range .Setup}}		{{.}}
{{end}}
		// When
		result := {{.Action}}

		// Then
{{range .Assertions}}		{{.}}
{{end}}	})

{{end}}}
`

type goTestSpecData struct {
	Package      string
	FunctionName string
	SubTests     []goSubTestData
}

type goSubTestData struct {
	Name       string
	Setup      []string
	Action     string
	Assertions []string
}

// GenerateFromTestSpecs generates Go test code from IRSpec TestSpecs
// This produces cleaner, table-driven style tests with proper subtests
func (a *GoAdapter) GenerateFromTestSpecs(specs []model.TestSpec, targetFile string) (string, error) {
	if len(specs) == 0 {
		return "", fmt.Errorf("no test specs provided")
	}

	// Get function name from first spec
	funcName := specs[0].FunctionName
	if funcName == "" {
		funcName = "Unknown"
	}

	data := goTestSpecData{
		Package:      extractPackageName(targetFile),
		FunctionName: toGoFunctionName(funcName),
		SubTests:     make([]goSubTestData, 0, len(specs)),
	}

	for _, spec := range specs {
		subTest := goSubTestData{
			Name:       formatSubTestName(spec.Description),
			Setup:      make([]string, 0),
			Assertions: make([]string, 0),
		}

		// Generate setup from inputs
		args := make([]string, 0)
		for name, value := range spec.Inputs {
			varDecl := fmt.Sprintf("%s := %s", name, formatGoArg(value))
			subTest.Setup = append(subTest.Setup, varDecl)
			args = append(args, name)
		}

		// Generate action (function call)
		subTest.Action = fmt.Sprintf("%s(%s)", funcName, strings.Join(args, ", "))

		// Generate assertions from spec.Assertions
		for _, assertion := range spec.Assertions {
			assertCode := generateGoAssertionFromSpec(assertion)
			if assertCode != "" {
				subTest.Assertions = append(subTest.Assertions, assertCode)
			}
		}

		// Fallback if no assertions from spec but have Inputs
		if len(subTest.Assertions) == 0 && spec.Expected != nil {
			if expected, ok := spec.Expected["result"]; ok {
				subTest.Assertions = append(subTest.Assertions,
					fmt.Sprintf("if result != %v {\n\t\t\tt.Errorf(\"expected %%v, got %%v\", %v, result)\n\t\t}", expected, expected))
			}
		}

		data.SubTests = append(data.SubTests, subTest)
	}

	// Execute template
	tmpl, err := template.New("gotest-spec").Parse(goTestSpecTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// formatSubTestName formats a description into a valid Go sub-test name
func formatSubTestName(desc string) string {
	if desc == "" {
		return "test_case"
	}
	// Replace spaces with underscores, remove special chars
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		if r == ' ' || r == '-' {
			return '_'
		}
		return -1
	}, desc)
	return name
}

// generateGoAssertionFromSpec converts a model.Assertion to Go assertion code
func generateGoAssertionFromSpec(a model.Assertion) string {
	actual := a.Actual
	if actual == "" || actual == "result" {
		actual = "result"
	}

	// Format expected value properly for Go code
	expected := formatGoArg(a.Expected)

	switch a.Kind {
	case "equality", "equals":
		return fmt.Sprintf("if %s != %s {\n\t\t\tt.Errorf(\"expected %%v, got %%v\", %s, %s)\n\t\t}", actual, expected, expected, actual)

	case "not_equal", "not_equals":
		return fmt.Sprintf("if %s == %s {\n\t\t\tt.Errorf(\"expected not %%v\", %s)\n\t\t}", actual, expected, actual)

	case "greater_than":
		return fmt.Sprintf("if %s <= %s {\n\t\t\tt.Errorf(\"expected > %%v, got %%v\", %s, %s)\n\t\t}", actual, expected, expected, actual)

	case "less_than":
		return fmt.Sprintf("if %s >= %s {\n\t\t\tt.Errorf(\"expected < %%v, got %%v\", %s, %s)\n\t\t}", actual, expected, expected, actual)

	case "contains":
		return fmt.Sprintf("if !strings.Contains(fmt.Sprintf(\"%%v\", %s), %s) {\n\t\t\tt.Errorf(\"expected to contain %%v\", %s)\n\t\t}", actual, expected, expected)

	case "nil", "is_nil":
		return fmt.Sprintf("if %s != nil {\n\t\t\tt.Errorf(\"expected nil, got %%v\", %s)\n\t\t}", actual, actual)

	case "not_nil", "is_not_nil":
		return fmt.Sprintf("if %s == nil {\n\t\t\tt.Error(\"expected non-nil value\")\n\t\t}", actual)

	case "truthy":
		return fmt.Sprintf("if !%s {\n\t\t\tt.Error(\"expected truthy value\")\n\t\t}", actual)

	case "falsy":
		return fmt.Sprintf("if %s {\n\t\t\tt.Error(\"expected falsy value\")\n\t\t}", actual)

	case "throws", "error":
		return "if err == nil {\n\t\t\tt.Error(\"expected error, got nil\")\n\t\t}"

	default:
		if a.Expected != nil {
			return fmt.Sprintf("if %s != %s {\n\t\t\tt.Errorf(\"expected %%v, got %%v\", %s, %s)\n\t\t}", actual, expected, expected, actual)
		}
		return ""
	}
}
