package emitter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
)

// RSpecEmitter generates RSpec tests for Ruby/Rails APIs
type RSpecEmitter struct{}

func (e *RSpecEmitter) Name() string         { return "rspec" }
func (e *RSpecEmitter) Language() string     { return "ruby" }
func (e *RSpecEmitter) Framework() string    { return "rspec" }
func (e *RSpecEmitter) FileExtension() string { return "_spec.rb" }

// Emit generates a complete test file from multiple specs
func (e *RSpecEmitter) Emit(specs []model.TestSpec) (string, error) {
	var sb strings.Builder

	// File header
	sb.WriteString(`# frozen_string_literal: true

require 'rails_helper'

`)

	// Group specs by path prefix for describe blocks
	groups := e.groupByPath(specs)

	for groupName, groupSpecs := range groups {
		sb.WriteString(fmt.Sprintf("RSpec.describe '%s', type: :request do\n", groupName))

		for _, spec := range groupSpecs {
			testCode, err := e.emitTest(spec)
			if err != nil {
				continue
			}
			sb.WriteString(testCode)
			sb.WriteString("\n")
		}

		sb.WriteString("end\n\n")
	}

	return sb.String(), nil
}

// EmitSingle generates test code for a single spec
func (e *RSpecEmitter) EmitSingle(spec model.TestSpec) (string, error) {
	return e.emitTest(spec)
}

func (e *RSpecEmitter) emitTest(spec model.TestSpec) (string, error) {
	var sb strings.Builder

	// Describe block for the endpoint
	sb.WriteString(fmt.Sprintf("  describe '%s %s' do\n", spec.Method, spec.Path))

	// Test case
	testName := e.generateTestName(spec)
	sb.WriteString(fmt.Sprintf("    it '%s' do\n", testName))

	// Build headers hash
	if len(spec.Headers) > 0 {
		sb.WriteString("      headers = {\n")
		for key, value := range spec.Headers {
			sb.WriteString(fmt.Sprintf("        '%s' => '%s',\n", key, value))
		}
		sb.WriteString("      }\n\n")
	}

	// Build request body
	var bodyVar string
	if spec.Body != nil && (spec.Method == "POST" || spec.Method == "PUT" || spec.Method == "PATCH") {
		bodyJSON, _ := json.MarshalIndent(spec.Body, "      ", "  ")
		sb.WriteString(fmt.Sprintf("      body = %s\n\n", e.jsonToRubyHash(string(bodyJSON))))
		bodyVar = ", params: body.to_json, headers: headers.merge('Content-Type' => 'application/json')"
	} else if len(spec.Headers) > 0 {
		bodyVar = ", headers: headers"
	}

	// Make the request
	method := strings.ToLower(spec.Method)
	path := e.resolvePath(spec)
	sb.WriteString(fmt.Sprintf("      %s '%s'%s\n\n", method, path, bodyVar))

	// Add assertions
	for _, assertion := range spec.Assertions {
		sb.WriteString(e.emitAssertion(assertion))
	}

	sb.WriteString("    end\n")
	sb.WriteString("  end\n")

	return sb.String(), nil
}

func (e *RSpecEmitter) emitAssertion(a model.Assertion) string {
	switch a.Kind {
	case "status_code":
		return fmt.Sprintf("      expect(response).to have_http_status(%v)\n", a.Expected)

	case "equality":
		path := e.parseBodyPath(a.Actual)
		expected := e.formatRubyValue(a.Expected)
		return fmt.Sprintf("      expect(%s).to eq(%s)\n", path, expected)

	case "contains":
		path := e.parseBodyPath(a.Actual)
		expected := e.formatRubyValue(a.Expected)
		return fmt.Sprintf("      expect(%s).to include(%s)\n", path, expected)

	case "not_null":
		path := e.parseBodyPath(a.Actual)
		return fmt.Sprintf("      expect(%s).not_to be_nil\n", path)

	case "type":
		path := e.parseBodyPath(a.Actual)
		rubyType := e.goTypeToRubyClass(fmt.Sprintf("%v", a.Expected))
		return fmt.Sprintf("      expect(%s).to be_a(%s)\n", path, rubyType)

	default:
		return fmt.Sprintf("      # Unknown assertion kind: %s\n", a.Kind)
	}
}

func (e *RSpecEmitter) parseBodyPath(actual string) string {
	if actual == "status" {
		return "response.status"
	}
	if actual == "body" {
		return "JSON.parse(response.body)"
	}
	if strings.HasPrefix(actual, "body.") {
		path := strings.TrimPrefix(actual, "body.")
		parts := strings.Split(path, ".")
		accessors := make([]string, len(parts))
		for i, part := range parts {
			accessors[i] = fmt.Sprintf("['%s']", part)
		}
		return "JSON.parse(response.body)" + strings.Join(accessors, "")
	}
	return actual
}

func (e *RSpecEmitter) resolvePath(spec model.TestSpec) string {
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

func (e *RSpecEmitter) generateTestName(spec model.TestSpec) string {
	if spec.Description != "" {
		return spec.Description
	}
	return "returns expected response"
}

func (e *RSpecEmitter) groupByPath(specs []model.TestSpec) map[string][]model.TestSpec {
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

func (e *RSpecEmitter) formatRubyValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", val)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case nil:
		return "nil"
	case []interface{}:
		items := make([]string, len(val))
		for i, item := range val {
			items[i] = e.formatRubyValue(item)
		}
		return "[" + strings.Join(items, ", ") + "]"
	case map[string]interface{}:
		pairs := make([]string, 0)
		for k, v := range val {
			pairs = append(pairs, fmt.Sprintf("'%s' => %s", k, e.formatRubyValue(v)))
		}
		return "{ " + strings.Join(pairs, ", ") + " }"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func (e *RSpecEmitter) jsonToRubyHash(jsonStr string) string {
	// Simple conversion of JSON to Ruby hash syntax
	result := strings.ReplaceAll(jsonStr, "\":", "\" =>")
	result = strings.ReplaceAll(result, "null", "nil")
	result = strings.ReplaceAll(result, "true", "true")
	result = strings.ReplaceAll(result, "false", "false")
	return result
}

func (e *RSpecEmitter) goTypeToRubyClass(t string) string {
	switch t {
	case "string":
		return "String"
	case "int", "integer", "number":
		return "Integer"
	case "float":
		return "Float"
	case "bool", "boolean":
		return "TrueClass, FalseClass"
	case "array":
		return "Array"
	case "object", "hash", "map":
		return "Hash"
	default:
		return "Object"
	}
}
