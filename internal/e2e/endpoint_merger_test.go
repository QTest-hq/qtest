package e2e

import (
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/api"
)

func TestEndpointMerger_MergeBothNil(t *testing.T) {
	merger := NewEndpointMerger(nil)
	result := merger.Merge(nil, nil)

	if len(result.Warnings) == 0 {
		t.Error("Expected warning for nil specs")
	}
}

func TestEndpointMerger_MergeCodeOnly(t *testing.T) {
	merger := NewEndpointMerger(nil)

	codeSpec := &api.APISpec{
		Name:    "Test API",
		BaseURL: "https://api.example.com",
		Endpoints: []api.Endpoint{
			{ID: "1", Path: "/users", Method: api.MethodGET, Summary: "List users"},
		},
	}

	result := merger.Merge(codeSpec, nil)

	if result.Stats.CodeEndpoints != 1 {
		t.Errorf("Expected 1 code endpoint, got %d", result.Stats.CodeEndpoints)
	}
	if result.Stats.TotalEndpoints != 1 {
		t.Errorf("Expected 1 total endpoint, got %d", result.Stats.TotalEndpoints)
	}
}

func TestEndpointMerger_MergeTrafficOnly(t *testing.T) {
	merger := NewEndpointMerger(nil)

	trafficSpec := &api.APISpec{
		Name:    "Traffic API",
		BaseURL: "https://api.example.com",
		Endpoints: []api.Endpoint{
			{ID: "1", Path: "/users", Method: api.MethodGET, Confidence: 0.8},
		},
	}

	result := merger.Merge(nil, trafficSpec)

	if result.Stats.TrafficEndpoints != 1 {
		t.Errorf("Expected 1 traffic endpoint, got %d", result.Stats.TrafficEndpoints)
	}
}

func TestEndpointMerger_MergeMatchingEndpoints(t *testing.T) {
	merger := NewEndpointMerger(nil)

	codeSpec := &api.APISpec{
		Name:    "Code API",
		BaseURL: "https://api.example.com",
		Endpoints: []api.Endpoint{
			{
				ID:          "code-1",
				Path:        "/users",
				Method:      api.MethodGET,
				Summary:     "List all users",
				Description: "Returns a list of users from code",
				Tags:        []string{"users"},
				Confidence:  0.9,
			},
		},
	}

	trafficSpec := &api.APISpec{
		Name:    "Traffic API",
		BaseURL: "https://api.example.com",
		Endpoints: []api.Endpoint{
			{
				ID:         "traffic-1",
				Path:       "/users",
				Method:     api.MethodGET,
				Summary:    "Get users",
				Tags:       []string{"api"},
				Confidence: 0.7,
				Examples: []api.RequestExample{
					{Name: "Example 1", Body: map[string]string{"test": "data"}},
				},
			},
		},
	}

	result := merger.Merge(codeSpec, trafficSpec)

	if result.Stats.MergedEndpoints != 1 {
		t.Errorf("Expected 1 merged endpoint, got %d", result.Stats.MergedEndpoints)
	}
	if result.Stats.TotalEndpoints != 1 {
		t.Errorf("Expected 1 total endpoint, got %d", result.Stats.TotalEndpoints)
	}

	// Check merged properties
	endpoint := result.Spec.Endpoints[0]
	if endpoint.Summary != "List all users" {
		t.Errorf("Expected code summary, got %s", endpoint.Summary)
	}
	if endpoint.Source != "combined" {
		t.Errorf("Expected combined source, got %s", endpoint.Source)
	}

	// Check tags were merged
	if len(endpoint.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(endpoint.Tags))
	}
}

func TestEndpointMerger_NewEndpointsFromTraffic(t *testing.T) {
	merger := NewEndpointMerger(nil)

	codeSpec := &api.APISpec{
		Name:    "Code API",
		BaseURL: "https://api.example.com",
		Endpoints: []api.Endpoint{
			{Path: "/users", Method: api.MethodGET},
		},
	}

	trafficSpec := &api.APISpec{
		Name:    "Traffic API",
		BaseURL: "https://api.example.com",
		Endpoints: []api.Endpoint{
			{Path: "/users", Method: api.MethodGET, Confidence: 0.8},
			{Path: "/orders", Method: api.MethodGET, Confidence: 0.8}, // New from traffic
			{Path: "/products", Method: api.MethodGET, Confidence: 0.1}, // Low confidence, should be skipped
		},
	}

	result := merger.Merge(codeSpec, trafficSpec)

	if result.Stats.NewFromTraffic != 1 {
		t.Errorf("Expected 1 new from traffic, got %d", result.Stats.NewFromTraffic)
	}
	if result.Stats.TotalEndpoints != 2 {
		t.Errorf("Expected 2 total endpoints, got %d", result.Stats.TotalEndpoints)
	}

	// Check that low confidence endpoint was skipped
	if len(result.Warnings) == 0 {
		t.Error("Expected warning about skipped low-confidence endpoint")
	}
}

