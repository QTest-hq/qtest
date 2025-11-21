package emitter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
)

// SupertestEmitter generates Jest + Supertest tests for Express APIs
type SupertestEmitter struct{}

func (e *SupertestEmitter) Name() string          { return "supertest" }
func (e *SupertestEmitter) Language() string      { return "javascript" }
func (e *SupertestEmitter) Framework() string     { return "jest" }
func (e *SupertestEmitter) FileExtension() string { return ".test.js" }

// Emit generates a complete test file from multiple specs
func (e *SupertestEmitter) Emit(specs []model.TestSpec) (string, error) {
	var sb strings.Builder

	// File header
	sb.WriteString(`const request = require('supertest');
const app = require('./app');

`)

	// Group specs by path prefix for describe blocks
	groups := e.groupByPath(specs)

	for groupName, groupSpecs := range groups {
		sb.WriteString(fmt.Sprintf("describe('%s', () => {\n", groupName))

		for _, spec := range groupSpecs {
			testCode, err := e.emitTest(spec)
			if err != nil {
				continue
			}
			sb.WriteString(testCode)
			sb.WriteString("\n")
		}

		sb.WriteString("});\n\n")
	}

	return sb.String(), nil
}

// EmitSingle generates test code for a single spec
func (e *SupertestEmitter) EmitSingle(spec model.TestSpec) (string, error) {
	return e.emitTest(spec)
}

func (e *SupertestEmitter) emitTest(spec model.TestSpec) (string, error) {
	var sb strings.Builder

	// Test function
	testName := e.generateTestName(spec)
	sb.WriteString(fmt.Sprintf("  test('%s', async () => {\n", testName))

	// Build the request
	sb.WriteString("    const response = await request(app)\n")
	sb.WriteString(fmt.Sprintf("      .%s('%s')\n", strings.ToLower(spec.Method), e.resolvePath(spec)))

	// Add headers
	if len(spec.Headers) > 0 {
		for key, value := range spec.Headers {
			sb.WriteString(fmt.Sprintf("      .set('%s', '%s')\n", key, value))
		}
	}

	// Add request body for POST/PUT/PATCH
	if spec.Body != nil && (spec.Method == "POST" || spec.Method == "PUT" || spec.Method == "PATCH") {
		bodyJSON, _ := json.Marshal(spec.Body)
		sb.WriteString(fmt.Sprintf("      .send(%s)\n", string(bodyJSON)))
	}

	sb.WriteString(";\n\n")

	// Add assertions
	for _, assertion := range spec.Assertions {
		sb.WriteString(e.emitAssertion(assertion))
	}

	sb.WriteString("  });\n")

	return sb.String(), nil
}

func (e *SupertestEmitter) emitAssertion(a model.Assertion) string {
	switch a.Kind {
	case "status_code":
		return fmt.Sprintf("    expect(response.status).toBe(%v);\n", a.Expected)

	case "equality":
		path := e.parseBodyPath(a.Actual)
		expectedJSON, _ := json.Marshal(a.Expected)
		return fmt.Sprintf("    expect(%s).toEqual(%s);\n", path, string(expectedJSON))

	case "contains":
		path := e.parseBodyPath(a.Actual)
		expectedJSON, _ := json.Marshal(a.Expected)
		return fmt.Sprintf("    expect(%s).toContain(%s);\n", path, string(expectedJSON))

	case "not_null":
		path := e.parseBodyPath(a.Actual)
		return fmt.Sprintf("    expect(%s).toBeDefined();\n", path)

	default:
		return fmt.Sprintf("    // Unknown assertion kind: %s\n", a.Kind)
	}
}

func (e *SupertestEmitter) parseBodyPath(actual string) string {
	if actual == "status" {
		return "response.status"
	}
	if strings.HasPrefix(actual, "body.") {
		return "response.body." + strings.TrimPrefix(actual, "body.")
	}
	if actual == "body" {
		return "response.body"
	}
	return "response." + actual
}

func (e *SupertestEmitter) resolvePath(spec model.TestSpec) string {
	path := spec.Path

	// Replace path parameters
	for key, value := range spec.PathParams {
		placeholder := ":" + key
		path = strings.Replace(path, placeholder, fmt.Sprintf("%v", value), 1)

		// Also handle {key} style
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

func (e *SupertestEmitter) generateTestName(spec model.TestSpec) string {
	if spec.Description != "" {
		return spec.Description
	}
	return fmt.Sprintf("%s %s returns expected response", spec.Method, spec.Path)
}

func (e *SupertestEmitter) groupByPath(specs []model.TestSpec) map[string][]model.TestSpec {
	groups := make(map[string][]model.TestSpec)

	for _, spec := range specs {
		// Extract first path segment as group name
		parts := strings.Split(strings.TrimPrefix(spec.Path, "/"), "/")
		groupName := "API"
		if len(parts) > 0 && parts[0] != "" {
			groupName = "/" + parts[0]
		}

		groups[groupName] = append(groups[groupName], spec)
	}

	return groups
}
