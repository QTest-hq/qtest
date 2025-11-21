package emitter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
)

// PytestEmitter generates pytest + httpx tests for Python APIs
type PytestEmitter struct{}

func (e *PytestEmitter) Name() string          { return "pytest" }
func (e *PytestEmitter) Language() string      { return "python" }
func (e *PytestEmitter) Framework() string     { return "pytest" }
func (e *PytestEmitter) FileExtension() string { return "_test.py" }

// Emit generates a complete test file from multiple specs
func (e *PytestEmitter) Emit(specs []model.TestSpec) (string, error) {
	var sb strings.Builder

	// File header
	sb.WriteString(`import pytest
import httpx
from fastapi.testclient import TestClient
from main import app

client = TestClient(app)


`)

	// Generate tests
	for _, spec := range specs {
		testCode, err := e.emitTest(spec)
		if err != nil {
			continue
		}
		sb.WriteString(testCode)
		sb.WriteString("\n\n")
	}

	return sb.String(), nil
}

// EmitSingle generates test code for a single spec
func (e *PytestEmitter) EmitSingle(spec model.TestSpec) (string, error) {
	return e.emitTest(spec)
}

func (e *PytestEmitter) emitTest(spec model.TestSpec) (string, error) {
	var sb strings.Builder

	testName := e.generateTestName(spec)
	sb.WriteString(fmt.Sprintf("def %s():\n", testName))

	// Add docstring
	if spec.Description != "" {
		sb.WriteString(fmt.Sprintf("    \"\"\"%s\"\"\"\n", spec.Description))
	}

	// Build the request
	path := e.resolvePath(spec)
	method := strings.ToLower(spec.Method)

	if spec.Body != nil && (spec.Method == "POST" || spec.Method == "PUT" || spec.Method == "PATCH") {
		bodyJSON, _ := json.MarshalIndent(spec.Body, "    ", "    ")
		sb.WriteString(fmt.Sprintf("    response = client.%s(\n", method))
		sb.WriteString(fmt.Sprintf("        \"%s\",\n", path))
		sb.WriteString(fmt.Sprintf("        json=%s,\n", string(bodyJSON)))

		if len(spec.Headers) > 0 {
			sb.WriteString("        headers={\n")
			for key, value := range spec.Headers {
				sb.WriteString(fmt.Sprintf("            \"%s\": \"%s\",\n", key, value))
			}
			sb.WriteString("        },\n")
		}
		sb.WriteString("    )\n\n")
	} else {
		if len(spec.Headers) > 0 {
			sb.WriteString(fmt.Sprintf("    response = client.%s(\n", method))
			sb.WriteString(fmt.Sprintf("        \"%s\",\n", path))
			sb.WriteString("        headers={\n")
			for key, value := range spec.Headers {
				sb.WriteString(fmt.Sprintf("            \"%s\": \"%s\",\n", key, value))
			}
			sb.WriteString("        },\n")
			sb.WriteString("    )\n\n")
		} else {
			sb.WriteString(fmt.Sprintf("    response = client.%s(\"%s\")\n\n", method, path))
		}
	}

	// Add assertions
	for _, assertion := range spec.Assertions {
		sb.WriteString(e.emitAssertion(assertion))
	}

	return sb.String(), nil
}

func (e *PytestEmitter) emitAssertion(a model.Assertion) string {
	switch a.Kind {
	case "status_code":
		return fmt.Sprintf("    assert response.status_code == %v\n", a.Expected)

	case "equality":
		path := e.parseBodyPath(a.Actual)
		expectedJSON, _ := json.Marshal(a.Expected)
		return fmt.Sprintf("    assert %s == %s\n", path, string(expectedJSON))

	case "contains":
		path := e.parseBodyPath(a.Actual)
		expectedJSON, _ := json.Marshal(a.Expected)
		return fmt.Sprintf("    assert %s in %s\n", string(expectedJSON), path)

	case "not_null":
		path := e.parseBodyPath(a.Actual)
		return fmt.Sprintf("    assert %s is not None\n", path)

	default:
		return fmt.Sprintf("    # Unknown assertion kind: %s\n", a.Kind)
	}
}

func (e *PytestEmitter) parseBodyPath(actual string) string {
	if actual == "status" {
		return "response.status_code"
	}
	if strings.HasPrefix(actual, "body.") {
		// Convert body.field to response.json()["field"]
		field := strings.TrimPrefix(actual, "body.")
		parts := strings.Split(field, ".")
		result := "response.json()"
		for _, p := range parts {
			result += fmt.Sprintf("[\"%s\"]", p)
		}
		return result
	}
	if actual == "body" {
		return "response.json()"
	}
	return actual
}

func (e *PytestEmitter) resolvePath(spec model.TestSpec) string {
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

func (e *PytestEmitter) generateTestName(spec model.TestSpec) string {
	// Convert path to valid Python function name
	path := strings.ReplaceAll(spec.Path, "/", "_")
	path = strings.ReplaceAll(path, ":", "")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	path = strings.TrimPrefix(path, "_")
	path = strings.ToLower(path)

	return fmt.Sprintf("test_%s_%s", strings.ToLower(spec.Method), path)
}
