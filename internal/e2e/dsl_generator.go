// Package e2e provides end-to-end test generation capabilities.
package e2e

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/QTest-hq/qtest/internal/api"
	"github.com/QTest-hq/qtest/internal/flow"
)

// DSLGenerator generates E2E test DSL from various sources.
type DSLGenerator struct {
	config *DSLGeneratorConfig
}

// DSLGeneratorConfig configures DSL generation behavior.
type DSLGeneratorConfig struct {
	// Test organization
	GroupByFeature    bool `json:"groupByFeature"`
	GroupByFlowType   bool `json:"groupByFlowType"`
	MaxStepsPerTest   int  `json:"maxStepsPerTest"`
	MaxTestsPerFile   int  `json:"maxTestsPerFile"`

	// Enhancement options
	GenerateNegativeTests bool `json:"generateNegativeTests"`
	GenerateBoundaryTests bool `json:"generateBoundaryTests"`
	GenerateAuthTests     bool `json:"generateAuthTests"`

	// Selector preferences
	PreferTestID      bool `json:"preferTestID"`
	PreferDataCy      bool `json:"preferDataCy"`
	FallbackToCSS     bool `json:"fallbackToCSS"`

	// Wait strategies
	DefaultWaitStrategy string        `json:"defaultWaitStrategy"` // networkidle, load, selector
	DefaultTimeout      time.Duration `json:"defaultTimeout"`

	// Output options
	IncludeComments     bool `json:"includeComments"`
	IncludeMetadata     bool `json:"includeMetadata"`
	IncludeScreenshots  bool `json:"includeScreenshots"`
}

// DefaultDSLGeneratorConfig returns default configuration.
func DefaultDSLGeneratorConfig() *DSLGeneratorConfig {
	return &DSLGeneratorConfig{
		GroupByFeature:        true,
		GroupByFlowType:       false,
		MaxStepsPerTest:       20,
		MaxTestsPerFile:       10,
		GenerateNegativeTests: true,
		GenerateBoundaryTests: false,
		GenerateAuthTests:     true,
		PreferTestID:          true,
		PreferDataCy:          true,
		FallbackToCSS:         true,
		DefaultWaitStrategy:   "networkidle",
		DefaultTimeout:        30 * time.Second,
		IncludeComments:       true,
		IncludeMetadata:       true,
		IncludeScreenshots:    false,
	}
}

// NewDSLGenerator creates a new DSL generator.
func NewDSLGenerator(config *DSLGeneratorConfig) *DSLGenerator {
	if config == nil {
		config = DefaultDSLGeneratorConfig()
	}
	return &DSLGenerator{config: config}
}

// GenerationInput holds all inputs for DSL generation.
type GenerationInput struct {
	Flows          []flow.Flow        `json:"flows"`
	APISpec        *api.APISpec       `json:"apiSpec,omitempty"`
	NetworkData    []api.NetworkRequest `json:"networkData,omitempty"`
	Hints          []flow.FlowHint    `json:"hints,omitempty"`
	Credentials    map[string]string  `json:"credentials,omitempty"`
	BaseURL        string             `json:"baseUrl"`
	ProjectName    string             `json:"projectName"`
}

// GenerationOutput holds the generated DSL specs.
type GenerationOutput struct {
	Specs       []*E2ETestSpec `json:"specs"`
	TotalTests  int            `json:"totalTests"`
	TotalSteps  int            `json:"totalSteps"`
	Coverage    *CoverageStats `json:"coverage,omitempty"`
	Suggestions []string       `json:"suggestions,omitempty"`
	Warnings    []string       `json:"warnings,omitempty"`
}

// CoverageStats shows what was covered by generated tests.
type CoverageStats struct {
	FlowsCovered      int      `json:"flowsCovered"`
	EndpointsCovered  int      `json:"endpointsCovered"`
	FormsCovered      int      `json:"formsCovered"`
	AuthFlowsCovered  bool     `json:"authFlowsCovered"`
	CRUDFlowsCovered  []string `json:"crudFlowsCovered,omitempty"` // e.g., ["create", "read", "update"]
}

