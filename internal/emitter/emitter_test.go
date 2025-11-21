package emitter

import (
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
)

// Test Registry
func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}

	// Check all emitters are registered
	emitters := r.List()
	expected := []string{"supertest", "go-http", "pytest", "junit", "rspec", "playwright", "cypress"}

	if len(emitters) != len(expected) {
		t.Errorf("expected %d emitters, got %d", len(expected), len(emitters))
	}

	for _, name := range expected {
		if _, err := r.Get(name); err != nil {
			t.Errorf("emitter %s not found: %v", name, err)
		}
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	// Test existing emitter
	e, err := r.Get("supertest")
	if err != nil {
		t.Errorf("failed to get supertest emitter: %v", err)
	}
	if e.Name() != "supertest" {
		t.Errorf("expected supertest, got %s", e.Name())
	}

	// Test non-existing emitter
	_, err = r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent emitter")
	}
}

func TestRegistry_GetForLanguage(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		language     string
		wantLanguage string
		wantErr      bool
	}{
		{"javascript", "javascript", false},
		{"go", "go", false},
		{"python", "python", false},
		{"java", "java", false},
		{"ruby", "ruby", false},
		{"typescript", "typescript", false},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		e, err := r.GetForLanguage(tt.language)
		if tt.wantErr {
			if err == nil {
				t.Errorf("GetForLanguage(%s) expected error", tt.language)
			}
			continue
		}
		if err != nil {
			t.Errorf("GetForLanguage(%s) error: %v", tt.language, err)
			continue
		}
		// Just verify we got an emitter for the correct language
		if e.Language() != tt.wantLanguage {
			t.Errorf("GetForLanguage(%s) returned emitter for %s, want %s", tt.language, e.Language(), tt.wantLanguage)
		}
	}
}

// Helper to create a test spec
func createAPITestSpec(method, path, description string) model.TestSpec {
	return model.TestSpec{
		ID:          "test-1",
		Level:       model.LevelAPI,
		TargetKind:  "endpoint",
		Description: description,
		Method:      method,
		Path:        path,
		Headers:     map[string]string{"Content-Type": "application/json"},
		Assertions: []model.Assertion{
			{Kind: "status_code", Expected: 200},
			{Kind: "not_null", Actual: "body"},
		},
	}
}

func createE2ETestSpec(path, description string) model.TestSpec {
	return model.TestSpec{
		ID:          "e2e-1",
		Level:       model.LevelE2E,
		TargetKind:  "e2e",
		Description: description,
		Path:        path,
		Inputs: map[string]interface{}{
			"url":   "https://example.com",
			"click": "@submit-button",
			"fill": map[string]interface{}{
				"#username": "testuser",
				"#password": "testpass",
			},
		},
		Assertions: []model.Assertion{
			{Kind: "visible", Actual: "@success-message"},
			{Kind: "text", Actual: "title", Expected: "Dashboard"},
		},
	}
}

// Supertest Emitter Tests
func TestSupertestEmitter_Metadata(t *testing.T) {
	e := &SupertestEmitter{}

	if e.Name() != "supertest" {
		t.Errorf("Name() = %s, want supertest", e.Name())
	}
	if e.Language() != "javascript" {
		t.Errorf("Language() = %s, want javascript", e.Language())
	}
	if e.Framework() != "jest" {
		t.Errorf("Framework() = %s, want jest", e.Framework())
	}
	if e.FileExtension() != ".test.js" {
		t.Errorf("FileExtension() = %s, want .test.js", e.FileExtension())
	}
}

func TestSupertestEmitter_Emit(t *testing.T) {
	e := &SupertestEmitter{}
	specs := []model.TestSpec{
		createAPITestSpec("GET", "/users", "should get users"),
		createAPITestSpec("POST", "/users", "should create user"),
	}

	code, err := e.Emit(specs)
	if err != nil {
		t.Fatalf("Emit() error: %v", err)
	}

	// Check for expected content
	expectations := []string{
		"const request = require('supertest')",
		"describe(",
		"test('should get users'",
		"test('should create user'",
		".get('/users')",
		".post('/users')",
		"expect(response.status).toBe(200)",
	}

	for _, exp := range expectations {
		if !strings.Contains(code, exp) {
			t.Errorf("Emit() missing expected content: %s", exp)
		}
	}
}

