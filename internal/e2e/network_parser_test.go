package e2e

import (
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/api"
	"github.com/QTest-hq/qtest/internal/sidecar/playwright"
)

func TestNetworkParser_ParseCapturedRequests(t *testing.T) {
	parser := NewNetworkParser(nil)

	captured := []playwright.CapturedNetworkRequest{
		{
			RequestID:    "req-1",
			Method:       "GET",
			URL:          "https://api.example.com/api/v1/users?page=1",
			Headers:      map[string]string{"Content-Type": "application/json"},
			ResourceType: "xhr",
			Timestamp:    time.Now().UnixMilli(),
			Response: &playwright.CapturedNetworkResponse{
				StatusCode: 200,
				Body:       `[{"id": 1, "name": "John"}]`,
			},
		},
		{
			RequestID:    "req-2",
			Method:       "POST",
			URL:          "https://api.example.com/api/v1/users",
			Headers:      map[string]string{"Content-Type": "application/json"},
			Body:         `{"name": "Jane", "email": "jane@example.com"}`,
			ResourceType: "xhr",
			Timestamp:    time.Now().UnixMilli(),
			Response: &playwright.CapturedNetworkResponse{
				StatusCode: 201,
				Body:       `{"id": 2, "name": "Jane"}`,
			},
		},
	}

	requests := parser.ParseCapturedRequests(captured)

	if len(requests) != 2 {
		t.Errorf("Expected 2 requests, got %d", len(requests))
	}

	// Check first request
	if requests[0].Method != api.MethodGET {
		t.Errorf("Expected GET, got %s", requests[0].Method)
	}
	if requests[0].StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", requests[0].StatusCode)
	}
	if requests[0].QueryParams["page"] != "1" {
		t.Errorf("Expected page=1, got %s", requests[0].QueryParams["page"])
	}
}

func TestNetworkParser_ShouldIgnoreStaticAssets(t *testing.T) {
	parser := NewNetworkParser(nil)

	captured := []playwright.CapturedNetworkRequest{
		{
			RequestID:    "req-1",
			Method:       "GET",
			URL:          "https://example.com/static/bundle.js",
			ResourceType: "script",
		},
		{
			RequestID:    "req-2",
			Method:       "GET",
			URL:          "https://example.com/images/logo.png",
			ResourceType: "image",
		},
		{
			RequestID:    "req-3",
			Method:       "GET",
			URL:          "https://api.example.com/api/users",
			ResourceType: "xhr",
			Response: &playwright.CapturedNetworkResponse{
				StatusCode: 200,
				Body:       `[]`,
			},
		},
	}

	requests := parser.ParseCapturedRequests(captured)

	if len(requests) != 1 {
		t.Errorf("Expected 1 request (static assets filtered), got %d", len(requests))
	}
}

func TestNetworkParser_InferAPISpec(t *testing.T) {
	parser := NewNetworkParser(nil)

	requests := []api.NetworkRequest{
		{
			ID:           "req-1",
			URL:          "https://api.example.com/api/v1/users",
			Method:       api.MethodGET,
			ContentType:  api.ContentTypeJSON,
			StatusCode:   200,
			ResponseBody: `[{"id": 1, "name": "John"}]`,
		},
		{
			ID:           "req-2",
			URL:          "https://api.example.com/api/v1/users/123",
			Method:       api.MethodGET,
			ContentType:  api.ContentTypeJSON,
			StatusCode:   200,
			ResponseBody: `{"id": 123, "name": "John"}`,
		},
		{
			ID:          "req-3",
			URL:         "https://api.example.com/api/v1/users",
			Method:      api.MethodPOST,
			ContentType: api.ContentTypeJSON,
			Body:        `{"name": "Jane", "email": "jane@example.com"}`,
			StatusCode:  201,
			ResponseBody: `{"id": 2, "name": "Jane"}`,
		},
	}

	result := parser.InferAPISpec(requests, "https://api.example.com")

	if result.Spec == nil {
		t.Fatal("Expected spec, got nil")
	}

	if len(result.Spec.Endpoints) != 3 {
		t.Errorf("Expected 3 endpoints, got %d", len(result.Spec.Endpoints))
	}

	if result.Spec.BaseURL != "https://api.example.com" {
		t.Errorf("Expected base URL https://api.example.com, got %s", result.Spec.BaseURL)
	}

	if result.Source != "traffic" {
		t.Errorf("Expected source 'traffic', got %s", result.Source)
	}
}