// Generate creates E2E test specs from the input.
func (g *DSLGenerator) Generate(input *GenerationInput) (*GenerationOutput, error) {
	output := &GenerationOutput{
		Specs:    make([]*E2ETestSpec, 0),
		Coverage: &CoverageStats{},
	}

	// Generate specs from flows
	if len(input.Flows) > 0 {
		flowSpecs := g.generateFromFlows(input.Flows, input.Credentials, input.BaseURL)
		output.Specs = append(output.Specs, flowSpecs...)
		output.Coverage.FlowsCovered = len(input.Flows)
	}

	// Generate specs from API spec
	if input.APISpec != nil && len(input.APISpec.Endpoints) > 0 {
		apiSpecs := g.generateFromAPISpec(input.APISpec)
		output.Specs = append(output.Specs, apiSpecs...)
		output.Coverage.EndpointsCovered = len(input.APISpec.Endpoints)
	}

	// Generate negative/auth tests if configured
	if g.config.GenerateAuthTests && input.APISpec != nil && input.APISpec.Auth != nil {
		authSpec := g.generateAuthTests(input.APISpec)
		if authSpec != nil {
			output.Specs = append(output.Specs, authSpec)
			output.Coverage.AuthFlowsCovered = true
		}
	}

	// Aggregate stats
	for _, spec := range output.Specs {
		output.TotalTests += len(spec.TestCases)
		for _, tc := range spec.TestCases {
			output.TotalSteps += len(tc.Steps)
		}
	}

	// Add suggestions
	output.Suggestions = g.generateSuggestions(input, output)

	return output, nil
}

// generateFromFlows converts flows to E2E test specs.
func (g *DSLGenerator) generateFromFlows(flows []flow.Flow, credentials map[string]string, baseURL string) []*E2ETestSpec {
	var specs []*E2ETestSpec

	if g.config.GroupByFlowType {
		// Group by flow type
		grouped := make(map[flow.FlowType][]flow.Flow)
		for _, f := range flows {
			grouped[f.Type] = append(grouped[f.Type], f)
		}

		for flowType, typeFlows := range grouped {
			spec := g.createSpecFromFlows(typeFlows, string(flowType)+" Tests", credentials, baseURL)
			specs = append(specs, spec)
		}
	} else {
		// One spec per flow
		for _, f := range flows {
			spec := FlowToSpec(&f, credentials)
			if baseURL != "" && spec.BaseURL == "" {
				spec.BaseURL = baseURL
			}
			specs = append(specs, spec)
		}
	}

	return specs
}

