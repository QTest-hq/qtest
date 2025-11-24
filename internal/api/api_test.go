package api

import (
	"testing"
)

func TestDefaultInferenceConfig(t *testing.T) {
	config := DefaultInferenceConfig()

	if config.MinConfidence != 0.5 {
		t.Errorf("MinConfidence = %v, want 0.5", config.MinConfidence)
	}
	if !config.InferAuth {
		t.Error("InferAuth should be true by default")
	}
	if !config.InferSchemas {
		t.Error("InferSchemas should be true by default")
	}
	if !config.GroupByResource {
		t.Error("GroupByResource should be true by default")
	}
	if config.MaxRequestsToInfer != 1000 {
		t.Errorf("MaxRequestsToInfer = %v, want 1000", config.MaxRequestsToInfer)
	}
	if len(config.IgnorePatterns) == 0 {
		t.Error("IgnorePatterns should not be empty")
	}
}

func TestHTTPMethods(t *testing.T) {
	tests := []struct {
		method   HTTPMethod
		expected string
	}{
		{MethodGET, "GET"},
		{MethodPOST, "POST"},
		{MethodPUT, "PUT"},
		{MethodPATCH, "PATCH"},
		{MethodDELETE, "DELETE"},
		{MethodOPTIONS, "OPTIONS"},
		{MethodHEAD, "HEAD"},
	}

	for _, tt := range tests {
		if string(tt.method) != tt.expected {
			t.Errorf("HTTPMethod %v = %q, want %q", tt.method, string(tt.method), tt.expected)
		}
	}
}

func TestContentTypes(t *testing.T) {
	tests := []struct {
		ct       ContentType
		expected string
	}{
		{ContentTypeJSON, "application/json"},
		{ContentTypeForm, "application/x-www-form-urlencoded"},
		{ContentTypeMultipart, "multipart/form-data"},
		{ContentTypeXML, "application/xml"},
		{ContentTypeText, "text/plain"},
		{ContentTypeHTML, "text/html"},
	}

	for _, tt := range tests {
		if string(tt.ct) != tt.expected {
			t.Errorf("ContentType %v = %q, want %q", tt.ct, string(tt.ct), tt.expected)
		}
	}
}

func TestParameterLocations(t *testing.T) {
	tests := []struct {
		loc      ParameterLocation
		expected string
	}{
		{ParamLocationPath, "path"},
		{ParamLocationQuery, "query"},
		{ParamLocationHeader, "header"},
		{ParamLocationCookie, "cookie"},
		{ParamLocationBody, "body"},
	}

	for _, tt := range tests {
		if string(tt.loc) != tt.expected {
			t.Errorf("ParameterLocation %v = %q, want %q", tt.loc, string(tt.loc), tt.expected)
		}
	}
}

func TestDataTypes(t *testing.T) {
	tests := []struct {
		dt       DataType
		expected string
	}{
		{DataTypeString, "string"},
		{DataTypeInteger, "integer"},
		{DataTypeNumber, "number"},
		{DataTypeBoolean, "boolean"},
		{DataTypeArray, "array"},
		{DataTypeObject, "object"},
	}

	for _, tt := range tests {
		if string(tt.dt) != tt.expected {
			t.Errorf("DataType %v = %q, want %q", tt.dt, string(tt.dt), tt.expected)
		}
	}
}

func TestAuthTypes(t *testing.T) {
	tests := []struct {
		at       AuthType
		expected string
	}{
		{AuthTypeNone, "none"},
		{AuthTypeBasic, "basic"},
		{AuthTypeBearer, "bearer"},
		{AuthTypeAPIKey, "apikey"},
		{AuthTypeOAuth2, "oauth2"},
		{AuthTypeCookie, "cookie"},
	}

	for _, tt := range tests {
		if string(tt.at) != tt.expected {
			t.Errorf("AuthType %v = %q, want %q", tt.at, string(tt.at), tt.expected)
		}
	}
}

func TestOpenAPIParser_ParseOpenAPI3(t *testing.T) {
	parser := NewOpenAPIParser(nil)

	spec := []byte(`{
		"openapi": "3.0.0",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"servers": [{"url": "https://api.example.com"}],
		"paths": {
			"/users": {
				"get": {
					"summary": "List users",
					"responses": {
						"200": {"description": "Success"}
					}
				},
				"post": {
					"summary": "Create user",
					"requestBody": {
						"content": {
							"application/json": {
								"schema": {"type": "object"}
							}
						}
					}
				}
			},
			"/users/{id}": {
				"get": {
					"summary": "Get user",
					"parameters": [
						{"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}
					]
				}
			}
		}
	}`)

	result, err := parser.Parse(spec)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if result.Spec.Name != "Test API" {
		t.Errorf("Name = %q, want %q", result.Spec.Name, "Test API")
	}

	if result.Spec.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %q, want %q", result.Spec.BaseURL, "https://api.example.com")
	}

	if len(result.Spec.Endpoints) != 3 {
		t.Errorf("Endpoints count = %d, want 3", len(result.Spec.Endpoints))
	}

	if result.Source != "openapi" {
		t.Errorf("Source = %q, want %q", result.Source, "openapi")
	}

	if result.Confidence < 0.9 {
		t.Errorf("Confidence = %v, should be >= 0.9", result.Confidence)
	}
}