func TestNetworkParser_InferPathParameters(t *testing.T) {
	parser := NewNetworkParser(nil)

	tests := []struct {
		url      string
		expected string
	}{
		{"https://api.example.com/users/123", "/users/{id}"},
		{"https://api.example.com/users/550e8400-e29b-41d4-a716-446655440000", "/users/{id}"},
		{"https://api.example.com/orders/abc123def456abc123def456", "/orders/{id}"}, // MongoDB ObjectId
		{"https://api.example.com/products/shoes", "/products/shoes"},
	}

	for _, tt := range tests {
		requests := []api.NetworkRequest{
			{URL: tt.url, Method: api.MethodGET, ContentType: api.ContentTypeJSON, StatusCode: 200},
		}
		result := parser.InferAPISpec(requests, "")
		if len(result.Spec.Endpoints) == 0 {
			t.Errorf("No endpoints for URL %s", tt.url)
			continue
		}
		if result.Spec.Endpoints[0].Path != tt.expected {
			t.Errorf("URL %s: expected path %s, got %s", tt.url, tt.expected, result.Spec.Endpoints[0].Path)
		}
	}
}

func TestNetworkParser_InferSchemas(t *testing.T) {
	parser := NewNetworkParser(nil)

	requests := []api.NetworkRequest{
		{
			URL:          "https://api.example.com/api/users",
			Method:       api.MethodPOST,
			ContentType:  api.ContentTypeJSON,
			Body:         `{"name": "John", "age": 30, "active": true, "email": "john@example.com"}`,
			StatusCode:   201,
			ResponseBody: `{"id": 1, "name": "John"}`,
		},
	}

	result := parser.InferAPISpec(requests, "")

	if len(result.Spec.Endpoints) == 0 {
		t.Fatal("Expected endpoints")
	}

	endpoint := result.Spec.Endpoints[0]

	// Check request body schema
	if endpoint.RequestBody == nil {
		t.Fatal("Expected request body")
	}
	if endpoint.RequestBody.Schema == nil {
		t.Fatal("Expected request body schema")
	}

	schema := endpoint.RequestBody.Schema
	if schema.Type != api.DataTypeObject {
		t.Errorf("Expected object type, got %s", schema.Type)
	}

	// Check properties
	if schema.Properties["name"] == nil {
		t.Error("Expected 'name' property")
	}
	if schema.Properties["age"] == nil || schema.Properties["age"].Type != api.DataTypeInteger {
		t.Error("Expected 'age' property with integer type")
	}
	if schema.Properties["active"] == nil || schema.Properties["active"].Type != api.DataTypeBoolean {
		t.Error("Expected 'active' property with boolean type")
	}
	if schema.Properties["email"] == nil || schema.Properties["email"].Format != "email" {
		t.Error("Expected 'email' property with email format")
	}
}

func TestNetworkParser_InferAuth(t *testing.T) {
	config := api.DefaultInferenceConfig()
	config.InferAuth = true
	parser := NewNetworkParser(config)

	requests := []api.NetworkRequest{
		{
			URL:         "https://api.example.com/api/users",
			Method:      api.MethodGET,
			Headers:     map[string]string{"Authorization": "Bearer token123"},
			ContentType: api.ContentTypeJSON,
			StatusCode:  200,
		},
		{
			URL:         "https://api.example.com/api/orders",
			Method:      api.MethodGET,
			Headers:     map[string]string{"Authorization": "Bearer token456"},
			ContentType: api.ContentTypeJSON,
			StatusCode:  200,
		},
	}

	result := parser.InferAPISpec(requests, "")

	if result.Spec.Auth == nil {
		t.Fatal("Expected auth to be inferred")
	}
	if result.Spec.Auth.Type != api.AuthTypeBearer {
		t.Errorf("Expected bearer auth, got %s", result.Spec.Auth.Type)
	}
}