func TestEndpointMerger_MergeParameters(t *testing.T) {
	merger := NewEndpointMerger(nil)

	codeParams := []api.Parameter{
		{Name: "page", Location: api.ParamLocationQuery, Required: false, Type: api.DataTypeInteger},
		{Name: "id", Location: api.ParamLocationPath, Required: true, Type: api.DataTypeString},
	}

	trafficParams := []api.Parameter{
		{Name: "page", Location: api.ParamLocationQuery, Type: api.DataTypeInteger, Example: 1},
		{Name: "limit", Location: api.ParamLocationQuery, Type: api.DataTypeInteger, Example: 10},
	}

	merged := merger.mergeParameters(codeParams, trafficParams)

	if len(merged) != 3 {
		t.Errorf("Expected 3 merged parameters, got %d", len(merged))
	}

	// Find page param and check it has example from traffic
	var pageParam *api.Parameter
	for i := range merged {
		if merged[i].Name == "page" {
			pageParam = &merged[i]
			break
		}
	}
	if pageParam == nil {
		t.Fatal("Expected page parameter")
	}
	if pageParam.Example == nil {
		t.Error("Expected page to have example from traffic")
	}
}

func TestEndpointMerger_MergeRequestBody(t *testing.T) {
	merger := NewEndpointMerger(nil)

	codeBody := &api.RequestBody{
		ContentType: api.ContentTypeJSON,
		Required:    true,
		Schema: &api.Schema{
			Type: api.DataTypeObject,
			Properties: map[string]*api.Schema{
				"name": {Type: api.DataTypeString},
			},
		},
	}

	trafficBody := &api.RequestBody{
		ContentType: api.ContentTypeJSON,
		Required:    true,
		Schema: &api.Schema{
			Type: api.DataTypeObject,
			Properties: map[string]*api.Schema{
				"name":  {Type: api.DataTypeString},
				"email": {Type: api.DataTypeString, Format: "email"},
			},
		},
		Example: map[string]string{"name": "John", "email": "john@example.com"},
	}

	merged := merger.mergeRequestBody(codeBody, trafficBody)

	if merged.Example == nil {
		t.Error("Expected example from traffic")
	}

	// Check schema has both properties
	if len(merged.Schema.Properties) != 2 {
		t.Errorf("Expected 2 schema properties, got %d", len(merged.Schema.Properties))
	}
}

func TestEndpointMerger_MergeResponses(t *testing.T) {
	merger := NewEndpointMerger(nil)

	codeResponses := []api.Response{
		{StatusCode: 200, Description: "OK"},
		{StatusCode: 404, Description: "Not Found"},
	}

	trafficResponses := []api.Response{
		{StatusCode: 200, Description: "Success", Example: map[string]int{"id": 1}},
		{StatusCode: 500, Description: "Server Error"},
	}

	merged := merger.mergeResponses(codeResponses, trafficResponses)

	if len(merged) != 3 { // 200, 404, 500
		t.Errorf("Expected 3 merged responses, got %d", len(merged))
	}

	// Check 200 has example from traffic
	var resp200 *api.Response
	for i := range merged {
		if merged[i].StatusCode == 200 {
			resp200 = &merged[i]
			break
		}
	}
	if resp200 == nil {
		t.Fatal("Expected 200 response")
	}
	if resp200.Example == nil {
		t.Error("Expected 200 to have example from traffic")
	}
}

func TestEndpointMerger_MergeSchemas(t *testing.T) {
	merger := NewEndpointMerger(nil)

	codeSchema := &api.Schema{
		Type: api.DataTypeObject,
		Properties: map[string]*api.Schema{
			"id":   {Type: api.DataTypeInteger},
			"name": {Type: api.DataTypeString},
		},
	}

	trafficSchema := &api.Schema{
		Type: api.DataTypeObject,
		Properties: map[string]*api.Schema{
			"name":  {Type: api.DataTypeString, Example: "John"},
			"email": {Type: api.DataTypeString, Format: "email"},
		},
		Example: map[string]interface{}{"id": 1, "name": "John", "email": "john@example.com"},
	}

	merged := merger.mergeSchema(codeSchema, trafficSchema)

	if len(merged.Properties) != 3 { // id, name, email
		t.Errorf("Expected 3 properties, got %d", len(merged.Properties))
	}

	// Check traffic example is used
	if merged.Example == nil {
		t.Error("Expected example from traffic")
	}
}

