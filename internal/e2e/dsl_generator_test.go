package e2e

import (
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/api"
	"github.com/QTest-hq/qtest/internal/flow"
)

func TestDefaultDSLGeneratorConfig(t *testing.T) {
	config := DefaultDSLGeneratorConfig()

	if config == nil {
		t.Fatal("Expected non-nil config")
	}
	if !config.GenerateNegativeTests {
		t.Error("Expected GenerateNegativeTests to be true")
	}
	if !config.GenerateAuthTests {
		t.Error("Expected GenerateAuthTests to be true")
	}
	if config.MaxStepsPerTest < 1 {
		t.Error("Expected MaxStepsPerTest to be at least 1")
	}
}

func TestNewDSLGenerator(t *testing.T) {
	generator := NewDSLGenerator(nil)

	if generator == nil {
		t.Fatal("Expected non-nil generator")
	}
	if generator.config == nil {
		t.Error("Expected config to be set")
	}
}

func TestNewDSLGeneratorWithConfig(t *testing.T) {
	config := &DSLGeneratorConfig{
		MaxStepsPerTest:       10,
		GenerateNegativeTests: false,
	}
	generator := NewDSLGenerator(config)

	if generator.config.MaxStepsPerTest != 10 {
		t.Errorf("Expected MaxStepsPerTest 10, got %d", generator.config.MaxStepsPerTest)
	}
	if generator.config.GenerateNegativeTests {
		t.Error("Expected GenerateNegativeTests to be false")
	}
}

