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
		return generateJestMock(action)
	case "http_mock":
		return generateJestHTTPMock(action)
	case "db_setup":
		return "// database setup"
	default:
		return fmt.Sprintf("// %s", action.Type)
	}
}

// generateJestMock generates Jest mock setup code
func generateJestMock(action dsl.Action) string {
	target, _ := action.Params["target"].(string)
	mockType, _ := action.Params["type"].(string)
	returnVal, hasReturn := action.Params["return"]
	method, hasMethod := action.Params["method"].(string)

	switch mockType {
	case "module":
		// Mock an entire module
		if target != "" {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("jest.mock('%s'", target))
			if hasReturn {
				buf.WriteString(fmt.Sprintf(", () => ({ default: %v }))", formatJSVal(returnVal)))
			} else {
				buf.WriteString(")")
			}
			return buf.String()
		}
		return "jest.mock('module-name')"

	case "function":
		// Mock a function with jest.fn()
		if target != "" {
			if hasReturn {
				return fmt.Sprintf("const %s = jest.fn().mockReturnValue(%v)", target, formatJSVal(returnVal))
			}
			return fmt.Sprintf("const %s = jest.fn()", target)
		}
		return "const mockFn = jest.fn()"

	case "spy":
		// Spy on an object method
		if target != "" && hasMethod {
			if hasReturn {
				return fmt.Sprintf("jest.spyOn(%s, '%s').mockReturnValue(%v)", target, method, formatJSVal(returnVal))
			}
			return fmt.Sprintf("jest.spyOn(%s, '%s')", target, method)
		}
		return "jest.spyOn(object, 'method')"

	case "async":
		// Mock async function
		if target != "" {
			if hasReturn {
				return fmt.Sprintf("const %s = jest.fn().mockResolvedValue(%v)", target, formatJSVal(returnVal))
			}
			return fmt.Sprintf("const %s = jest.fn().mockResolvedValue(undefined)", target)
		}
		return "const mockAsync = jest.fn().mockResolvedValue(undefined)"

	case "implementation":
		// Mock with custom implementation
		impl, _ := action.Params["implementation"].(string)
		if target != "" && impl != "" {
			return fmt.Sprintf("const %s = jest.fn().mockImplementation(%s)", target, impl)
		}
		return "const mockFn = jest.fn().mockImplementation(() => {})"

	case "timer":
		// Mock timers
		return "jest.useFakeTimers()"

	default:
		// Default module mock
		if target != "" {
			return fmt.Sprintf("jest.mock('%s')", target)
		}
		return "// mock setup - jest.mock('module') or jest.fn()"
	}
}

// generateJestHTTPMock generates HTTP mock code for Jest (using nock or msw patterns)
func generateJestHTTPMock(action dsl.Action) string {
	method, _ := action.Params["method"].(string)
	if method == "" {
		method = "GET"
	}
	path, _ := action.Params["path"].(string)
	if path == "" {
		path = "/"
	}
	baseURL, _ := action.Params["baseUrl"].(string)
	if baseURL == "" {
		baseURL = "http://localhost"
	}
	statusCode, ok := action.Params["status"].(int)
	if !ok {
		statusCode = 200
	}
	responseBody, _ := action.Params["response"]

	// Generate fetch mock
	return fmt.Sprintf(`global.fetch = jest.fn().mockImplementation((url, options) => {
      if (url.includes('%s') && (!options?.method || options.method === '%s')) {
        return Promise.resolve({
          ok: %v,
          status: %d,
          json: () => Promise.resolve(%v)
        });
      }
      return Promise.reject(new Error('Not found'));
    })`, path, method, statusCode >= 200 && statusCode < 300, statusCode, formatJSVal(responseBody))
}

// formatJSVal formats a value for JavaScript code (used by mock generation)
// Note: formatJSValue exists in jest_spec_adapter.go with more comprehensive handling
func formatJSVal(val interface{}) string {
	if val == nil {
		return "null"
	}
	switch v := val.(type) {
	case string:
		return fmt.Sprintf("'%s'", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case map[string]interface{}:
		// Simple object literal
		parts := make([]string, 0, len(v))
		for k, vv := range v {
			parts = append(parts, fmt.Sprintf("%s: %s", k, formatJSVal(vv)))
		}
		return fmt.Sprintf("{ %s }", strings.Join(parts, ", "))
	case []interface{}:
		parts := make([]string, len(v))
		for i, vv := range v {
			parts[i] = formatJSVal(vv)
		}
		return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
	default:
		return fmt.Sprintf("%v", v)
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
