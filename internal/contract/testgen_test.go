package contract

import (
	"strings"
	"testing"
)

func TestNewContractTestGenerator(t *testing.T) {
	ctg := NewContractTestGenerator()

	if ctg == nil {
		t.Fatal("NewContractTestGenerator() returned nil")
	}
	if ctg.dataGen == nil {
		t.Error("dataGen should not be nil")
	}
}

func TestGenerateTests_JavaScript(t *testing.T) {
	contract := &Contract{
		Name:    "Test API",
		Version: "1.0.0",
		Endpoints: []EndpointContract{
			{
				ID:     "ep1",
				Method: "GET",
				Path:   "/users",
				Response: ResponseContract{
					StatusCode:  200,
					ContentType: "application/json",
					Body:        &SchemaSpec{Type: "object"},
				},
			},
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.GenerateTests(contract, "javascript")

	// Verify imports
	if !strings.Contains(code, "require('supertest')") {
		t.Error("Should contain supertest import")
	}
	if !strings.Contains(code, "require('ajv')") {
		t.Error("Should contain ajv import")
	}

	// Verify describe block
	if !strings.Contains(code, "describe('Contract Tests'") {
		t.Error("Should contain describe block")
	}

	// Verify endpoint test
	if !strings.Contains(code, "GET /users") {
		t.Error("Should contain endpoint description")
	}
	if !strings.Contains(code, "should return status 200") {
		t.Error("Should contain status code test")
	}
}

func TestGenerateTests_TypeScript(t *testing.T) {
	contract := &Contract{
		Name:    "Test API",
		Version: "1.0.0",
		Endpoints: []EndpointContract{
			{ID: "ep1", Method: "GET", Path: "/users", Response: ResponseContract{StatusCode: 200}},
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.GenerateTests(contract, "typescript")

	// TypeScript should use same Jest generator
	if !strings.Contains(code, "describe('Contract Tests'") {
		t.Error("Should contain describe block for TypeScript")
	}
}

func TestGenerateTests_Python(t *testing.T) {
	contract := &Contract{
		Name:    "Test API",
		Version: "1.0.0",
		Endpoints: []EndpointContract{
			{
				ID:     "ep1",
				Method: "GET",
				Path:   "/users",
				Response: ResponseContract{
					StatusCode: 200,
					Body:       &SchemaSpec{Type: "object"},
				},
			},
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.GenerateTests(contract, "python")

	// Verify imports
	if !strings.Contains(code, "import pytest") {
		t.Error("Should contain pytest import")
	}
	if !strings.Contains(code, "from fastapi.testclient import TestClient") {
		t.Error("Should contain TestClient import")
	}

	// Verify test function
	if !strings.Contains(code, "def test_contract_") {
		t.Error("Should contain test function")
	}
	if !strings.Contains(code, "assert response.status_code ==") {
		t.Error("Should contain status code assertion")
	}
}

func TestGenerateTests_Go(t *testing.T) {
	contract := &Contract{
		Name:    "Test API",
		Version: "1.0.0",
		Endpoints: []EndpointContract{
			{
				ID:     "ep1",
				Method: "GET",
				Path:   "/users",
				Response: ResponseContract{
					StatusCode: 200,
					Body:       &SchemaSpec{Type: "object"},
				},
			},
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.GenerateTests(contract, "go")

	// Verify package
	if !strings.Contains(code, "package contract_test") {
		t.Error("Should contain package declaration")
	}

	// Verify imports
	if !strings.Contains(code, `"net/http"`) {
		t.Error("Should contain net/http import")
	}
	if !strings.Contains(code, `"net/http/httptest"`) {
		t.Error("Should contain httptest import")
	}

	// Verify test function
	if !strings.Contains(code, "func TestContract_GET_") {
		t.Error("Should contain test function")
	}
}

func TestGenerateTests_Default(t *testing.T) {
	contract := &Contract{
		Name:      "Test API",
		Version:   "1.0.0",
		Endpoints: []EndpointContract{},
	}

	ctg := NewContractTestGenerator()
	code := ctg.GenerateTests(contract, "unknown")

	// Should default to Jest
	if !strings.Contains(code, "describe('Contract Tests'") {
		t.Error("Unknown language should default to Jest")
	}
}

func TestGenerateJestEndpointContractTest_PathParams(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "GET",
		Path:   "/users/:id",
		Request: RequestContract{
			PathParams: map[string]ParamSpec{"id": {Type: "string"}},
		},
		Response: ResponseContract{
			StatusCode: 200,
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generateJestEndpointContractTest(ep)

	// Path params should be replaced with test values
	if !strings.Contains(code, "/users/123") {
		t.Error("Should replace :id with test value")
	}
}

func TestGenerateJestEndpointContractTest_BraceParams(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "GET",
		Path:   "/users/{userId}",
		Request: RequestContract{
			PathParams: map[string]ParamSpec{"userId": {Type: "string"}},
		},
		Response: ResponseContract{
			StatusCode: 200,
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generateJestEndpointContractTest(ep)

	// Brace-style params should also be replaced
	if !strings.Contains(code, "/users/123") {
		t.Error("Should replace {userId} with test value")
	}
}

func TestGenerateJestEndpointContractTest_WithHeaders(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "GET",
		Path:   "/users",
		Request: RequestContract{
			Headers: map[string]HeaderSpec{
				"Authorization": {Required: true},
			},
		},
		Response: ResponseContract{
			StatusCode: 200,
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generateJestEndpointContractTest(ep)

	if !strings.Contains(code, ".set('Authorization'") {
		t.Error("Should set required headers")
	}
}

func TestGenerateJestEndpointContractTest_WithBody(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "POST",
		Path:   "/users",
		Request: RequestContract{
			Body: &SchemaSpec{Type: "object"},
		},
		Response: ResponseContract{
			StatusCode: 201,
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generateJestEndpointContractTest(ep)

	if !strings.Contains(code, ".send(") {
		t.Error("Should send body for POST")
	}
}

func TestGenerateJestEndpointContractTest_ContentType(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "GET",
		Path:   "/users",
		Response: ResponseContract{
			StatusCode:  200,
			ContentType: "application/json",
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generateJestEndpointContractTest(ep)

	if !strings.Contains(code, "should return correct content-type") {
		t.Error("Should test content type")
	}
}

func TestGenerateJestEndpointContractTest_RequiredHeader(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "GET",
		Path:   "/users",
		Request: RequestContract{
			Headers: map[string]HeaderSpec{
				"Authorization": {Required: true},
			},
		},
		Response: ResponseContract{
			StatusCode: 200,
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generateJestEndpointContractTest(ep)

	if !strings.Contains(code, "should require Authorization header") {
		t.Error("Should test required header")
	}
}

func TestGeneratePytestEndpointContractTest_Methods(t *testing.T) {
	tests := []struct {
		method   string
		expected string
	}{
		{"GET", "client.get("},
		{"POST", "client.post("},
		{"PUT", "client.put("},
		{"DELETE", "client.delete("},
	}

	ctg := NewContractTestGenerator()

	for _, tt := range tests {
		ep := EndpointContract{
			ID:     "ep1",
			Method: tt.method,
			Path:   "/users",
			Response: ResponseContract{
				StatusCode: 200,
			},
		}

		code := ctg.generatePytestEndpointContractTest(ep)

		if !strings.Contains(code, tt.expected) {
			t.Errorf("Should contain %s for method %s", tt.expected, tt.method)
		}
	}
}

func TestGeneratePytestEndpointContractTest_Headers(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "GET",
		Path:   "/users",
		Request: RequestContract{
			Headers: map[string]HeaderSpec{
				"Authorization": {Required: true},
			},
		},
		Response: ResponseContract{
			StatusCode: 200,
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generatePytestEndpointContractTest(ep)

	if !strings.Contains(code, "headers={") {
		t.Error("Should include headers")
	}
}

func TestGeneratePytestEndpointContractTest_Schema(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "GET",
		Path:   "/users",
		Response: ResponseContract{
			StatusCode: 200,
			Body:       &SchemaSpec{Type: "object"},
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generatePytestEndpointContractTest(ep)

	if !strings.Contains(code, "test_contract_") && strings.Contains(code, "_schema") {
		t.Error("Should include schema test")
	}
	if !strings.Contains(code, "validate(instance=response.json()") {
		t.Error("Should validate response against schema")
	}
}

func TestGenerateGoEndpointContractTest_Basic(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "GET",
		Path:   "/users",
		Response: ResponseContract{
			StatusCode: 200,
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generateGoEndpointContractTest(ep)

	if !strings.Contains(code, "func TestContract_GET_") {
		t.Error("Should have test function")
	}
	if !strings.Contains(code, `http.NewRequest("GET"`) {
		t.Error("Should create HTTP request")
	}
	if !strings.Contains(code, "httptest.NewRecorder()") {
		t.Error("Should use httptest recorder")
	}
}

func TestGenerateGoEndpointContractTest_WithHeaders(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "GET",
		Path:   "/users",
		Request: RequestContract{
			Headers: map[string]HeaderSpec{
				"Authorization": {Required: true},
			},
		},
		Response: ResponseContract{
			StatusCode: 200,
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generateGoEndpointContractTest(ep)

	if !strings.Contains(code, `req.Header.Set("Authorization"`) {
		t.Error("Should set header in test")
	}
}

func TestGenerateGoEndpointContractTest_WithBody(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "GET",
		Path:   "/users",
		Response: ResponseContract{
			StatusCode: 200,
			Body:       &SchemaSpec{Type: "object"},
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generateGoEndpointContractTest(ep)

	if !strings.Contains(code, "_Schema(t *testing.T)") {
		t.Error("Should have schema test function")
	}
	if !strings.Contains(code, "json.Unmarshal") {
		t.Error("Should unmarshal JSON response")
	}
}

func TestGenerateGoEndpointContractTest_PathParams(t *testing.T) {
	ep := EndpointContract{
		ID:     "ep1",
		Method: "GET",
		Path:   "/users/:id",
		Request: RequestContract{
			PathParams: map[string]ParamSpec{"id": {Type: "string"}},
		},
		Response: ResponseContract{
			StatusCode: 200,
		},
	}

	ctg := NewContractTestGenerator()
	code := ctg.generateGoEndpointContractTest(ep)

	// Should replace path params
	if !strings.Contains(code, "/users/123") {
		t.Error("Should replace path params with test value")
	}
}

func TestGenerateContract_MultipleEndpoints(t *testing.T) {
	contract := &Contract{
		Name:    "Test API",
		Version: "1.0.0",
		Endpoints: []EndpointContract{
			{ID: "ep1", Method: "GET", Path: "/users", Response: ResponseContract{StatusCode: 200}},
			{ID: "ep2", Method: "POST", Path: "/users", Response: ResponseContract{StatusCode: 201}},
			{ID: "ep3", Method: "DELETE", Path: "/users/:id", Response: ResponseContract{StatusCode: 204}, Request: RequestContract{PathParams: map[string]ParamSpec{"id": {Type: "string"}}}},
		},
	}

	ctg := NewContractTestGenerator()

	// Test JavaScript
	jsCode := ctg.GenerateTests(contract, "javascript")
	if strings.Count(jsCode, "describe('GET /users'") != 1 {
		t.Error("Should have one GET /users test block")
	}
	if strings.Count(jsCode, "describe('POST /users'") != 1 {
		t.Error("Should have one POST /users test block")
	}
	if strings.Count(jsCode, "describe('DELETE /users/:id'") != 1 {
		t.Error("Should have one DELETE /users/:id test block")
	}

	// Test Python
	pyCode := ctg.GenerateTests(contract, "python")
	if strings.Count(pyCode, "def test_contract_") < 3 {
		t.Error("Should have at least 3 test functions for Python")
	}

	// Test Go
	goCode := ctg.GenerateTests(contract, "go")
	if strings.Count(goCode, "func TestContract_") < 3 {
		t.Error("Should have at least 3 test functions for Go")
	}
}