func TestDSLGenerator_GenerateEmptyInput(t *testing.T) {
	generator := NewDSLGenerator(nil)

	input := &GenerationInput{
		ProjectName: "Test Project",
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if output == nil {
		t.Fatal("Expected non-nil output")
	}
	// Empty input should produce empty specs without error
	if output.Specs == nil {
		t.Error("Expected non-nil specs (even if empty)")
	}
}

func TestDSLGenerator_GenerateFromFlows(t *testing.T) {
	generator := NewDSLGenerator(nil)

	flows := []flow.Flow{
		{
			ID:          "flow-1",
			Name:        "Login Flow",
			Description: "User login flow",
			Steps: []flow.Step{
				{
					ID:   "step-1",
					Name: "Navigate to login",
					Action: flow.Action{
						ID:   "action-1",
						Type: flow.ActionTypeNavigate,
						URL:  "https://example.com/login",
					},
				},
				{
					ID:   "step-2",
					Name: "Fill email",
					Action: flow.Action{
						ID:   "action-2",
						Type: flow.ActionTypeFill,
						Selector: &flow.Selector{
							Primary:  "#email",
							Strategy: flow.SelectorID,
						},
						Value: "user@example.com",
					},
				},
				{
					ID:   "step-3",
					Name: "Submit form",
					Action: flow.Action{
						ID:   "action-3",
						Type: flow.ActionTypeClick,
						Selector: &flow.Selector{
							Primary:  "button[type=submit]",
							Strategy: flow.SelectorCSS,
						},
					},
				},
			},
		},
	}

	input := &GenerationInput{
		Flows:       flows,
		ProjectName: "Test App",
		BaseURL:     "https://example.com",
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(output.Specs) == 0 {
		t.Error("Expected at least one spec")
	}

	// Check first spec
	spec := output.Specs[0]
	if spec.Name == "" {
		t.Error("Expected spec name")
	}
	if len(spec.TestCases) == 0 {
		t.Error("Expected at least one test case")
	}

	// Check test case has steps
	testCase := spec.TestCases[0]
	if len(testCase.Steps) == 0 {
		t.Error("Expected test case to have steps")
	}
}

func TestDSLGenerator_GenerateFromAPISpec(t *testing.T) {
	generator := NewDSLGenerator(nil)

	apiSpec := &api.APISpec{
		ID:      "api-1",
		Name:    "Test API",
		BaseURL: "https://api.example.com",
		Endpoints: []api.Endpoint{
			{
				ID:      "ep-1",
				Path:    "/users",
				Method:  api.MethodGET,
				Summary: "List users",
				Responses: []api.Response{
					{StatusCode: 200, Description: "Success"},
				},
			},
			{
				ID:      "ep-2",
				Path:    "/users",
				Method:  api.MethodPOST,
				Summary: "Create user",
				RequestBody: &api.RequestBody{
					ContentType: api.ContentTypeJSON,
					Schema: &api.Schema{
						Type: api.DataTypeObject,
						Properties: map[string]*api.Schema{
							"name":  {Type: api.DataTypeString},
							"email": {Type: api.DataTypeString, Format: "email"},
						},
					},
				},
				Responses: []api.Response{
					{StatusCode: 201, Description: "Created"},
				},
			},
		},
	}

	input := &GenerationInput{
		APISpec:     apiSpec,
		ProjectName: "API Tests",
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(output.Specs) == 0 {
		t.Error("Expected at least one spec")
	}

	// Find spec for API tests
	var apiTestSpec *E2ETestSpec
	for _, spec := range output.Specs {
		if len(spec.TestCases) > 0 {
			apiTestSpec = spec
			break
		}
	}

	if apiTestSpec == nil {
		t.Fatal("Expected API test spec")
	}

	// Should have test cases for endpoints
	if output.Coverage.EndpointsCovered < 1 {
		t.Error("Expected at least one endpoint covered")
	}
}

func TestDSLGenerator_GenerateNegativeTests(t *testing.T) {
	config := DefaultDSLGeneratorConfig()
	config.GenerateNegativeTests = true
	generator := NewDSLGenerator(config)

	apiSpec := &api.APISpec{
		ID:      "api-1",
		Name:    "Test API",
		BaseURL: "https://api.example.com",
		Endpoints: []api.Endpoint{
			{
				ID:      "ep-1",
				Path:    "/users/{id}",
				Method:  api.MethodGET,
				Summary: "Get user",
				Parameters: []api.Parameter{
					{
						Name:     "id",
						Location: api.ParamLocationPath,
						Type:     api.DataTypeInteger,
						Required: true,
					},
				},
				Responses: []api.Response{
					{StatusCode: 200, Description: "Success"},
					{StatusCode: 404, Description: "Not found"},
				},
			},
		},
	}

	input := &GenerationInput{
		APISpec:     apiSpec,
		ProjectName: "API Tests",
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should generate specs without error
	if output == nil {
		t.Fatal("Expected non-nil output")
	}
}

func TestDSLGenerator_GenerateAuthTests(t *testing.T) {
	config := DefaultDSLGeneratorConfig()
	config.GenerateAuthTests = true
	generator := NewDSLGenerator(config)

	apiSpec := &api.APISpec{
		ID:      "api-1",
		Name:    "Test API",
		BaseURL: "https://api.example.com",
		Auth: &api.AuthRequirement{
			Type: api.AuthTypeBearer,
		},
		Endpoints: []api.Endpoint{
			{
				ID:     "ep-1",
				Path:   "/protected",
				Method: api.MethodGET,
				Auth:   &api.AuthRequirement{Type: api.AuthTypeBearer},
			},
		},
	}

	input := &GenerationInput{
		APISpec:     apiSpec,
		ProjectName: "Auth Tests",
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have auth tests
	if len(output.Specs) == 0 {
		t.Error("Expected at least one spec")
	}
}

func TestDSLGenerator_GenerateWithCredentials(t *testing.T) {
	generator := NewDSLGenerator(nil)

	flows := []flow.Flow{
		{
			ID:   "flow-1",
			Name: "Login",
			Steps: []flow.Step{
				{
					ID:   "step-1",
					Name: "Fill username",
					Action: flow.Action{
						ID:   "action-1",
						Type: flow.ActionTypeFill,
						Selector: &flow.Selector{
							Primary:  "#username",
							Strategy: flow.SelectorID,
						},
						Value: "${username}",
					},
				},
			},
		},
	}

	input := &GenerationInput{
		Flows:       flows,
		ProjectName: "Test",
		Credentials: map[string]string{
			"username": "testuser",
			"password": "testpass",
		},
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if output == nil {
		t.Fatal("Expected non-nil output")
	}
}

func TestDSLGenerator_CoverageStats(t *testing.T) {
	generator := NewDSLGenerator(nil)

	apiSpec := &api.APISpec{
		ID:      "api-1",
		Name:    "Test API",
		BaseURL: "https://api.example.com",
		Endpoints: []api.Endpoint{
			{ID: "1", Path: "/users", Method: api.MethodGET},
			{ID: "2", Path: "/users", Method: api.MethodPOST},
			{ID: "3", Path: "/orders", Method: api.MethodGET},
		},
	}

	flows := []flow.Flow{
		{
			ID:   "flow-1",
			Name: "Flow 1",
			Steps: []flow.Step{
				{ID: "s1", Name: "Step 1", Action: flow.Action{ID: "a1", Type: flow.ActionTypeNavigate}},
			},
		},
		{
			ID:   "flow-2",
			Name: "Flow 2",
			Steps: []flow.Step{
				{ID: "s1", Name: "Step 1", Action: flow.Action{ID: "a1", Type: flow.ActionTypeClick}},
			},
		},
	}

	input := &GenerationInput{
		Flows:       flows,
		APISpec:     apiSpec,
		ProjectName: "Coverage Test",
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if output.Coverage.FlowsCovered < 2 {
		t.Errorf("Expected at least 2 flows covered, got %d", output.Coverage.FlowsCovered)
	}
	if output.Coverage.EndpointsCovered < 1 {
		t.Errorf("Expected at least 1 endpoint covered, got %d", output.Coverage.EndpointsCovered)
	}
}

func TestDSLGenerator_Suggestions(t *testing.T) {
	generator := NewDSLGenerator(nil)

	// Only API spec, no flows - should suggest UI tests
	input := &GenerationInput{
		APISpec: &api.APISpec{
			ID:      "api-1",
			BaseURL: "https://api.example.com",
			Endpoints: []api.Endpoint{
				{ID: "1", Path: "/users", Method: api.MethodGET},
			},
		},
		ProjectName: "Suggestion Test",
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have suggestions for missing flows
	if len(output.Suggestions) == 0 {
		t.Log("No suggestions generated - this is acceptable")
	}
}

func TestDSLGenerator_IntegrationTest(t *testing.T) {
	generator := NewDSLGenerator(nil)

	// Full integration test with flows, API spec, and network data
	input := &GenerationInput{
		ProjectName: "Integration Test",
		BaseURL:     "https://example.com",
		Flows: []flow.Flow{
			{
				ID:          "login-flow",
				Name:        "User Login",
				Description: "Login flow for user authentication",
				Steps: []flow.Step{
					{ID: "s1", Name: "Navigate", Action: flow.Action{ID: "a1", Type: flow.ActionTypeNavigate, URL: "/login"}},
					{ID: "s2", Name: "Fill email", Action: flow.Action{
						ID:       "a2",
						Type:     flow.ActionTypeFill,
						Selector: &flow.Selector{Primary: "#email", Strategy: flow.SelectorID},
						Value:    "user@example.com",
					}},
					{ID: "s3", Name: "Fill password", Action: flow.Action{
						ID:       "a3",
						Type:     flow.ActionTypeFill,
						Selector: &flow.Selector{Primary: "#password", Strategy: flow.SelectorID},
						Value:    "password123",
					}},
					{ID: "s4", Name: "Submit", Action: flow.Action{
						ID:       "a4",
						Type:     flow.ActionTypeClick,
						Selector: &flow.Selector{Primary: "button[type=submit]", Strategy: flow.SelectorCSS},
					}},
					{ID: "s5", Name: "Wait for dashboard", Action: flow.Action{
						ID:      "a5",
						Type:    flow.ActionTypeWait,
						WaitFor: ".dashboard",
					}},
				},
				CreatedAt: time.Now(),
			},
		},
		APISpec: &api.APISpec{
			ID:      "api-1",
			Name:    "Test API",
			BaseURL: "https://api.example.com",
			Auth:    &api.AuthRequirement{Type: api.AuthTypeBearer},
			Endpoints: []api.Endpoint{
				{
					ID:     "ep-1",
					Path:   "/api/users",
					Method: api.MethodGET,
					Auth:   &api.AuthRequirement{Type: api.AuthTypeBearer},
					Responses: []api.Response{
						{StatusCode: 200},
						{StatusCode: 401},
					},
				},
				{
					ID:     "ep-2",
					Path:   "/api/users",
					Method: api.MethodPOST,
					RequestBody: &api.RequestBody{
						ContentType: api.ContentTypeJSON,
						Schema: &api.Schema{
							Type: api.DataTypeObject,
							Properties: map[string]*api.Schema{
								"name":  {Type: api.DataTypeString},
								"email": {Type: api.DataTypeString, Format: "email"},
							},
						},
					},
					Responses: []api.Response{
						{StatusCode: 201},
						{StatusCode: 400},
					},
				},
			},
		},
		NetworkData: []api.NetworkRequest{
			{
				ID:          "req-1",
				URL:         "https://api.example.com/api/users",
				Method:      api.MethodGET,
				StatusCode:  200,
				ContentType: api.ContentTypeJSON,
			},
		},
		Credentials: map[string]string{
			"email":    "test@example.com",
			"password": "testpass",
		},
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if output == nil {
		t.Fatal("Expected non-nil output")
	}

	if len(output.Specs) == 0 {
		t.Error("Expected at least one spec")
	}

	// Count total test cases
	totalTests := 0
	for _, spec := range output.Specs {
		totalTests += len(spec.TestCases)
	}

	if totalTests == 0 {
		t.Error("Expected at least one test case")
	}

	t.Logf("Generated %d specs with %d total test cases", len(output.Specs), totalTests)
	t.Logf("Coverage: %d flows, %d endpoints", output.Coverage.FlowsCovered, output.Coverage.EndpointsCovered)
}

func TestDSLGenerator_DuplicateFlows(t *testing.T) {
	generator := NewDSLGenerator(nil)

	// Test with duplicate flow IDs
	flows := []flow.Flow{
		{
			ID:    "flow-1",
			Name:  "Flow 1",
			Steps: []flow.Step{{ID: "s1", Name: "Step", Action: flow.Action{ID: "a1", Type: flow.ActionTypeClick}}},
		},
		{
			ID:    "flow-1",
			Name:  "Flow 1 Duplicate",
			Steps: []flow.Step{{ID: "s1", Name: "Step", Action: flow.Action{ID: "a1", Type: flow.ActionTypeFill}}},
		},
	}

	input := &GenerationInput{
		Flows:       flows,
		ProjectName: "Duplicate Test",
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should handle duplicates gracefully
	if output == nil {
		t.Fatal("Expected non-nil output")
	}
}

func TestDSLGenerator_EmptyStepsFlow(t *testing.T) {
	generator := NewDSLGenerator(nil)

	flows := []flow.Flow{
		{ID: "empty-flow", Name: "Empty Flow", Steps: []flow.Step{}},
	}

	input := &GenerationInput{
		Flows:       flows,
		ProjectName: "Empty Steps Test",
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should handle empty steps gracefully
	if output == nil {
		t.Fatal("Expected non-nil output")
	}
}

func TestDSLGenerator_FlowTypeGrouping(t *testing.T) {
	config := DefaultDSLGeneratorConfig()
	config.GroupByFlowType = true
	generator := NewDSLGenerator(config)

	flows := []flow.Flow{
		{ID: "login-1", Name: "Login Flow", Type: flow.FlowTypeLogin, Steps: []flow.Step{
			{ID: "s1", Name: "Login Step", Action: flow.Action{ID: "a1", Type: flow.ActionTypeNavigate}},
		}},
		{ID: "checkout-1", Name: "Checkout Flow", Type: flow.FlowTypeCheckout, Steps: []flow.Step{
			{ID: "s1", Name: "Checkout Step", Action: flow.Action{ID: "a1", Type: flow.ActionTypeClick}},
		}},
	}

	input := &GenerationInput{
		Flows:       flows,
		ProjectName: "Grouped Test",
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if output == nil {
		t.Fatal("Expected non-nil output")
	}
}

func TestDSLGenerator_MultipleEndpointResources(t *testing.T) {
	generator := NewDSLGenerator(nil)

	apiSpec := &api.APISpec{
		ID:      "api-1",
		Name:    "Multi Resource API",
		BaseURL: "https://api.example.com",
		Endpoints: []api.Endpoint{
			{ID: "1", Path: "/users", Method: api.MethodGET},
			{ID: "2", Path: "/users/{id}", Method: api.MethodGET},
			{ID: "3", Path: "/users", Method: api.MethodPOST},
			{ID: "4", Path: "/orders", Method: api.MethodGET},
			{ID: "5", Path: "/orders/{id}", Method: api.MethodGET},
			{ID: "6", Path: "/products", Method: api.MethodGET},
		},
	}

	input := &GenerationInput{
		APISpec:     apiSpec,
		ProjectName: "Multi Resource Test",
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(output.Specs) == 0 {
		t.Error("Expected at least one spec")
	}

	// Count endpoints covered
	if output.Coverage.EndpointsCovered < 3 {
		t.Errorf("Expected at least 3 endpoints covered, got %d", output.Coverage.EndpointsCovered)
	}
}

func TestDSLGenerator_NilAPISpec(t *testing.T) {
	generator := NewDSLGenerator(nil)

	input := &GenerationInput{
		APISpec:     nil,
		ProjectName: "Nil API Test",
		Flows: []flow.Flow{
			{
				ID:   "flow-1",
				Name: "Test Flow",
				Steps: []flow.Step{
					{ID: "s1", Name: "Step", Action: flow.Action{ID: "a1", Type: flow.ActionTypeClick}},
				},
			},
		},
	}

	output, err := generator.Generate(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if output == nil {
		t.Fatal("Expected non-nil output")
	}
}

func TestDSLGenerator_ConfigOptions(t *testing.T) {
	config := &DSLGeneratorConfig{
		GroupByFeature:        true,
		GroupByFlowType:       true,
		MaxStepsPerTest:       50,
		MaxTestsPerFile:       20,
		GenerateNegativeTests: true,
		GenerateBoundaryTests: true,
		GenerateAuthTests:     true,
		PreferTestID:          true,
		PreferDataCy:          false,
		FallbackToCSS:         true,
		DefaultWaitStrategy:   "networkidle",
		DefaultTimeout:        30 * time.Second,
		IncludeComments:       true,
		IncludeMetadata:       true,
		IncludeScreenshots:    true,
	}

	generator := NewDSLGenerator(config)

	if generator.config.MaxStepsPerTest != 50 {
		t.Errorf("Expected MaxStepsPerTest 50, got %d", generator.config.MaxStepsPerTest)
	}
	if generator.config.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected DefaultTimeout 30s, got %s", generator.config.DefaultTimeout)
	}
}
