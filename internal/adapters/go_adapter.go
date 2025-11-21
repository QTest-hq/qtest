package adapters

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/QTest-hq/qtest/pkg/dsl"
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
		// Check if it's a variable reference like ${a}, $a, or *a
		if strings.HasPrefix(v, "${") || strings.HasPrefix(v, "$") || strings.HasPrefix(v, "*") {
			// This is a variable reference that wasn't resolved - use a default
			return "0"
		}
		// Check if it's a number stored as string
		if _, err := fmt.Sscanf(v, "%d", new(int)); err == nil {
			return v
		}
		if _, err := fmt.Sscanf(v, "%f", new(float64)); err == nil {
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
	default:
		return fmt.Sprintf("%v", v)
	}
}
