package emitter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
)

// JUnitEmitter generates JUnit 5 tests for Java/Spring Boot
type JUnitEmitter struct{}

func (e *JUnitEmitter) Name() string          { return "junit" }
func (e *JUnitEmitter) Language() string      { return "java" }
func (e *JUnitEmitter) Framework() string     { return "junit5" }
func (e *JUnitEmitter) FileExtension() string { return "Test.java" }

// Emit generates a complete test file from multiple specs
func (e *JUnitEmitter) Emit(specs []model.TestSpec) (string, error) {
	var sb strings.Builder

	// Package declaration (default)
	sb.WriteString("package com.example.tests;\n\n")

	// Imports
	sb.WriteString(`import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;
import static org.junit.jupiter.api.Assertions.*;

`)

	// Class declaration
	sb.WriteString("@SpringBootTest\n")
	sb.WriteString("@AutoConfigureMockMvc\n")
	sb.WriteString("public class ApiTest {\n\n")

	// MockMvc injection
	sb.WriteString("    @Autowired\n")
	sb.WriteString("    private MockMvc mockMvc;\n\n")

	// Generate tests
	for _, spec := range specs {
		testCode, err := e.emitTest(spec)
		if err != nil {
			continue
		}
		sb.WriteString(testCode)
		sb.WriteString("\n")
	}

	// Close class
	sb.WriteString("}\n")

	return sb.String(), nil
}

// EmitSingle generates test code for a single spec
func (e *JUnitEmitter) EmitSingle(spec model.TestSpec) (string, error) {
	return e.emitTest(spec)
}

func (e *JUnitEmitter) emitTest(spec model.TestSpec) (string, error) {
	var sb strings.Builder

	testName := e.generateTestName(spec)
	displayName := e.generateDisplayName(spec)

	sb.WriteString(fmt.Sprintf("    @Test\n"))
	sb.WriteString(fmt.Sprintf("    @DisplayName(\"%s\")\n", displayName))
	sb.WriteString(fmt.Sprintf("    void %s() throws Exception {\n", testName))

	// Build the request path
	path := e.resolvePath(spec)

	// Build MockMvc request
	method := strings.ToLower(spec.Method)
	sb.WriteString(fmt.Sprintf("        MvcResult result = mockMvc.perform(%s(\"%s\")\n", method, path))

	// Add content type for body requests
	if spec.Body != nil && (spec.Method == "POST" || spec.Method == "PUT" || spec.Method == "PATCH") {
		bodyJSON, _ := json.Marshal(spec.Body)
		sb.WriteString("                .contentType(MediaType.APPLICATION_JSON)\n")
		sb.WriteString(fmt.Sprintf("                .content(\"%s\")\n", e.escapeJavaString(string(bodyJSON))))
	}

	// Add headers
	for key, value := range spec.Headers {
		sb.WriteString(fmt.Sprintf("                .header(\"%s\", \"%s\")\n", key, value))
	}

	sb.WriteString("        )\n")

	// Add assertions
	for _, assertion := range spec.Assertions {
		sb.WriteString(e.emitAssertion(assertion))
	}

	// Default status assertion if none specified
	hasStatusAssertion := false
	for _, a := range spec.Assertions {
		if a.Kind == "status_code" {
			hasStatusAssertion = true
			break
		}
	}
	if !hasStatusAssertion {
		sb.WriteString("                .andExpect(status().isOk())\n")
	}

	sb.WriteString("                .andReturn();\n\n")

	// Get response body for further assertions
	sb.WriteString("        String responseBody = result.getResponse().getContentAsString();\n")
	sb.WriteString("        assertNotNull(responseBody);\n")

	sb.WriteString("    }\n")

	return sb.String(), nil
}

func (e *JUnitEmitter) emitAssertion(a model.Assertion) string {
	switch a.Kind {
	case "status_code":
		status := fmt.Sprintf("%v", a.Expected)
		switch status {
		case "200":
			return "                .andExpect(status().isOk())\n"
		case "201":
			return "                .andExpect(status().isCreated())\n"
		case "204":
			return "                .andExpect(status().isNoContent())\n"
		case "400":
			return "                .andExpect(status().isBadRequest())\n"
		case "401":
			return "                .andExpect(status().isUnauthorized())\n"
		case "403":
			return "                .andExpect(status().isForbidden())\n"
		case "404":
			return "                .andExpect(status().isNotFound())\n"
		case "500":
			return "                .andExpect(status().isInternalServerError())\n"
		default:
			return fmt.Sprintf("                .andExpect(status().is(%s))\n", status)
		}

	case "equality":
		if strings.HasPrefix(a.Actual, "body.") {
			jsonPath := "$." + strings.TrimPrefix(a.Actual, "body.")
			expectedJSON, _ := json.Marshal(a.Expected)
			return fmt.Sprintf("                .andExpect(jsonPath(\"%s\").value(%s))\n", jsonPath, string(expectedJSON))
		}
		return fmt.Sprintf("        // TODO: Assert %s equals %v\n", a.Actual, a.Expected)

	case "not_null":
		if strings.HasPrefix(a.Actual, "body.") {
			jsonPath := "$." + strings.TrimPrefix(a.Actual, "body.")
			return fmt.Sprintf("                .andExpect(jsonPath(\"%s\").exists())\n", jsonPath)
		}
		return fmt.Sprintf("        // TODO: Assert %s is not null\n", a.Actual)

	case "contains":
		return fmt.Sprintf("                .andExpect(content().string(org.hamcrest.Matchers.containsString(\"%v\")))\n", a.Expected)

	default:
		return fmt.Sprintf("        // Unknown assertion kind: %s\n", a.Kind)
	}
}

func (e *JUnitEmitter) resolvePath(spec model.TestSpec) string {
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

func (e *JUnitEmitter) generateTestName(spec model.TestSpec) string {
	// Convert path to valid Java method name
	path := strings.ReplaceAll(spec.Path, "/", "_")
	path = strings.ReplaceAll(path, ":", "")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	path = strings.ReplaceAll(path, "-", "_")
	path = strings.TrimPrefix(path, "_")
	path = strings.TrimSuffix(path, "_")

	if path == "" {
		path = "root"
	}

	return fmt.Sprintf("test%s_%s", strings.Title(strings.ToLower(spec.Method)), path)
}

func (e *JUnitEmitter) generateDisplayName(spec model.TestSpec) string {
	return fmt.Sprintf("%s %s", spec.Method, spec.Path)
}

func (e *JUnitEmitter) escapeJavaString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