func TestOpenAPIParser_ParseSwagger2(t *testing.T) {
	parser := NewOpenAPIParser(nil)

	spec := []byte(`{
		"swagger": "2.0",
		"info": {
			"title": "Swagger API",
			"version": "1.0.0"
		},
		"host": "api.example.com",
		"basePath": "/v1",
		"schemes": ["https"],
		"paths": {
			"/products": {
				"get": {
					"summary": "List products",
					"parameters": [
						{"name": "limit", "in": "query", "type": "integer"}
					]
				}
			}
		}
	}`)

	result, err := parser.Parse(spec)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if result.Spec.Name != "Swagger API" {
		t.Errorf("Name = %q, want %q", result.Spec.Name, "Swagger API")
	}

	if result.Spec.BaseURL != "https://api.example.com/v1" {
		t.Errorf("BaseURL = %q, want %q", result.Spec.BaseURL, "https://api.example.com/v1")
	}

	if len(result.Spec.Endpoints) != 1 {
		t.Errorf("Endpoints count = %d, want 1", len(result.Spec.Endpoints))
	}
}

func TestOpenAPIParser_ParseYAML(t *testing.T) {
	parser := NewOpenAPIParser(nil)

	spec := []byte(`
openapi: "3.0.0"
info:
  title: YAML API
  version: "1.0.0"
servers:
  - url: https://yaml.example.com
paths:
  /items:
    get:
      summary: List items
`)

	result, err := parser.Parse(spec)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if result.Spec.Name != "YAML API" {
		t.Errorf("Name = %q, want %q", result.Spec.Name, "YAML API")
	}
}

func TestOpenAPIParser_InvalidSpec(t *testing.T) {
	parser := NewOpenAPIParser(nil)

	// Not JSON or YAML
	_, err := parser.Parse([]byte("invalid content"))
	if err == nil {
		t.Error("Expected error for invalid spec")
	}

	// Missing version
	_, err = parser.Parse([]byte(`{"info": {"title": "Test"}}`))
	if err == nil {
		t.Error("Expected error for missing version")
	}
}

func TestTrafficAnalyzer_Analyze(t *testing.T) {
	analyzer := NewTrafficAnalyzer(nil)

	requests := []NetworkRequest{
		{
			ID:          "1",
			URL:         "https://api.example.com/api/v1/users",
			Method:      MethodGET,
			ContentType: ContentTypeJSON,
			StatusCode:  200,
		},
		{
			ID:           "2",
			URL:          "https://api.example.com/api/v1/users",
			Method:       MethodPOST,
			ContentType:  ContentTypeJSON,
			Body:         `{"name": "John", "email": "john@example.com"}`,
			StatusCode:   201,
			ResponseBody: `{"id": "123", "name": "John"}`,
		},
		{
			ID:          "3",
			URL:         "https://api.example.com/api/v1/users/123",
			Method:      MethodGET,
			ContentType: ContentTypeJSON,
			StatusCode:  200,
		},
	}

	result, err := analyzer.Analyze(requests)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	if result.Source != "traffic" {
		t.Errorf("Source = %q, want %q", result.Source, "traffic")
	}

	if result.Spec.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %q, want %q", result.Spec.BaseURL, "https://api.example.com")
	}

	if len(result.Spec.Endpoints) == 0 {
		t.Error("Should have inferred at least one endpoint")
	}
}

func TestTrafficAnalyzer_FilterRequests(t *testing.T) {
	config := DefaultInferenceConfig()
	analyzer := NewTrafficAnalyzer(config)

	requests := []NetworkRequest{
		{ID: "1", URL: "https://api.example.com/api/users", Method: MethodGET, ContentType: ContentTypeJSON},
		{ID: "2", URL: "https://api.example.com/static/app.js", Method: MethodGET},
		{ID: "3", URL: "https://api.example.com/assets/logo.png", Method: MethodGET},
		{ID: "4", URL: "https://api.example.com/api/products", Method: MethodGET, ContentType: ContentTypeJSON},
	}

	result, _ := analyzer.Analyze(requests)

	// Should filter out static assets
	for _, ep := range result.Spec.Endpoints {
		if ep.Path == "/static/app.js" || ep.Path == "/assets/logo.png" {
			t.Errorf("Should have filtered out static asset: %s", ep.Path)
		}
	}
}

