package e2e

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/QTest-hq/qtest/internal/api"
	"github.com/QTest-hq/qtest/internal/flow"
)

// APITestGenerator generates API tests from endpoint specifications.
type APITestGenerator struct {
	config *GenerationConfig
}

// NewAPITestGenerator creates a new API test generator.
func NewAPITestGenerator(config *GenerationConfig) *APITestGenerator {
	if config == nil {
		config = DefaultGenerationConfig()
	}
	return &APITestGenerator{config: config}
}

// GenerateFromSpec generates tests from an API specification.
func (g *APITestGenerator) GenerateFromSpec(apiSpec *api.APISpec) (*GenerationResult, error) {
	result := &GenerationResult{}

	if len(apiSpec.Endpoints) == 0 {
		result.Warnings = append(result.Warnings, "No endpoints to generate tests for")
		return result, nil
	}

	// Convert API spec to E2E test spec
	e2eSpec := g.apiSpecToE2ESpec(apiSpec)

	// Generate Playwright API tests
	testCode := g.generateAPITestFile(e2eSpec, apiSpec)
	fileName := g.generateFileName(apiSpec.Name)

	result.Files = append(result.Files, GeneratedFile{
		Path:      fmt.Sprintf("%s/%s", g.config.OutputDir, fileName),
		Content:   testCode,
		Language:  string(g.config.Language),
		TestCount: len(e2eSpec.TestCases),
	})
	result.TestCount = len(e2eSpec.TestCases)
	result.StepCount = result.TestCount // One request per test for API tests

	return result, nil
}