func TestSupertestEmitter_EmitSingle(t *testing.T) {
	e := &SupertestEmitter{}
	spec := createAPITestSpec("GET", "/users/:id", "should get user by id")
	spec.PathParams = map[string]interface{}{"id": 123}

	code, err := e.EmitSingle(spec)
	if err != nil {
		t.Fatalf("EmitSingle() error: %v", err)
	}

	if !strings.Contains(code, ".get('/users/123')") {
		t.Error("EmitSingle() should resolve path params")
	}
}

// Go-HTTP Emitter Tests
func TestGoHTTPEmitter_Metadata(t *testing.T) {
	e := &GoHTTPEmitter{}

	if e.Name() != "go-http" {
		t.Errorf("Name() = %s, want go-http", e.Name())
	}
	if e.Language() != "go" {
		t.Errorf("Language() = %s, want go", e.Language())
	}
	if e.FileExtension() != "_test.go" {
		t.Errorf("FileExtension() = %s, want _test.go", e.FileExtension())
	}
}

func TestGoHTTPEmitter_Emit(t *testing.T) {
	e := &GoHTTPEmitter{}
	specs := []model.TestSpec{
		createAPITestSpec("GET", "/api/health", "should check health"),
	}

	code, err := e.Emit(specs)
	if err != nil {
		t.Fatalf("Emit() error: %v", err)
	}

	expectations := []string{
		"package",
		"import",
		"net/http",
		"testing",
		"func Test",
		"http.NewRequest",
	}

	for _, exp := range expectations {
		if !strings.Contains(code, exp) {
			t.Errorf("Emit() missing expected content: %s", exp)
		}
	}
}

// Pytest Emitter Tests
func TestPytestEmitter_Metadata(t *testing.T) {
	e := &PytestEmitter{}

	if e.Name() != "pytest" {
		t.Errorf("Name() = %s, want pytest", e.Name())
	}
	if e.Language() != "python" {
		t.Errorf("Language() = %s, want python", e.Language())
	}
	if e.FileExtension() != "_test.py" {
		t.Errorf("FileExtension() = %s, want _test.py", e.FileExtension())
	}
}

func TestPytestEmitter_Emit(t *testing.T) {
	e := &PytestEmitter{}
	specs := []model.TestSpec{
		createAPITestSpec("GET", "/items", "should list items"),
	}

	code, err := e.Emit(specs)
	if err != nil {
		t.Fatalf("Emit() error: %v", err)
	}

	expectations := []string{
		"import pytest",
		"import httpx",
		"TestClient",
		"def test_",
		"client.get",
		"assert response.status_code == 200",
	}

	for _, exp := range expectations {
		if !strings.Contains(code, exp) {
			t.Errorf("Emit() missing expected content: %s", exp)
		}
	}
}

// JUnit Emitter Tests
func TestJUnitEmitter_Metadata(t *testing.T) {
	e := &JUnitEmitter{}

	if e.Name() != "junit" {
		t.Errorf("Name() = %s, want junit", e.Name())
	}
	if e.Language() != "java" {
		t.Errorf("Language() = %s, want java", e.Language())
	}
	if e.FileExtension() != "Test.java" {
		t.Errorf("FileExtension() = %s, want Test.java", e.FileExtension())
	}
}

func TestJUnitEmitter_Emit(t *testing.T) {
	e := &JUnitEmitter{}
	specs := []model.TestSpec{
		createAPITestSpec("GET", "/api/products", "should get products"),
	}

	code, err := e.Emit(specs)
	if err != nil {
		t.Fatalf("Emit() error: %v", err)
	}

	expectations := []string{
		"import org.junit.jupiter.api",
		"import org.springframework",
		"@SpringBootTest",
		"@AutoConfigureMockMvc",
		"@Test",
		"mockMvc.perform",
		".andExpect(status().isOk())",
	}

	for _, exp := range expectations {
		if !strings.Contains(code, exp) {
			t.Errorf("Emit() missing expected content: %s", exp)
		}
	}
}

// RSpec Emitter Tests
func TestRSpecEmitter_Metadata(t *testing.T) {
	e := &RSpecEmitter{}

	if e.Name() != "rspec" {
		t.Errorf("Name() = %s, want rspec", e.Name())
	}
	if e.Language() != "ruby" {
		t.Errorf("Language() = %s, want ruby", e.Language())
	}
	if e.FileExtension() != "_spec.rb" {
		t.Errorf("FileExtension() = %s, want _spec.rb", e.FileExtension())
	}
}