// createSpecFromFlows creates a single spec from multiple flows.
func (g *DSLGenerator) createSpecFromFlows(flows []flow.Flow, name string, credentials map[string]string, baseURL string) *E2ETestSpec {
	spec := &E2ETestSpec{
		ID:        uuid.New().String(),
		Name:      name,
		BaseURL:   baseURL,
		TestCases: make([]TestCase, 0),
		Config:    DefaultTestConfig(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	for _, f := range flows {
		tc := g.flowToTestCase(&f, credentials)
		spec.TestCases = append(spec.TestCases, tc)

		if spec.BaseURL == "" && f.StartURL != "" {
			spec.BaseURL = f.StartURL
		}
	}

	return spec
}

// flowToTestCase converts a single flow to a test case.
func (g *DSLGenerator) flowToTestCase(f *flow.Flow, credentials map[string]string) TestCase {
	tc := TestCase{
		ID:          uuid.New().String(),
		Name:        f.Name,
		Description: f.Description,
		Tags:        append(f.Tags, string(f.Type)),
		Steps:       make([]TestStep, 0),
		Timeout:     g.config.DefaultTimeout,
	}

	for _, step := range f.Steps {
		testStep := g.convertFlowStep(&step, credentials)
		tc.Steps = append(tc.Steps, testStep)
	}

	// Add default assertions based on flow type
	tc.Expected = g.generateFlowTypeAssertions(f.Type)

	return tc
}

// convertFlowStep converts a flow step to a test step.
func (g *DSLGenerator) convertFlowStep(step *flow.Step, credentials map[string]string) TestStep {
	testStep := TestStep{
		ID:    uuid.New().String(),
		Name:  step.Name,
		Order: step.Order,
		Action: TestAction{
			Type:      step.Action.Type,
			Value:     step.Action.Value,
			URL:       step.Action.URL,
			Key:       step.Action.Key,
			Modifiers: step.Action.Modifiers,
		},
	}

	// Handle selector
	if step.Action.Selector != nil {
		testStep.Action.Selector = g.selectBestSelector(step.Action.Selector)
	}

	// Credential substitution
	if credentials != nil && testStep.Action.Value != "" {
		for key, value := range credentials {
			placeholder := fmt.Sprintf("${%s}", key)
			testStep.Action.Value = strings.ReplaceAll(testStep.Action.Value, placeholder, value)
		}
	}

	// Add wait strategy
	if g.config.DefaultWaitStrategy != "" {
		testStep.Wait = &WaitConfig{
			Type:    g.config.DefaultWaitStrategy,
			Timeout: g.config.DefaultTimeout,
		}
	}

	// Convert assertions
	for _, assertion := range step.Assertions {
		testStep.Assertions = append(testStep.Assertions, g.convertAssertion(&assertion))
	}

	// Screenshot if configured
	testStep.Screenshot = g.config.IncludeScreenshots

	return testStep
}

// selectBestSelector chooses the best selector based on config.
func (g *DSLGenerator) selectBestSelector(selector *flow.Selector) string {
	if selector == nil {
		return ""
	}

	// Check preferences
	switch selector.Strategy {
	case flow.SelectorTestID:
		if g.config.PreferTestID {
			return selector.Primary
		}
	case flow.SelectorDataCy:
		if g.config.PreferDataCy {
			return selector.Primary
		}
	}

	// Use primary if confidence is high
	if selector.Confidence >= 0.8 {
		return selector.Primary
	}

	// Try fallbacks
	for _, fb := range selector.Fallbacks {
		if fb.Confidence >= 0.7 {
			return fb.Primary
		}
	}

	// Default to primary
	return selector.Primary
}

// convertAssertion converts a flow assertion to a test assertion.
func (g *DSLGenerator) convertAssertion(assertion *flow.Assertion) Assertion {
	result := Assertion{
		Type:     mapAssertionType(assertion.Type),
		Expected: assertion.Expected,
	}

	if assertion.Selector != nil {
		result.Selector = assertion.Selector.Primary
	}

	return result
}

// generateFlowTypeAssertions generates default assertions based on flow type.
func (g *DSLGenerator) generateFlowTypeAssertions(flowType flow.FlowType) []Assertion {
	var assertions []Assertion

	switch flowType {
	case flow.FlowTypeLogin:
		assertions = append(assertions, Assertion{
			Type:    AssertURL,
			Message: "Should redirect after login",
		})
	case flow.FlowTypeRegistration:
		assertions = append(assertions, Assertion{
			Type:    AssertVisible,
			Message: "Should show success message",
		})
	case flow.FlowTypeCheckout:
		assertions = append(assertions, Assertion{
			Type:    AssertURL,
			Message: "Should reach confirmation page",
		})
	case flow.FlowTypeSearch:
		assertions = append(assertions, Assertion{
			Type:    AssertVisible,
			Message: "Should show search results",
		})
	}

	return assertions
}

// generateFromAPISpec converts an API spec to E2E test specs.
func (g *DSLGenerator) generateFromAPISpec(apiSpec *api.APISpec) []*E2ETestSpec {
	spec := &E2ETestSpec{
		ID:        uuid.New().String(),
		Name:      fmt.Sprintf("API Tests: %s", apiSpec.Name),
		BaseURL:   apiSpec.BaseURL,
		TestCases: make([]TestCase, 0),
		Config:    DefaultTestConfig(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Group endpoints by resource
	resources := g.groupEndpointsByResource(apiSpec.Endpoints)

	for resource, endpoints := range resources {
		// Create test cases for each resource
		tc := g.createResourceTestCase(resource, endpoints, apiSpec.Auth)
		spec.TestCases = append(spec.TestCases, tc)

		// Generate negative tests if configured
		if g.config.GenerateNegativeTests {
			negativeTC := g.createNegativeTestCase(resource, endpoints, apiSpec.Auth)
			if len(negativeTC.Steps) > 0 {
				spec.TestCases = append(spec.TestCases, negativeTC)
			}
		}
	}

	return []*E2ETestSpec{spec}
}

// groupEndpointsByResource groups endpoints by their resource path.
func (g *DSLGenerator) groupEndpointsByResource(endpoints []api.Endpoint) map[string][]api.Endpoint {
	resources := make(map[string][]api.Endpoint)

	for _, ep := range endpoints {
		resource := g.extractResource(ep.Path)
		resources[resource] = append(resources[resource], ep)
	}

	return resources
}

// extractResource extracts the resource name from a path.
func (g *DSLGenerator) extractResource(path string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")

	// Skip common prefixes like "api", "v1", "v2"
	for i, seg := range segments {
		if seg != "api" && seg != "v1" && seg != "v2" && !strings.HasPrefix(seg, "{") {
			return seg
		}
		if i == len(segments)-1 {
			return seg
		}
	}

	return "root"
}

// createResourceTestCase creates a test case for a resource's CRUD operations.
func (g *DSLGenerator) createResourceTestCase(resource string, endpoints []api.Endpoint, auth *api.AuthRequirement) TestCase {
	tc := TestCase{
		ID:          uuid.New().String(),
		Name:        fmt.Sprintf("CRUD operations for %s", resource),
		Description: fmt.Sprintf("Test create, read, update, delete operations for %s", resource),
		Tags:        []string{"api", resource, "crud"},
		Steps:       make([]TestStep, 0),
	}

	// Sort by method to get a logical order: POST, GET, PUT/PATCH, DELETE
	sort.Slice(endpoints, func(i, j int) bool {
		order := map[api.HTTPMethod]int{
			api.MethodPOST:   1,
			api.MethodGET:    2,
			api.MethodPUT:    3,
			api.MethodPATCH:  4,
			api.MethodDELETE: 5,
		}
		return order[endpoints[i].Method] < order[endpoints[j].Method]
	})

	for i, ep := range endpoints {
		step := g.endpointToTestStep(&ep, i+1, auth)
		tc.Steps = append(tc.Steps, step)
	}

	return tc
}

// endpointToTestStep converts an endpoint to a test step.
func (g *DSLGenerator) endpointToTestStep(ep *api.Endpoint, order int, auth *api.AuthRequirement) TestStep {
	step := TestStep{
		ID:    uuid.New().String(),
		Name:  ep.Summary,
		Order: order,
		Action: TestAction{
			Type: flow.ActionTypeNavigate, // We use navigate for API calls
			URL:  ep.Path,
			Options: map[string]interface{}{
				"method":      string(ep.Method),
				"contentType": "application/json",
			},
		},
	}

	// Add auth if required
	if auth != nil && auth.Type != api.AuthTypeNone {
		step.Action.Options["auth"] = map[string]string{
			"type": string(auth.Type),
		}
	}

	// Add request body for POST/PUT/PATCH
	if ep.RequestBody != nil && ep.RequestBody.Example != nil {
		step.Action.Options["body"] = ep.RequestBody.Example
	}

	// Add assertions based on expected response
	for _, resp := range ep.Responses {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			step.Assertions = append(step.Assertions, Assertion{
				Type:     AssertStatusCode,
				Expected: resp.StatusCode,
			})
			break
		}
	}

	return step
}

// createNegativeTestCase creates negative/error test cases.
func (g *DSLGenerator) createNegativeTestCase(resource string, endpoints []api.Endpoint, auth *api.AuthRequirement) TestCase {
	tc := TestCase{
		ID:          uuid.New().String(),
		Name:        fmt.Sprintf("Error handling for %s", resource),
		Description: fmt.Sprintf("Test error scenarios for %s API", resource),
		Tags:        []string{"api", resource, "negative", "error"},
		Steps:       make([]TestStep, 0),
	}

	stepOrder := 1

	for _, ep := range endpoints {
		// Test missing required fields for POST/PUT
		if ep.Method == api.MethodPOST || ep.Method == api.MethodPUT {
			step := TestStep{
				ID:    uuid.New().String(),
				Name:  fmt.Sprintf("%s with invalid data", ep.Summary),
				Order: stepOrder,
				Action: TestAction{
					Type: flow.ActionTypeNavigate,
					URL:  ep.Path,
					Options: map[string]interface{}{
						"method": string(ep.Method),
						"body":   map[string]interface{}{}, // Empty body
					},
				},
				Assertions: []Assertion{
					{Type: AssertStatusCode, Expected: 400},
				},
			}
			tc.Steps = append(tc.Steps, step)
			stepOrder++
		}

		// Test unauthorized access if auth is required
		if auth != nil && auth.Type != api.AuthTypeNone {
			step := TestStep{
				ID:    uuid.New().String(),
				Name:  fmt.Sprintf("%s without auth", ep.Summary),
				Order: stepOrder,
				Action: TestAction{
					Type: flow.ActionTypeNavigate,
					URL:  ep.Path,
					Options: map[string]interface{}{
						"method": string(ep.Method),
						// No auth headers
					},
				},
				Assertions: []Assertion{
					{Type: AssertStatusCode, Expected: 401},
				},
			}
			tc.Steps = append(tc.Steps, step)
			stepOrder++
		}
	}

	return tc
}

// generateAuthTests creates authentication test cases.
func (g *DSLGenerator) generateAuthTests(apiSpec *api.APISpec) *E2ETestSpec {
	if apiSpec.Auth == nil || apiSpec.Auth.Type == api.AuthTypeNone {
		return nil
	}

	spec := &E2ETestSpec{
		ID:        uuid.New().String(),
		Name:      "Authentication Tests",
		BaseURL:   apiSpec.BaseURL,
		TestCases: make([]TestCase, 0),
		Config:    DefaultTestConfig(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test valid auth
	validAuthTC := TestCase{
		ID:          uuid.New().String(),
		Name:        "Valid authentication",
		Description: fmt.Sprintf("Test %s authentication with valid credentials", apiSpec.Auth.Type),
		Tags:        []string{"auth", "positive"},
		Steps: []TestStep{
			{
				ID:    uuid.New().String(),
				Name:  "Make authenticated request",
				Order: 1,
				Action: TestAction{
					Type: flow.ActionTypeNavigate,
					URL:  "/api/v1/me", // Common auth check endpoint
					Options: map[string]interface{}{
						"method": "GET",
						"auth":   map[string]string{"type": string(apiSpec.Auth.Type)},
					},
				},
				Assertions: []Assertion{
					{Type: AssertStatusCode, Expected: 200},
				},
			},
		},
	}
	spec.TestCases = append(spec.TestCases, validAuthTC)

	// Test invalid auth
	invalidAuthTC := TestCase{
		ID:          uuid.New().String(),
		Name:        "Invalid authentication",
		Description: "Test rejection of invalid credentials",
		Tags:        []string{"auth", "negative"},
		Steps: []TestStep{
			{
				ID:    uuid.New().String(),
				Name:  "Make request with invalid credentials",
				Order: 1,
				Action: TestAction{
					Type: flow.ActionTypeNavigate,
					URL:  "/api/v1/me",
					Options: map[string]interface{}{
						"method": "GET",
						"headers": map[string]string{
							"Authorization": "Bearer invalid-token",
						},
					},
				},
				Assertions: []Assertion{
					{Type: AssertStatusCode, Expected: 401},
				},
			},
		},
	}
	spec.TestCases = append(spec.TestCases, invalidAuthTC)

	// Test missing auth
	missingAuthTC := TestCase{
		ID:          uuid.New().String(),
		Name:        "Missing authentication",
		Description: "Test rejection of requests without auth",
		Tags:        []string{"auth", "negative"},
		Steps: []TestStep{
			{
				ID:    uuid.New().String(),
				Name:  "Make request without credentials",
				Order: 1,
				Action: TestAction{
					Type: flow.ActionTypeNavigate,
					URL:  "/api/v1/me",
					Options: map[string]interface{}{
						"method": "GET",
					},
				},
				Assertions: []Assertion{
					{Type: AssertStatusCode, Expected: 401},
				},
			},
		},
	}
	spec.TestCases = append(spec.TestCases, missingAuthTC)

	return spec
}

// generateSuggestions provides suggestions for improving test coverage.
func (g *DSLGenerator) generateSuggestions(input *GenerationInput, output *GenerationOutput) []string {
	var suggestions []string

	// Check for missing flow types
	hasLogin := false
	hasCheckout := false
	for _, f := range input.Flows {
		if f.Type == flow.FlowTypeLogin {
			hasLogin = true
		}
		if f.Type == flow.FlowTypeCheckout {
			hasCheckout = true
		}
	}

	if !hasLogin && input.APISpec != nil && input.APISpec.Auth != nil {
		suggestions = append(suggestions, "Consider adding login flow tests for authenticated endpoints")
	}

	if !hasCheckout && len(input.Flows) > 0 {
		// Check if there are form submissions that might be checkout
		for _, f := range input.Flows {
			if f.Type == flow.FlowTypeFormSubmit {
				suggestions = append(suggestions, "Consider identifying checkout flows for e-commerce testing")
				break
			}
		}
	}

	// Check test coverage
	if output.TotalTests < 5 {
		suggestions = append(suggestions, "Consider adding more test cases for comprehensive coverage")
	}

	// Check for API test coverage
	if input.APISpec != nil && output.Coverage.EndpointsCovered < len(input.APISpec.Endpoints) {
		suggestions = append(suggestions, fmt.Sprintf("Only %d of %d endpoints have tests",
			output.Coverage.EndpointsCovered, len(input.APISpec.Endpoints)))
	}

	return suggestions
}

// GenerateWithLLM uses an LLM to enhance the generated tests.
func (g *DSLGenerator) GenerateWithLLM(ctx context.Context, input *GenerationInput, enhancer *LLMEnhancer) (*GenerationOutput, error) {
	// First generate base tests
	output, err := g.Generate(input)
	if err != nil {
		return nil, err
	}

	// Enhance with LLM if available
	if enhancer != nil {
		for _, spec := range output.Specs {
			result, err := enhancer.Enhance(ctx, spec)
			if err != nil {
				output.Warnings = append(output.Warnings, fmt.Sprintf("LLM enhancement failed: %v", err))
				continue
			}
			// Apply enhancements
			spec.TestCases = append(spec.TestCases, result.SuggestedTests...)
		}
	}

	return output, nil
}
