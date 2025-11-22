package adapters

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/QTest-hq/qtest/pkg/dsl"
)

// JestAdapter generates Jest test code
type JestAdapter struct{}

func NewJestAdapter() *JestAdapter {
	return &JestAdapter{}
}

func (a *JestAdapter) Framework() Framework {
	return FrameworkJest
}

func (a *JestAdapter) FileExtension() string {
	return ".ts"
}

func (a *JestAdapter) TestFileSuffix() string {
	return ".test"
}

const jestTestTemplate = `{{range .Imports}}
import {{.}}
{{end}}

describe('{{.DescribeName}}', () => {
{{range .BeforeEach}}
  beforeEach({{if .Async}}async {{end}}() => {
    {{.Code}}
  });
{{end}}
{{range .AfterEach}}
  afterEach({{if .Async}}async {{end}}() => {
    {{.Code}}
  });
{{end}}
{{range .Tests}}
  {{if .Async}}it{{else}}test{{end}}('{{.Name}}', {{if .Async}}async {{end}}() => {
{{range .Steps}}
    // {{.Description}}
    {{.Code}}
{{end}}
  });
{{end}}
});
`

type jestTemplateData struct {
	Imports      []string
	DescribeName string
	BeforeEach   []jestHook
	AfterEach    []jestHook
	Tests        []jestTestData
}

type jestHook struct {
	Async bool
	Code  string
}

type jestTestData struct {
	Name  string
	Async bool
	Steps []jestStep
}

type jestStep struct {
	Description string
	Code        string
}

func (a *JestAdapter) Generate(test *dsl.TestDSL) (string, error) {
	data := jestTemplateData{
		Imports:      make([]string, 0),
		DescribeName: test.Target.Function,
		BeforeEach:   make([]jestHook, 0),
		AfterEach:    make([]jestHook, 0),
		Tests:        make([]jestTestData, 0),
	}

	// Add default imports
	if test.Target.File != "" {
		modulePath := strings.TrimSuffix(test.Target.File, ".ts")
		modulePath = strings.TrimSuffix(modulePath, ".js")
		data.Imports = append(data.Imports, fmt.Sprintf("{ %s } from '%s'", test.Target.Function, modulePath))
	}

	// Process lifecycle
	if test.Lifecycle != nil {
		for _, action := range test.Lifecycle.BeforeEach {
			data.BeforeEach = append(data.BeforeEach, jestHook{
				Async: false,
				Code:  generateJestAction(action),
			})
		}
		for _, action := range test.Lifecycle.AfterEach {
			data.AfterEach = append(data.AfterEach, jestHook{
				Async: false,
				Code:  generateJestAction(action),
			})
		}
	}

	// Create test
	testData := jestTestData{
		Name:  test.Name,
		Async: hasAsyncSteps(test),
		Steps: make([]jestStep, 0),
	}

	for _, step := range test.Steps {
		code := generateJestStepCode(step)
		testData.Steps = append(testData.Steps, jestStep{
			Description: step.Description,
			Code:        code,
		})
	}

	data.Tests = append(data.Tests, testData)

	// Execute template
	tmpl, err := template.New("jest").Parse(jestTestTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func hasAsyncSteps(test *dsl.TestDSL) bool {
	for _, step := range test.Steps {
		if step.Action.Type == dsl.ActionHTTP || step.Action.Type == dsl.ActionWait {
			return true
		}
	}
	return false
}

func generateJestAction(action dsl.Action) string {
	switch action.Type {
	case "mock":
		if target, ok := action.Params["target"].(string); ok {
			return fmt.Sprintf("jest.mock('%s')", target)
		}
		return "// mock setup"
	case "db_setup":
		return "// database setup"
	default:
		return fmt.Sprintf("// %s", action.Type)
	}
}

func generateJestStepCode(step dsl.TestStep) string {
	var code strings.Builder

	// Generate action
	switch step.Action.Type {
	case dsl.ActionCall:
		args := formatJSArgs(step.Action.Args)
		if step.Expected != nil {
			code.WriteString(fmt.Sprintf("const result = %s(%s);\n", step.Action.Target, args))
		} else {
			code.WriteString(fmt.Sprintf("%s(%s);\n", step.Action.Target, args))
		}

	case dsl.ActionHTTP:
		method := step.Action.Method
		if method == "" {
			method = "GET"
		}
		code.WriteString(fmt.Sprintf("const response = await fetch('%s', { method: '%s' });\n",
			step.Action.Target, method))
		code.WriteString("const data = await response.json();\n")

	case dsl.ActionAssert:
		// Just assertions, no action needed

	default:
		code.WriteString(fmt.Sprintf("// %s: %s\n", step.Action.Type, step.Action.Target))
	}

	// Generate assertions
	if step.Expected != nil {
		code.WriteString(generateJestAssertions(step.Expected))
	}

	return strings.TrimSpace(code.String())
}

func formatJSArgs(args []interface{}) string {
	if len(args) == 0 {
		return ""
	}
	strs := make([]string, len(args))
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
			strs[i] = fmt.Sprintf("'%s'", v)
		default:
			strs[i] = fmt.Sprintf("%v", v)
		}
	}
	return strings.Join(strs, ", ")
}

func generateJestAssertions(expected *dsl.Expected) string {
	var assertions strings.Builder

	if expected.Value != nil {
		switch v := expected.Value.(type) {
		case string:
			assertions.WriteString(fmt.Sprintf("    expect(result).toBe('%s');\n", v))
		default:
			assertions.WriteString(fmt.Sprintf("    expect(result).toBe(%v);\n", v))
		}
	}

	if expected.Type != "" {
		assertions.WriteString(fmt.Sprintf("    expect(typeof result).toBe('%s');\n", expected.Type))
	}

	if expected.Contains != nil {
		switch v := expected.Contains.(type) {
		case string:
			assertions.WriteString(fmt.Sprintf("    expect(result).toContain('%s');\n", v))
		default:
			assertions.WriteString(fmt.Sprintf("    expect(result).toContain(%v);\n", v))
		}
	}

	if expected.Error != nil {
		assertions.WriteString("    expect(() => result).toThrow();\n")
	}

	return assertions.String()
}