// GenerateFromEndpoint generates a test from a single endpoint.
func (g *APITestGenerator) GenerateFromEndpoint(endpoint *api.Endpoint, baseURL string, auth *api.AuthRequirement) (*E2ETestSpec, error) {
	spec := &E2ETestSpec{
		ID:        uuid.New().String(),
		Name:      fmt.Sprintf("API: %s", endpoint.Summary),
		BaseURL:   baseURL,
		TestCases: make([]TestCase, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Generate test cases for the endpoint
	testCases := g.generateEndpointTests(endpoint, auth)
	spec.TestCases = append(spec.TestCases, testCases...)

	return spec, nil
}

func (g *APITestGenerator) apiSpecToE2ESpec(apiSpec *api.APISpec) *E2ETestSpec {
	spec := &E2ETestSpec{
		ID:        uuid.New().String(),
		Name:      fmt.Sprintf("API Tests: %s", apiSpec.Name),
		BaseURL:   apiSpec.BaseURL,
		TestCases: make([]TestCase, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	for _, endpoint := range apiSpec.Endpoints {
		testCases := g.generateEndpointTests(&endpoint, apiSpec.Auth)
		spec.TestCases = append(spec.TestCases, testCases...)
	}

	return spec
}

func (g *APITestGenerator) generateEndpointTests(endpoint *api.Endpoint, auth *api.AuthRequirement) []TestCase {
	var testCases []TestCase

	// Happy path test
	happyPath := TestCase{
		ID:          uuid.New().String(),
		Name:        fmt.Sprintf("%s %s - Success", endpoint.Method, endpoint.Path),
		Description: endpoint.Description,
		Tags:        endpoint.Tags,
		Steps: []TestStep{
			{
				ID:    uuid.New().String(),
				Name:  "Make API request",
				Order: 1,
				Action: TestAction{
					Type: flow.ActionTypeNavigate, // We'll use custom API action in generation
					URL:  endpoint.Path,
				},
			},
		},
	}

	// Add success assertion
	for _, resp := range endpoint.Responses {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			happyPath.Expected = append(happyPath.Expected, Assertion{
				Type:     AssertStatusCode,
				Expected: resp.StatusCode,
			})
			break
		}
	}

	testCases = append(testCases, happyPath)

	// Error path tests for validation
	if endpoint.Method == api.MethodPOST || endpoint.Method == api.MethodPUT {
		errorTest := TestCase{
			ID:          uuid.New().String(),
			Name:        fmt.Sprintf("%s %s - Invalid Input", endpoint.Method, endpoint.Path),
			Description: "Test validation with invalid input",
			Tags:        append(endpoint.Tags, "negative"),
			Steps: []TestStep{
				{
					ID:    uuid.New().String(),
					Name:  "Make API request with invalid data",
					Order: 1,
					Action: TestAction{
						Type: flow.ActionTypeNavigate,
						URL:  endpoint.Path,
					},
				},
			},
			Expected: []Assertion{
				{
					Type:     AssertStatusCode,
					Expected: 400, // Bad Request
				},
			},
		}
		testCases = append(testCases, errorTest)
	}

	// Auth test if authentication is required
	if auth != nil && auth.Type != api.AuthTypeNone {
		authTest := TestCase{
			ID:          uuid.New().String(),
			Name:        fmt.Sprintf("%s %s - Unauthorized", endpoint.Method, endpoint.Path),
			Description: "Test without authentication",
			Tags:        append(endpoint.Tags, "auth", "negative"),
			Steps: []TestStep{
				{
					ID:    uuid.New().String(),
					Name:  "Make API request without auth",
					Order: 1,
					Action: TestAction{
						Type: flow.ActionTypeNavigate,
						URL:  endpoint.Path,
					},
				},
			},
			Expected: []Assertion{
				{
					Type:     AssertStatusCode,
					Expected: 401, // Unauthorized
				},
			},
		}
		testCases = append(testCases, authTest)
	}

	return testCases
}

func (g *APITestGenerator) generateAPITestFile(spec *E2ETestSpec, apiSpec *api.APISpec) string {
	var sb strings.Builder

	// Imports
	sb.WriteString(g.generateAPIImports())
	sb.WriteString("\n\n")

	// Base URL constant
	sb.WriteString(fmt.Sprintf("const BASE_URL = '%s';\n\n", apiSpec.BaseURL))

	// Auth helper if needed
	if apiSpec.Auth != nil {
		sb.WriteString(g.generateAuthHelper(apiSpec.Auth))
		sb.WriteString("\n")
	}

	// Test suite
	sb.WriteString(fmt.Sprintf("test.describe('API Tests: %s', () => {\n", escapeString(apiSpec.Name)))

	// Generate tests for each endpoint
	for _, endpoint := range apiSpec.Endpoints {
		sb.WriteString(g.generateEndpointTest(&endpoint, apiSpec.Auth))
		sb.WriteString("\n")
	}

	sb.WriteString("});\n")

	return sb.String()
}

func (g *APITestGenerator) generateAPIImports() string {
	if g.config.Language == LanguageTypeScript {
		return `import { test, expect, APIRequestContext, request } from '@playwright/test';`
	}
	return `const { test, expect, request } = require('@playwright/test');`
}

func (g *APITestGenerator) generateAuthHelper(auth *api.AuthRequirement) string {
	var sb strings.Builder

	switch auth.Type {
	case api.AuthTypeBearer:
		sb.WriteString(`// Configure Bearer token authentication
function getAuthHeaders(token: string) {
  return {
    'Authorization': ` + "`Bearer ${token}`" + `,
  };
}
`)
	case api.AuthTypeAPIKey:
		sb.WriteString(fmt.Sprintf(`// Configure API key authentication
function getAuthHeaders(apiKey: string) {
  return {
    '%s': apiKey,
  };
}
`, auth.Name))
	case api.AuthTypeBasic:
		sb.WriteString(`// Configure Basic authentication
function getAuthHeaders(username: string, password: string) {
  const credentials = Buffer.from(` + "`${username}:${password}`" + `).toString('base64');
  return {
    'Authorization': ` + "`Basic ${credentials}`" + `,
  };
}
`)
	}

	return sb.String()
}

func (g *APITestGenerator) generateEndpointTest(endpoint *api.Endpoint, auth *api.AuthRequirement) string {
	var sb strings.Builder

	// Test for the endpoint
	methodLower := strings.ToLower(string(endpoint.Method))
	testName := fmt.Sprintf("%s %s", endpoint.Method, endpoint.Path)

	sb.WriteString(fmt.Sprintf("  test('%s', async ({ request }) => {\n", escapeString(testName)))

	// Build request options
	sb.WriteString(g.generateRequestCode(endpoint, auth))

	// Make the request
	sb.WriteString(fmt.Sprintf("    const response = await request.%s(`${BASE_URL}%s`%s);\n",
		methodLower, endpoint.Path, g.getRequestOptions(endpoint)))
	sb.WriteString("\n")

	// Add assertions
	sb.WriteString(g.generateResponseAssertions(endpoint))

	sb.WriteString("  });\n")

	return sb.String()
}

func (g *APITestGenerator) generateRequestCode(endpoint *api.Endpoint, auth *api.AuthRequirement) string {
	var sb strings.Builder

	// Build headers
	hasHeaders := false
	if auth != nil && auth.Type != api.AuthTypeNone {
		hasHeaders = true
	}

	// Build request body for POST/PUT/PATCH
	needsBody := endpoint.Method == api.MethodPOST ||
		endpoint.Method == api.MethodPUT ||
		endpoint.Method == api.MethodPATCH

	if hasHeaders || needsBody {
		sb.WriteString("    const options = {\n")

		if hasHeaders {
			sb.WriteString("      headers: {\n")
			switch auth.Type {
			case api.AuthTypeBearer:
				sb.WriteString("        'Authorization': `Bearer ${process.env.API_TOKEN}`,\n")
			case api.AuthTypeAPIKey:
				sb.WriteString(fmt.Sprintf("        '%s': process.env.API_KEY,\n", auth.Name))
			}
			sb.WriteString("      },\n")
		}

		if needsBody && endpoint.RequestBody != nil {
			body := g.generateSampleBody(endpoint.RequestBody)
			sb.WriteString(fmt.Sprintf("      data: %s,\n", body))
		}

		sb.WriteString("    };\n\n")
	}

	return sb.String()
}

func (g *APITestGenerator) getRequestOptions(endpoint *api.Endpoint) string {
	needsOptions := endpoint.Method == api.MethodPOST ||
		endpoint.Method == api.MethodPUT ||
		endpoint.Method == api.MethodPATCH ||
		endpoint.RequestBody != nil

	if needsOptions {
		return ", options"
	}
	return ""
}

func (g *APITestGenerator) generateSampleBody(reqBody *api.RequestBody) string {
	if reqBody.Example != nil {
		if jsonBytes, err := json.MarshalIndent(reqBody.Example, "      ", "  "); err == nil {
			return string(jsonBytes)
		}
	}

	if reqBody.Schema != nil {
		return g.schemaToSampleJSON(reqBody.Schema)
	}

	return "{}"
}

func (g *APITestGenerator) schemaToSampleJSON(schema *api.Schema) string {
	sample := make(map[string]interface{})

	if schema.Properties != nil {
		for name, prop := range schema.Properties {
			sample[name] = g.generateSampleValue(prop)
		}
	}

	if jsonBytes, err := json.MarshalIndent(sample, "      ", "  "); err == nil {
		return string(jsonBytes)
	}
	return "{}"
}

func (g *APITestGenerator) generateSampleValue(schema *api.Schema) interface{} {
	if schema.Example != nil {
		return schema.Example
	}

	switch schema.Type {
	case api.DataTypeString:
		if schema.Format == "email" {
			return "test@example.com"
		}
		if schema.Format == "uuid" {
			return "550e8400-e29b-41d4-a716-446655440000"
		}
		if schema.Format == "date-time" {
			return time.Now().Format(time.RFC3339)
		}
		return "string"
	case api.DataTypeInteger:
		return 1
	case api.DataTypeNumber:
		return 1.0
	case api.DataTypeBoolean:
		return true
	case api.DataTypeArray:
		if schema.Items != nil {
			return []interface{}{g.generateSampleValue(schema.Items)}
		}
		return []interface{}{}
	case api.DataTypeObject:
		if schema.Properties != nil {
			obj := make(map[string]interface{})
			for name, prop := range schema.Properties {
				obj[name] = g.generateSampleValue(prop)
			}
			return obj
		}
		return map[string]interface{}{}
	default:
		return "value"
	}
}

func (g *APITestGenerator) generateResponseAssertions(endpoint *api.Endpoint) string {
	var sb strings.Builder

	// Assert status code
	expectedStatus := 200
	for _, resp := range endpoint.Responses {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			expectedStatus = resp.StatusCode
			break
		}
	}

	sb.WriteString(fmt.Sprintf("    expect(response.status()).toBe(%d);\n", expectedStatus))

	// Assert response body structure if we have a schema
	for _, resp := range endpoint.Responses {
		if resp.StatusCode == expectedStatus && resp.Schema != nil {
			sb.WriteString("    const body = await response.json();\n")
			sb.WriteString(g.generateSchemaAssertions(resp.Schema, "body"))
			break
		}
	}

	return sb.String()
}

func (g *APITestGenerator) generateSchemaAssertions(schema *api.Schema, varName string) string {
	var sb strings.Builder

	if schema.Type == api.DataTypeArray {
		sb.WriteString(fmt.Sprintf("    expect(Array.isArray(%s)).toBe(true);\n", varName))
	} else if schema.Type == api.DataTypeObject && schema.Properties != nil {
		for propName := range schema.Properties {
			sb.WriteString(fmt.Sprintf("    expect(%s).toHaveProperty('%s');\n", varName, propName))
		}
	}

	return sb.String()
}

func (g *APITestGenerator) generateFileName(name string) string {
	// Convert name to kebab-case
	name = strings.ReplaceAll(strings.ToLower(name), " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	ext := "ts"
	if g.config.Language == LanguageJavaScript {
		ext = "js"
	}

	return fmt.Sprintf("api-%s.spec.%s", name, ext)
}