func TestEndpointMerger_NormalizePath(t *testing.T) {
	merger := NewEndpointMerger(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"/users/{id}", "/users/{id}"},
		{"/users/:id", "/users/{id}"},
		{"/users/", "/users"},
		{"/", "/"},
	}

	for _, tt := range tests {
		result := merger.normalizePath(tt.input)
		if result != tt.expected {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestEndpointMerger_MergeTags(t *testing.T) {
	merger := NewEndpointMerger(nil)

	tags := merger.mergeTags([]string{"users", "api"}, []string{"v1", "users"})

	if len(tags) != 3 { // users, api, v1 (deduplicated)
		t.Errorf("Expected 3 tags, got %d", len(tags))
	}
}

func TestEndpointMerger_MergeExamples(t *testing.T) {
	merger := NewEndpointMerger(nil)

	codeExamples := []api.RequestExample{
		{Name: "Code example"},
	}

	trafficExamples := []api.RequestExample{
		{Name: "Traffic example"},
	}

	merged := merger.mergeExamples(codeExamples, trafficExamples)

	if len(merged) != 2 {
		t.Errorf("Expected 2 examples, got %d", len(merged))
	}

	// Traffic example should be prefixed
	found := false
	for _, ex := range merged {
		if ex.Name == "Captured: Traffic example" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected traffic example to be prefixed with 'Captured:'")
	}
}

func TestEndpointMerger_MergeWithAuth(t *testing.T) {
	merger := NewEndpointMerger(nil)

	codeSpec := &api.APISpec{
		Name: "Code API",
		Auth: &api.AuthRequirement{Type: api.AuthTypeBearer},
		Endpoints: []api.Endpoint{
			{Path: "/users", Method: api.MethodGET},
		},
	}

	trafficSpec := &api.APISpec{
		Name: "Traffic API",
		Auth: &api.AuthRequirement{Type: api.AuthTypeAPIKey, Name: "X-API-Key"},
		Endpoints: []api.Endpoint{
			{Path: "/users", Method: api.MethodGET, Confidence: 0.8},
		},
	}

	result := merger.Merge(codeSpec, trafficSpec)

	// Code auth should be preferred
	if result.Spec.Auth.Type != api.AuthTypeBearer {
		t.Errorf("Expected bearer auth from code, got %s", result.Spec.Auth.Type)
	}
}

func TestEndpointMerger_ConfidenceBoost(t *testing.T) {
	merger := NewEndpointMerger(nil)

	codeEndpoint := &api.Endpoint{
		Path:       "/users",
		Method:     api.MethodGET,
		Confidence: 0.8,
	}

	trafficEndpoint := &api.Endpoint{
		Path:       "/users",
		Method:     api.MethodGET,
		Confidence: 0.7,
	}

	merged := merger.mergeEndpoints(codeEndpoint, trafficEndpoint)

	// Both sources agree, confidence should be boosted to 0.9
	if merged.Confidence != 0.9 {
		t.Errorf("Expected boosted confidence 0.9, got %f", merged.Confidence)
	}
}

func TestEndpointMerger_SortedOutput(t *testing.T) {
	merger := NewEndpointMerger(nil)

	// Use both specs to trigger the merge path that sorts
	codeSpec := &api.APISpec{
		Endpoints: []api.Endpoint{
			{Path: "/users", Method: api.MethodPOST},
			{Path: "/orders", Method: api.MethodGET},
		},
	}

	trafficSpec := &api.APISpec{
		Endpoints: []api.Endpoint{
			{Path: "/users", Method: api.MethodGET, Confidence: 0.8},
		},
	}

	result := merger.Merge(codeSpec, trafficSpec)

	// Should be sorted by path, then method
	expected := []string{
		"GET /orders",
		"GET /users",
		"POST /users",
	}

	if len(result.Spec.Endpoints) != 3 {
		t.Fatalf("Expected 3 endpoints, got %d", len(result.Spec.Endpoints))
	}

	for i, ep := range result.Spec.Endpoints {
		actual := string(ep.Method) + " " + ep.Path
		if actual != expected[i] {
			t.Errorf("Position %d: expected %s, got %s", i, expected[i], actual)
		}
	}
}

func TestEndpointMerger_MergeSchemaSpecs(t *testing.T) {
	config := DefaultMergeConfig()
	config.MergeSchemas = true
	merger := NewEndpointMerger(config)

	codeSpec := &api.APISpec{
		Schemas: map[string]api.Schema{
			"User": {Type: api.DataTypeObject},
		},
		Endpoints: []api.Endpoint{},
	}

	trafficSpec := &api.APISpec{
		Schemas: map[string]api.Schema{
			"Order": {Type: api.DataTypeObject},
		},
		Endpoints: []api.Endpoint{},
	}

	result := merger.Merge(codeSpec, trafficSpec)

	if len(result.Spec.Schemas) != 2 {
		t.Errorf("Expected 2 schemas, got %d", len(result.Spec.Schemas))
	}
}

func TestEndpointMerger_CreatedAtPreserved(t *testing.T) {
	merger := NewEndpointMerger(nil)

	originalTime := time.Now().Add(-24 * time.Hour)
	codeSpec := &api.APISpec{
		Endpoints: []api.Endpoint{
			{Path: "/users", Method: api.MethodGET, CreatedAt: originalTime},
		},
	}

	trafficSpec := &api.APISpec{
		Endpoints: []api.Endpoint{
			{Path: "/users", Method: api.MethodGET, CreatedAt: time.Now(), Confidence: 0.8},
		},
	}

	result := merger.Merge(codeSpec, trafficSpec)

	if result.Spec.Endpoints[0].CreatedAt != originalTime {
		t.Error("Expected original created_at to be preserved from code")
	}
}