func TestNetworkParser_GenerateSummary(t *testing.T) {
	parser := NewNetworkParser(nil)

	tests := []struct {
		method   api.HTTPMethod
		path     string
		expected string
	}{
		{api.MethodGET, "/users", "List users"},
		{api.MethodGET, "/users/{id}", "Get user"},
		{api.MethodPOST, "/users", "Create user"},
		{api.MethodPUT, "/users/{id}", "Update user"},
		{api.MethodDELETE, "/users/{id}", "Delete user"},
		{api.MethodGET, "/categories", "List categories"},
		{api.MethodGET, "/categories/{id}", "Get category"},
	}

	for _, tt := range tests {
		summary := parser.generateSummary(tt.method, tt.path)
		if summary != tt.expected {
			t.Errorf("%s %s: expected %q, got %q", tt.method, tt.path, tt.expected, summary)
		}
	}
}

func TestNetworkParser_InferQueryParams(t *testing.T) {
	parser := NewNetworkParser(nil)

	requests := []api.NetworkRequest{
		{
			URL:         "https://api.example.com/api/users?page=1&limit=10",
			Method:      api.MethodGET,
			QueryParams: map[string]string{"page": "1", "limit": "10"},
			ContentType: api.ContentTypeJSON,
			StatusCode:  200,
		},
		{
			URL:         "https://api.example.com/api/users?page=2&limit=10",
			Method:      api.MethodGET,
			QueryParams: map[string]string{"page": "2", "limit": "10"},
			ContentType: api.ContentTypeJSON,
			StatusCode:  200,
		},
	}

	result := parser.InferAPISpec(requests, "")

	if len(result.Spec.Endpoints) == 0 {
		t.Fatal("Expected endpoints")
	}

	endpoint := result.Spec.Endpoints[0]

	// Find query params
	var pageParam, limitParam *api.Parameter
	for i := range endpoint.Parameters {
		if endpoint.Parameters[i].Name == "page" {
			pageParam = &endpoint.Parameters[i]
		}
		if endpoint.Parameters[i].Name == "limit" {
			limitParam = &endpoint.Parameters[i]
		}
	}

	if pageParam == nil {
		t.Error("Expected 'page' query parameter")
	} else {
		if pageParam.Location != api.ParamLocationQuery {
			t.Errorf("Expected query location, got %s", pageParam.Location)
		}
		if pageParam.Type != api.DataTypeInteger {
			t.Errorf("Expected integer type, got %s", pageParam.Type)
		}
	}

	if limitParam == nil {
		t.Error("Expected 'limit' query parameter")
	}
}

func TestNetworkParser_EmptyRequests(t *testing.T) {
	parser := NewNetworkParser(nil)

	result := parser.InferAPISpec(nil, "")

	if result.Spec != nil && len(result.Spec.Endpoints) > 0 {
		t.Error("Expected no endpoints for nil input")
	}
	if len(result.Warnings) == 0 {
		t.Error("Expected warning for empty requests")
	}
}

func TestNetworkParser_FilterAPIRequests(t *testing.T) {
	parser := NewNetworkParser(nil)

	requests := []api.NetworkRequest{
		{URL: "https://example.com/api/users", ContentType: api.ContentTypeJSON},
		{URL: "https://example.com/page.html", ContentType: api.ContentTypeHTML},
		{URL: "https://example.com/data", ResponseBody: `{"valid": true}`},
	}

	filtered := parser.filterAPIRequests(requests)

	// Should include JSON content-type and JSON-looking response
	if len(filtered) != 2 {
		t.Errorf("Expected 2 API requests, got %d", len(filtered))
	}
}

// Test helper functions
func TestIsUUID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"not-a-uuid", false},
		{"123", false},
		{"", false},
	}

	for _, tt := range tests {
		if isUUID(tt.input) != tt.expected {
			t.Errorf("isUUID(%q) = %v, want %v", tt.input, !tt.expected, tt.expected)
		}
	}
}

func TestIsNumericID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"1", true},
		{"0", true},
		{"abc", false},
		{"12a", false},
		{"", false},
	}

	for _, tt := range tests {
		if isNumericID(tt.input) != tt.expected {
			t.Errorf("isNumericID(%q) = %v, want %v", tt.input, !tt.expected, tt.expected)
		}
	}
}

func TestSingularize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "user"},
		{"categories", "category"},
		{"boxes", "box"},
		{"user", "user"},
		{"class", "class"},
	}

	for _, tt := range tests {
		if singularize(tt.input) != tt.expected {
			t.Errorf("singularize(%q) = %q, want %q", tt.input, singularize(tt.input), tt.expected)
		}
	}
}