func TestRSpecEmitter_Emit(t *testing.T) {
	e := &RSpecEmitter{}
	specs := []model.TestSpec{
		createAPITestSpec("GET", "/posts", "returns posts"),
	}

	code, err := e.Emit(specs)
	if err != nil {
		t.Fatalf("Emit() error: %v", err)
	}

	expectations := []string{
		"require 'rails_helper'",
		"RSpec.describe",
		"type: :request",
		"describe 'GET /posts'",
		"it 'returns posts'",
		"get '/posts'",
		"expect(response).to have_http_status(200)",
	}

	for _, exp := range expectations {
		if !strings.Contains(code, exp) {
			t.Errorf("Emit() missing expected content: %s", exp)
		}
	}
}

// Playwright Emitter Tests
func TestPlaywrightEmitter_Metadata(t *testing.T) {
	e := &PlaywrightEmitter{}

	if e.Name() != "playwright" {
		t.Errorf("Name() = %s, want playwright", e.Name())
	}
	if e.Language() != "typescript" {
		t.Errorf("Language() = %s, want typescript", e.Language())
	}
	if e.Framework() != "playwright" {
		t.Errorf("Framework() = %s, want playwright", e.Framework())
	}
	if e.FileExtension() != ".spec.ts" {
		t.Errorf("FileExtension() = %s, want .spec.ts", e.FileExtension())
	}
}

func TestPlaywrightEmitter_Emit(t *testing.T) {
	e := &PlaywrightEmitter{}
	specs := []model.TestSpec{
		createE2ETestSpec("/login", "should login successfully"),
	}

	code, err := e.Emit(specs)
	if err != nil {
		t.Fatalf("Emit() error: %v", err)
	}

	expectations := []string{
		"import { test, expect } from '@playwright/test'",
		"test.describe(",
		"test('should login successfully'",
		"async ({ page })",
		"await page.goto",
		"await page.click",
		"await page.fill",
		"await expect(",
		"toBeVisible()",
		"toHaveTitle(",
	}

	for _, exp := range expectations {
		if !strings.Contains(code, exp) {
			t.Errorf("Emit() missing expected content: %s", exp)
		}
	}
}

