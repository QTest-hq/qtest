package adapters

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/QTest-hq/qtest/pkg/dsl"
)

// PytestAdapter generates Pytest code
type PytestAdapter struct{}

func NewPytestAdapter() *PytestAdapter {
	return &PytestAdapter{}
}

func (a *PytestAdapter) Framework() Framework {
	return FrameworkPytest
}

func (a *PytestAdapter) FileExtension() string {
	return ".py"
}

func (a *PytestAdapter) TestFileSuffix() string {
	return "_test"
}

const pytestTemplate = `{{range .Imports}}
{{.}}
{{end}}
import pytest
{{if .HasFixtures}}

{{range .Fixtures}}
@pytest.fixture
def {{.Name}}():
    {{.Setup}}
    yield {{.YieldValue}}
    {{.Teardown}}
{{end}}
{{end}}
{{range .Tests}}

{{if .Markers}}{{range .Markers}}@pytest.mark.{{.}}
{{end}}{{end}}{{if .Async}}@pytest.mark.asyncio
async {{end}}def test_{{.FunctionName}}({{.FixtureArgs}}):
    """{{.Docstring}}"""
{{range .Steps}}
    # {{.Description}}
{{.Code}}
{{end}}
{{end}}
`

type pytestTemplateData struct {
	Imports     []string
	HasFixtures bool
	Fixtures    []pytestFixture
	Tests       []pytestTestData
}

type pytestFixture struct {
	Name       string
	Setup      string
	YieldValue string
	Teardown   string
}

type pytestTestData struct {
	FunctionName string
	Docstring    string
	Markers      []string
	Async        bool
	FixtureArgs  string
	Steps        []pytestStep
}

type pytestStep struct {
	Description string
	Code        string
}

func (a *PytestAdapter) Generate(test *dsl.TestDSL) (string, error) {
	data := pytestTemplateData{
		Imports:     make([]string, 0),
		HasFixtures: false,
		Fixtures:    make([]pytestFixture, 0),
		Tests:       make([]pytestTestData, 0),
	}

	// Add imports based on target
	if test.Target.File != "" {
		moduleName := strings.TrimSuffix(test.Target.File, ".py")
		moduleName = strings.ReplaceAll(moduleName, "/", ".")
		data.Imports = append(data.Imports,
			fmt.Sprintf("from %s import %s", moduleName, test.Target.Function))
	}

	// Process resources as fixtures
	fixtures := make([]string, 0)
	for _, resource := range test.Resources {
		fixture := resourceToFixture(resource)
		data.Fixtures = append(data.Fixtures, fixture)
		data.HasFixtures = true
		fixtures = append(fixtures, fixture.Name)
	}

	// Create test
	funcName := toPythonFunctionName(test.Name)
	if funcName == "" {
		funcName = toPythonFunctionName(test.Target.Function)
	}

	testData := pytestTestData{
		FunctionName: funcName,
		Docstring:    test.Description,
		Markers:      make([]string, 0),
		Async:        hasAsyncPythonSteps(test),
		FixtureArgs:  strings.Join(fixtures, ", "),
		Steps:        make([]pytestStep, 0),
	}

	// Add markers based on test type
	switch test.Type {
	case dsl.TestTypeIntegration:
		testData.Markers = append(testData.Markers, "integration")
	case dsl.TestTypeE2E:
		testData.Markers = append(testData.Markers, "e2e")
	}

	// Process steps
	for _, step := range test.Steps {
		code := generatePytestStepCode(step)
		testData.Steps = append(testData.Steps, pytestStep{
			Description: step.Description,
			Code:        code,
		})
	}

	data.Tests = append(data.Tests, testData)

	// Execute template
	tmpl, err := template.New("pytest").Parse(pytestTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func toPythonFunctionName(name string) string {
	if name == "" {
		return ""
	}

	// Convert to snake_case
	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == ' ' || r == '-' || r == '.'
	})

	for i := range words {
		words[i] = strings.ToLower(words[i])
	}

	return strings.Join(words, "_")
}

func hasAsyncPythonSteps(test *dsl.TestDSL) bool {
	for _, step := range test.Steps {
		if step.Action.Type == dsl.ActionHTTP || step.Action.Type == dsl.ActionWait {
			return true
		}
	}
	return false
}

func resourceToFixture(resource dsl.Resource) pytestFixture {
	fixture := pytestFixture{
		Name:       strings.ToLower(string(resource.Type)) + "_" + resource.Name,
		YieldValue: resource.Name,
	}

	switch resource.Type {
	case dsl.ResourceDatabase:
		fixture.Setup = "# Setup database connection"
		fixture.Teardown = "# Cleanup database"
	case dsl.ResourceCache:
		fixture.Setup = "# Setup cache"
		fixture.Teardown = "# Clear cache"
	default:
		fixture.Setup = fmt.Sprintf("# Setup %s", resource.Type)
		fixture.Teardown = fmt.Sprintf("# Teardown %s", resource.Type)
	}

	return fixture
}

func generatePytestStepCode(step dsl.TestStep) string {
	var code strings.Builder

	// Generate action
	switch step.Action.Type {
	case dsl.ActionCall:
		args := formatPythonArgs(step.Action.Args)
		if step.Expected != nil {
			code.WriteString(fmt.Sprintf("    result = %s(%s)\n", step.Action.Target, args))
		} else {
			code.WriteString(fmt.Sprintf("    %s(%s)\n", step.Action.Target, args))
		}

	case dsl.ActionHTTP:
		method := strings.ToLower(step.Action.Method)
		if method == "" {
			method = "get"
		}
		code.WriteString(fmt.Sprintf("    response = await client.%s('%s')\n",
			method, step.Action.Target))

	case dsl.ActionAssert:
		// Just assertions

	default:
		code.WriteString(fmt.Sprintf("    # %s: %s\n", step.Action.Type, step.Action.Target))
	}

	// Generate assertions
	if step.Expected != nil {
		code.WriteString(generatePytestAssertions(step.Expected))
	}

	return code.String()
}

func formatPythonArgs(args []interface{}) string {
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

func generatePytestAssertions(expected *dsl.Expected) string {
	var assertions strings.Builder

	if expected.Value != nil {
		switch v := expected.Value.(type) {
		case string:
			assertions.WriteString(fmt.Sprintf("    assert result == '%s'\n", v))
		default:
			assertions.WriteString(fmt.Sprintf("    assert result == %v\n", v))
		}
	}

	if expected.Type != "" {
		assertions.WriteString(fmt.Sprintf("    assert isinstance(result, %s)\n", expected.Type))
	}

	if expected.Contains != nil {
		switch v := expected.Contains.(type) {
		case string:
			assertions.WriteString(fmt.Sprintf("    assert '%s' in result\n", v))
		default:
			assertions.WriteString(fmt.Sprintf("    assert %v in result\n", v))
		}
	}

	if expected.Error != nil {
		if expected.Error.Type != "" {
			assertions.WriteString(fmt.Sprintf("    with pytest.raises(%s):\n", expected.Error.Type))
		} else {
			assertions.WriteString("    with pytest.raises(Exception):\n")
		}
	}

	return assertions.String()
}