func TestTrafficAnalyzer_NormalizePath(t *testing.T) {
	analyzer := NewTrafficAnalyzer(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"/users/123", "/users/{id}"},
		{"/users/550e8400-e29b-41d4-a716-446655440000", "/users/{id}"},
		{"/orders/abc123def456789012345678", "/orders/{id}"},
		{"/products/42/reviews", "/products/{id}/reviews"},
		{"/api/v1/users", "/api/v1/users"},
	}

	for _, tt := range tests {
		result := analyzer.normalizePath(tt.input)
		if result != tt.expected {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestTrafficAnalyzer_InferDataType(t *testing.T) {
	analyzer := NewTrafficAnalyzer(nil)

	tests := []struct {
		values   []string
		expected DataType
	}{
		{[]string{"123", "456", "789"}, DataTypeInteger},
		{[]string{"12.5", "3.14", "0.5"}, DataTypeNumber},
		{[]string{"true", "false", "true"}, DataTypeBoolean},
		{[]string{"hello", "world"}, DataTypeString},
		{[]string{}, DataTypeString},
	}

	for _, tt := range tests {
		result := analyzer.inferDataType(tt.values)
		if result != tt.expected {
			t.Errorf("inferDataType(%v) = %v, want %v", tt.values, result, tt.expected)
		}
	}
}

func TestTrafficAnalyzer_EmptyRequests(t *testing.T) {
	analyzer := NewTrafficAnalyzer(nil)

	_, err := analyzer.Analyze([]NetworkRequest{})
	if err == nil {
		t.Error("Expected error for empty requests")
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"550E8400-E29B-41D4-A716-446655440000", true},
		{"not-a-uuid", false},
		{"123", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isUUID(tt.input)
		if result != tt.expected {
			t.Errorf("isUUID(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestIsNumericID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"456789", true},
		{"0", true},
		{"abc", false},
		{"12.5", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isNumericID(tt.input)
		if result != tt.expected {
			t.Errorf("isNumericID(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestIsObjectID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"507f1f77bcf86cd799439011", true},
		{"507F1F77BCF86CD799439011", true},
		{"not-an-objectid", false},
		{"123", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isObjectID(tt.input)
		if result != tt.expected {
			t.Errorf("isObjectID(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestIsEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"test@example.com", true},
		{"user.name@domain.org", true},
		{"notanemail", false},
		{"@nodomain", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isEmail(tt.input)
		if result != tt.expected {
			t.Errorf("isEmail(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestHttpStatusDescription(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{200, "Successful response"},
		{201, "Resource created"},
		{400, "Bad request"},
		{401, "Unauthorized"},
		{404, "Not found"},
		{500, "Internal server error"},
		{418, "HTTP 418"}, // Unknown status
	}

	for _, tt := range tests {
		result := httpStatusDescription(tt.code)
		if result != tt.expected {
			t.Errorf("httpStatusDescription(%d) = %q, want %q", tt.code, result, tt.expected)
		}
	}
}

func TestExtractJSONFromResponse(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`Here is the JSON: [{"name": "test"}]`, `[{"name": "test"}]`},
		{`Response: {"key": "value"}`, `{"key": "value"}`},
		{`[{"a": 1}]`, `[{"a": 1}]`},
		{`No JSON here`, ""},
		{`Partial [{"incomplete"`, ""},
	}

	for _, tt := range tests {
		result := extractJSONFromResponse(tt.input)
		if result != tt.expected {
			t.Errorf("extractJSONFromResponse(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestEndpointStructure(t *testing.T) {
	endpoint := Endpoint{
		ID:      "test-id",
		Path:    "/api/users",
		Method:  MethodGET,
		Summary: "List users",
		Parameters: []Parameter{
			{Name: "limit", Location: ParamLocationQuery, Type: DataTypeInteger},
		},
		Responses: []Response{
			{StatusCode: 200, Description: "Success"},
		},
		Confidence: 0.9,
	}

	if endpoint.Path != "/api/users" {
		t.Errorf("Path = %q, want %q", endpoint.Path, "/api/users")
	}

	if len(endpoint.Parameters) != 1 {
		t.Errorf("Parameters count = %d, want 1", len(endpoint.Parameters))
	}

	if len(endpoint.Responses) != 1 {
		t.Errorf("Responses count = %d, want 1", len(endpoint.Responses))
	}
}

func TestAPISpecStructure(t *testing.T) {
	spec := APISpec{
		ID:      "spec-id",
		Name:    "Test API",
		Version: "1.0.0",
		BaseURL: "https://api.example.com",
		Endpoints: []Endpoint{
			{ID: "1", Path: "/users", Method: MethodGET},
			{ID: "2", Path: "/users", Method: MethodPOST},
		},
		Schemas: map[string]Schema{
			"User": {Type: DataTypeObject},
		},
	}

	if spec.Name != "Test API" {
		t.Errorf("Name = %q, want %q", spec.Name, "Test API")
	}

	if len(spec.Endpoints) != 2 {
		t.Errorf("Endpoints count = %d, want 2", len(spec.Endpoints))
	}

	if len(spec.Schemas) != 1 {
		t.Errorf("Schemas count = %d, want 1", len(spec.Schemas))
	}
}