func TestPlaywrightEmitter_FormatSelector(t *testing.T) {
	e := &PlaywrightEmitter{}

	tests := []struct {
		input    string
		expected string
	}{
		{"@submit", `[data-testid="submit"]`},
		{"text:Submit", "text=Submit"},
		{"role:button", "role=button"},
		{".btn-primary", ".btn-primary"},
		{"#username", "#username"},
		{"[data-cy=test]", "[data-cy=test]"},
	}

	for _, tt := range tests {
		result := e.formatSelector(tt.input)
		if result != tt.expected {
			t.Errorf("formatSelector(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

// Cypress Emitter Tests
func TestCypressEmitter_Metadata(t *testing.T) {
	e := &CypressEmitter{}

	if e.Name() != "cypress" {
		t.Errorf("Name() = %s, want cypress", e.Name())
	}
	if e.Language() != "javascript" {
		t.Errorf("Language() = %s, want javascript", e.Language())
	}
	if e.Framework() != "cypress" {
		t.Errorf("Framework() = %s, want cypress", e.Framework())
	}
	if e.FileExtension() != ".cy.js" {
		t.Errorf("FileExtension() = %s, want .cy.js", e.FileExtension())
	}
}

func TestCypressEmitter_Emit(t *testing.T) {
	e := &CypressEmitter{}
	specs := []model.TestSpec{
		createE2ETestSpec("/signup", "should signup successfully"),
	}

	code, err := e.Emit(specs)
	if err != nil {
		t.Fatalf("Emit() error: %v", err)
	}

	expectations := []string{
		"/// <reference types=\"cypress\" />",
		"describe(",
		"it('should signup successfully'",
		"cy.visit(",
		"cy.get(",
		".click()",
		".clear().type(",
		"should('be.visible')",
	}

	for _, exp := range expectations {
		if !strings.Contains(code, exp) {
			t.Errorf("Emit() missing expected content: %s", exp)
		}
	}
}

func TestCypressEmitter_FormatSelector(t *testing.T) {
	e := &CypressEmitter{}

	tests := []struct {
		input    string
		expected string
	}{
		{"@submit", `[data-testid="submit"]`},
		{"$cypress-test", `[data-cy="cypress-test"]`},
		{".btn-primary", ".btn-primary"},
		{"#username", "#username"},
	}

	for _, tt := range tests {
		result := e.formatSelector(tt.input)
		if result != tt.expected {
			t.Errorf("formatSelector(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestCypressEmitter_WithIntercept(t *testing.T) {
	e := &CypressEmitter{}
	spec := model.TestSpec{
		ID:          "e2e-intercept",
		Level:       model.LevelE2E,
		Description: "should mock API",
		Inputs: map[string]interface{}{
			"url": "/dashboard",
			"intercept": []interface{}{
				map[string]interface{}{
					"method":   "GET",
					"url":      "/api/users",
					"alias":    "getUsers",
					"response": map[string]interface{}{"users": []string{"alice", "bob"}},
				},
			},
		},
	}

	code, err := e.EmitSingle(spec)
	if err != nil {
		t.Fatalf("EmitSingle() error: %v", err)
	}

	if !strings.Contains(code, "cy.intercept('GET', '/api/users'") {
		t.Error("EmitSingle() should include cy.intercept")
	}
	if !strings.Contains(code, ".as('getUsers')") {
		t.Error("EmitSingle() should include alias")
	}
}

// Test assertions for all emitters
func TestEmitter_Assertions(t *testing.T) {
	specs := []model.TestSpec{{
		ID:          "assertion-test",
		Level:       model.LevelAPI,
		Description: "test assertions",
		Method:      "GET",
		Path:        "/test",
		Assertions: []model.Assertion{
			{Kind: "status_code", Expected: 200},
			{Kind: "equality", Actual: "body.name", Expected: "test"},
			{Kind: "contains", Actual: "body.tags", Expected: "important"},
			{Kind: "not_null", Actual: "body.id"},
		},
	}}

	emitters := []Emitter{
		&SupertestEmitter{},
		&PytestEmitter{},
		&GoHTTPEmitter{},
		&JUnitEmitter{},
		&RSpecEmitter{},
	}

	for _, e := range emitters {
		code, err := e.Emit(specs)
		if err != nil {
			t.Errorf("%s.Emit() error: %v", e.Name(), err)
			continue
		}
		if code == "" {
			t.Errorf("%s.Emit() returned empty code", e.Name())
		}
	}
}

// Test E2E assertions
func TestE2EEmitter_Assertions(t *testing.T) {
	specs := []model.TestSpec{{
		ID:          "e2e-assertion-test",
		Level:       model.LevelE2E,
		Description: "test E2E assertions",
		Path:        "/page",
		Assertions: []model.Assertion{
			{Kind: "visible", Actual: "#element"},
			{Kind: "hidden", Actual: ".loading"},
			{Kind: "text", Actual: "h1", Expected: "Welcome"},
			{Kind: "contains", Actual: ".content", Expected: "hello"},
			{Kind: "value", Actual: "#input", Expected: "test"},
			{Kind: "enabled", Actual: "button"},
			{Kind: "disabled", Actual: ".disabled-btn"},
			{Kind: "exists", Actual: ".exists"},
		},
	}}

	emitters := []Emitter{
		&PlaywrightEmitter{},
		&CypressEmitter{},
	}

	for _, e := range emitters {
		code, err := e.Emit(specs)
		if err != nil {
			t.Errorf("%s.Emit() error: %v", e.Name(), err)
			continue
		}
		if code == "" {
			t.Errorf("%s.Emit() returned empty code", e.Name())
		}
	}
}

// Test empty specs
func TestEmitters_EmptySpecs(t *testing.T) {
	emitters := []Emitter{
		&SupertestEmitter{},
		&PytestEmitter{},
		&GoHTTPEmitter{},
		&JUnitEmitter{},
		&RSpecEmitter{},
		&PlaywrightEmitter{},
		&CypressEmitter{},
	}

	for _, e := range emitters {
		code, err := e.Emit([]model.TestSpec{})
		if err != nil {
			t.Errorf("%s.Emit([]) error: %v", e.Name(), err)
		}
		// Empty specs should produce minimal output (headers only)
		if code == "" {
			t.Logf("%s.Emit([]) returned empty string (acceptable)", e.Name())
		}
	}
}

// =============================================================================
// GoHTTPEmitter Helper Function Tests
// =============================================================================

func TestGoHTTPEmitter_ResolvePath(t *testing.T) {
	e := &GoHTTPEmitter{}

	tests := []struct {
		name     string
		spec     model.TestSpec
		expected string
	}{
		{
			name:     "simple path",
			spec:     model.TestSpec{Path: "/api/users"},
			expected: "/api/users",
		},
		{
			name: "path with colon param",
			spec: model.TestSpec{
				Path:       "/api/users/:id",
				PathParams: map[string]interface{}{"id": 123},
			},
			expected: "/api/users/123",
		},
		{
			name: "path with brace param",
			spec: model.TestSpec{
				Path:       "/api/users/{id}",
				PathParams: map[string]interface{}{"id": 456},
			},
			expected: "/api/users/456",
		},
		{
			name: "path with query params",
			spec: model.TestSpec{
				Path:        "/api/users",
				QueryParams: map[string]interface{}{"page": 1, "limit": 10},
			},
			expected: "/api/users?", // Check prefix only due to map ordering
		},
		{
			name: "path with both path and query params",
			spec: model.TestSpec{
				Path:        "/api/users/:id",
				PathParams:  map[string]interface{}{"id": 999},
				QueryParams: map[string]interface{}{"detail": "true"},
			},
			expected: "/api/users/999?detail=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.resolvePath(tt.spec)
			if tt.name == "path with query params" {
				// Check prefix due to map ordering
				if !strings.HasPrefix(result, tt.expected) {
					t.Errorf("resolvePath() = %s, want prefix %s", result, tt.expected)
				}
			} else if result != tt.expected {
				t.Errorf("resolvePath() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestGoHTTPEmitter_GenerateTestName(t *testing.T) {
	e := &GoHTTPEmitter{}

	tests := []struct {
		name     string
		spec     model.TestSpec
		expected string
	}{
		{
			name:     "simple GET",
			spec:     model.TestSpec{Method: "GET", Path: "/users"},
			expected: "Test_GET_users",
		},
		{
			name:     "POST with path params",
			spec:     model.TestSpec{Method: "POST", Path: "/users/:id"},
			expected: "Test_POST_users_id",
		},
		{
			name:     "DELETE with braces",
			spec:     model.TestSpec{Method: "DELETE", Path: "/users/{id}"},
			expected: "Test_DELETE_users_id",
		},
		{
			name:     "root path",
			spec:     model.TestSpec{Method: "GET", Path: "/"},
			expected: "Test_GET_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.generateTestName(tt.spec)
			if result != tt.expected {
				t.Errorf("generateTestName() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestGoHTTPEmitter_EmitAssertion(t *testing.T) {
	e := &GoHTTPEmitter{}

	tests := []struct {
		name      string
		assertion model.Assertion
		contains  string
	}{
		{
			name:      "status code 200",
			assertion: model.Assertion{Kind: "status_code", Expected: 200},
			contains:  "resp.StatusCode != 200",
		},
		{
			name:      "status code 404",
			assertion: model.Assertion{Kind: "status_code", Expected: 404},
			contains:  "resp.StatusCode != 404",
		},
		{
			name:      "equality body",
			assertion: model.Assertion{Kind: "equality", Actual: "body", Expected: "test"},
			contains:  "bodyBytes",
		},
		{
			name:      "equality body field",
			assertion: model.Assertion{Kind: "equality", Actual: "body.name", Expected: "John"},
			contains:  "bodyBytes",
		},
		{
			name:      "not_null",
			assertion: model.Assertion{Kind: "not_null", Actual: "body"},
			contains:  "TODO",
		},
		{
			name:      "unknown kind",
			assertion: model.Assertion{Kind: "custom_assertion"},
			contains:  "Unknown assertion kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.emitAssertion(tt.assertion)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("emitAssertion() = %s, want to contain %s", result, tt.contains)
			}
		})
	}
}

// =============================================================================
// JUnitEmitter Helper Function Tests
// =============================================================================

func TestJUnitEmitter_ResolvePath(t *testing.T) {
	e := &JUnitEmitter{}

	tests := []struct {
		name     string
		spec     model.TestSpec
		expected string
	}{
		{
			name:     "simple path",
			spec:     model.TestSpec{Path: "/api/users"},
			expected: "/api/users",
		},
		{
			name: "colon param",
			spec: model.TestSpec{
				Path:       "/api/users/:id",
				PathParams: map[string]interface{}{"id": 42},
			},
			expected: "/api/users/42",
		},
		{
			name: "brace param",
			spec: model.TestSpec{
				Path:       "/api/users/{userId}",
				PathParams: map[string]interface{}{"userId": "abc"},
			},
			expected: "/api/users/abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.resolvePath(tt.spec)
			if result != tt.expected {
				t.Errorf("resolvePath() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestJUnitEmitter_GenerateTestName(t *testing.T) {
	e := &JUnitEmitter{}

	tests := []struct {
		name     string
		spec     model.TestSpec
		expected string
	}{
		{
			name:     "simple GET",
			spec:     model.TestSpec{Method: "GET", Path: "/users"},
			expected: "testGet_users",
		},
		{
			name:     "POST with dashes",
			spec:     model.TestSpec{Method: "POST", Path: "/user-profile"},
			expected: "testPost_user_profile",
		},
		{
			name:     "root path",
			spec:     model.TestSpec{Method: "GET", Path: "/"},
			expected: "testGet_root",
		},
		{
			name:     "path with params",
			spec:     model.TestSpec{Method: "DELETE", Path: "/users/{id}"},
			expected: "testDelete_users_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.generateTestName(tt.spec)
			if result != tt.expected {
				t.Errorf("generateTestName() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestJUnitEmitter_GenerateDisplayName(t *testing.T) {
	e := &JUnitEmitter{}

	spec := model.TestSpec{Method: "POST", Path: "/api/users"}
	expected := "POST /api/users"

	result := e.generateDisplayName(spec)
	if result != expected {
		t.Errorf("generateDisplayName() = %s, want %s", result, expected)
	}
}

func TestJUnitEmitter_EscapeJavaString(t *testing.T) {
	e := &JUnitEmitter{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no escaping needed",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "escape quotes",
			input:    `{"name": "John"}`,
			expected: `{\"name\": \"John\"}`,
		},
		{
			name:     "escape backslash",
			input:    `path\to\file`,
			expected: `path\\to\\file`,
		},
		{
			name:     "escape both",
			input:    `"path\to"`,
			expected: `\"path\\to\"`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.escapeJavaString(tt.input)
			if result != tt.expected {
				t.Errorf("escapeJavaString() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestJUnitEmitter_EmitAssertion(t *testing.T) {
	e := &JUnitEmitter{}

	tests := []struct {
		name      string
		assertion model.Assertion
		contains  string
	}{
		{
			name:      "status 200",
			assertion: model.Assertion{Kind: "status_code", Expected: 200},
			contains:  "status().isOk()",
		},
		{
			name:      "status 201",
			assertion: model.Assertion{Kind: "status_code", Expected: 201},
			contains:  "status().isCreated()",
		},
		{
			name:      "status 204",
			assertion: model.Assertion{Kind: "status_code", Expected: 204},
			contains:  "status().isNoContent()",
		},
		{
			name:      "status 400",
			assertion: model.Assertion{Kind: "status_code", Expected: 400},
			contains:  "status().isBadRequest()",
		},
		{
			name:      "status 401",
			assertion: model.Assertion{Kind: "status_code", Expected: 401},
			contains:  "status().isUnauthorized()",
		},
		{
			name:      "status 403",
			assertion: model.Assertion{Kind: "status_code", Expected: 403},
			contains:  "status().isForbidden()",
		},
		{
			name:      "status 404",
			assertion: model.Assertion{Kind: "status_code", Expected: 404},
			contains:  "status().isNotFound()",
		},
		{
			name:      "status 500",
			assertion: model.Assertion{Kind: "status_code", Expected: 500},
			contains:  "status().isInternalServerError()",
		},
		{
			name:      "status custom",
			assertion: model.Assertion{Kind: "status_code", Expected: 418},
			contains:  "status().is(418)",
		},
		{
			name:      "equality body field",
			assertion: model.Assertion{Kind: "equality", Actual: "body.name", Expected: "John"},
			contains:  "jsonPath",
		},
		{
			name:      "equality non-body",
			assertion: model.Assertion{Kind: "equality", Actual: "header", Expected: "value"},
			contains:  "TODO",
		},
		{
			name:      "not_null body field",
			assertion: model.Assertion{Kind: "not_null", Actual: "body.id"},
			contains:  "jsonPath",
		},
		{
			name:      "not_null non-body",
			assertion: model.Assertion{Kind: "not_null", Actual: "other"},
			contains:  "TODO",
		},
		{
			name:      "contains",
			assertion: model.Assertion{Kind: "contains", Expected: "success"},
			contains:  "containsString",
		},
		{
			name:      "unknown",
			assertion: model.Assertion{Kind: "custom"},
			contains:  "Unknown assertion kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.emitAssertion(tt.assertion)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("emitAssertion() = %s, want to contain %s", result, tt.contains)
			}
		})
	}
}
