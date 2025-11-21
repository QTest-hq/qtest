package emitter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
)

// GoHTTPEmitter generates Go net/http tests
type GoHTTPEmitter struct{}

func (e *GoHTTPEmitter) Name() string         { return "go-http" }
func (e *GoHTTPEmitter) Language() string     { return "go" }
func (e *GoHTTPEmitter) Framework() string    { return "testing" }
func (e *GoHTTPEmitter) FileExtension() string { return "_test.go" }

// Emit generates a complete test file from multiple specs
func (e *GoHTTPEmitter) Emit(specs []model.TestSpec) (string, error) {
	var sb strings.Builder

	// Package declaration
	sb.WriteString("package main\n\n")

	// Imports
	sb.WriteString(`import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

`)

	// Generate tests
	for _, spec := range specs {
		testCode, err := e.emitTest(spec)
		if err != nil {
			continue
		}
		sb.WriteString(testCode)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// EmitSingle generates test code for a single spec
func (e *GoHTTPEmitter) EmitSingle(spec model.TestSpec) (string, error) {
	return e.emitTest(spec)
}

func (e *GoHTTPEmitter) emitTest(spec model.TestSpec) (string, error) {
	var sb strings.Builder

	testName := e.generateTestName(spec)
	sb.WriteString(fmt.Sprintf("func %s(t *testing.T) {\n", testName))

	// Create test server (assumes handler is available)
	sb.WriteString("\t// Create test server\n")
	sb.WriteString("\tts := httptest.NewServer(http.DefaultServeMux)\n")
	sb.WriteString("\tdefer ts.Close()\n\n")

	// Build request
	path := e.resolvePath(spec)

	if spec.Body != nil && (spec.Method == "POST" || spec.Method == "PUT" || spec.Method == "PATCH") {
		bodyJSON, _ := json.Marshal(spec.Body)
		sb.WriteString(fmt.Sprintf("\tbody := strings.NewReader(`%s`)\n", string(bodyJSON)))
		sb.WriteString(fmt.Sprintf("\treq, err := http.NewRequest(%q, ts.URL+%q, body)\n", spec.Method, path))
	} else {
		sb.WriteString(fmt.Sprintf("\treq, err := http.NewRequest(%q, ts.URL+%q, nil)\n", spec.Method, path))
	}

	sb.WriteString("\tif err != nil {\n")
	sb.WriteString("\t\tt.Fatalf(\"failed to create request: %v\", err)\n")
	sb.WriteString("\t}\n\n")

	// Add headers
	if len(spec.Headers) > 0 {
		for key, value := range spec.Headers {
			sb.WriteString(fmt.Sprintf("\treq.Header.Set(%q, %q)\n", key, value))
		}
		sb.WriteString("\n")
	}

	// Send request
	sb.WriteString("\tresp, err := http.DefaultClient.Do(req)\n")
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString("\t\tt.Fatalf(\"request failed: %v\", err)\n")
	sb.WriteString("\t}\n")
	sb.WriteString("\tdefer resp.Body.Close()\n\n")

	// Read body
	sb.WriteString("\tbodyBytes, _ := io.ReadAll(resp.Body)\n")
	sb.WriteString("\t_ = bodyBytes // Use for body assertions\n\n")

	// Add assertions
	for _, assertion := range spec.Assertions {
		sb.WriteString(e.emitAssertion(assertion))
	}

	sb.WriteString("}\n")

	return sb.String(), nil
}

func (e *GoHTTPEmitter) emitAssertion(a model.Assertion) string {
	switch a.Kind {
	case "status_code":
		return fmt.Sprintf("\tif resp.StatusCode != %v {\n\t\tt.Errorf(\"expected status %v, got %%d\", resp.StatusCode)\n\t}\n", a.Expected, a.Expected)

	case "equality":
		if a.Actual == "body" || strings.HasPrefix(a.Actual, "body.") {
			expectedJSON, _ := json.Marshal(a.Expected)
			return fmt.Sprintf("\t// Check body contains expected value\n\tif !strings.Contains(string(bodyBytes), `%s`) {\n\t\tt.Errorf(\"body does not contain expected value\")\n\t}\n", string(expectedJSON))
		}
		return fmt.Sprintf("\t// TODO: Assert %s equals %v\n", a.Actual, a.Expected)

	case "not_null":
		return fmt.Sprintf("\t// TODO: Assert %s is not null\n", a.Actual)

	default:
		return fmt.Sprintf("\t// Unknown assertion kind: %s\n", a.Kind)
	}
}

func (e *GoHTTPEmitter) resolvePath(spec model.TestSpec) string {
	path := spec.Path

	// Replace path parameters
	for key, value := range spec.PathParams {
		placeholder := ":" + key
		path = strings.Replace(path, placeholder, fmt.Sprintf("%v", value), 1)
		placeholder = "{" + key + "}"
		path = strings.Replace(path, placeholder, fmt.Sprintf("%v", value), 1)
	}

	// Add query parameters
	if len(spec.QueryParams) > 0 {
		params := make([]string, 0)
		for key, value := range spec.QueryParams {
			params = append(params, fmt.Sprintf("%s=%v", key, value))
		}
		path += "?" + strings.Join(params, "&")
	}

	return path
}

func (e *GoHTTPEmitter) generateTestName(spec model.TestSpec) string {
	// Convert path to valid Go function name
	path := strings.ReplaceAll(spec.Path, "/", "_")
	path = strings.ReplaceAll(path, ":", "")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	path = strings.TrimPrefix(path, "_")

	return fmt.Sprintf("Test_%s_%s", spec.Method, path)
}
