package contract

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/internal/datagen"
)

// ContractTestGenerator generates contract tests
type ContractTestGenerator struct {
	dataGen *datagen.SchemaGenerator
}

// NewContractTestGenerator creates a contract test generator
func NewContractTestGenerator() *ContractTestGenerator {
	return &ContractTestGenerator{
		dataGen: datagen.NewSchemaGenerator(),
	}
}

// GenerateTests generates contract test code from a contract
func (ctg *ContractTestGenerator) GenerateTests(contract *Contract, language string) string {
	switch language {
	case "javascript", "typescript":
		return ctg.generateJestContractTests(contract)
	case "python":
		return ctg.generatePytestContractTests(contract)
	case "go":
		return ctg.generateGoContractTests(contract)
	default:
		return ctg.generateJestContractTests(contract)
	}
}

func (ctg *ContractTestGenerator) generateJestContractTests(contract *Contract) string {
	var sb strings.Builder

	sb.WriteString("const request = require('supertest');\n")
	sb.WriteString("const app = require('./app');\n")
	sb.WriteString("const Ajv = require('ajv');\n")
	sb.WriteString("const ajv = new Ajv();\n\n")

	sb.WriteString(fmt.Sprintf("/**\n * Contract Tests for %s\n * Version: %s\n */\n\n", contract.Name, contract.Version))

	sb.WriteString("describe('Contract Tests', () => {\n")

	for _, ep := range contract.Endpoints {
		sb.WriteString(ctg.generateJestEndpointContractTest(ep))
	}

	sb.WriteString("});\n")

	return sb.String()
}

func (ctg *ContractTestGenerator) generateJestEndpointContractTest(ep EndpointContract) string {
	var sb strings.Builder

	// Convert path params for testing
	testPath := ep.Path
	for paramName := range ep.Request.PathParams {
		testPath = strings.ReplaceAll(testPath, ":"+paramName, "123")
		testPath = strings.ReplaceAll(testPath, "{"+paramName+"}", "123")
	}

	sb.WriteString(fmt.Sprintf("\n  describe('%s %s', () => {\n", ep.Method, ep.Path))

	// Test: Status code
	sb.WriteString(fmt.Sprintf("    test('should return status %d', async () => {\n", ep.Response.StatusCode))
	sb.WriteString(fmt.Sprintf("      const response = await request(app)\n"))
	sb.WriteString(fmt.Sprintf("        .%s('%s')", strings.ToLower(ep.Method), testPath))

	// Add headers
	for headerName := range ep.Request.Headers {
		sb.WriteString(fmt.Sprintf("\n        .set('%s', 'test-value')", headerName))
	}

	// Add body for POST/PUT/PATCH
	if ep.Request.Body != nil && (ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH") {
		sb.WriteString("\n        .send({ test: 'data' })")
	}

	sb.WriteString(";\n\n")
	sb.WriteString(fmt.Sprintf("      expect(response.status).toBe(%d);\n", ep.Response.StatusCode))
	sb.WriteString("    });\n\n")

	// Test: Response content type
	if ep.Response.ContentType != "" {
		sb.WriteString("    test('should return correct content-type', async () => {\n")
		sb.WriteString(fmt.Sprintf("      const response = await request(app)\n"))
		sb.WriteString(fmt.Sprintf("        .%s('%s')", strings.ToLower(ep.Method), testPath))
		for headerName := range ep.Request.Headers {
			sb.WriteString(fmt.Sprintf("\n        .set('%s', 'test-value')", headerName))
		}
		if ep.Request.Body != nil && (ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH") {
			sb.WriteString("\n        .send({ test: 'data' })")
		}
		sb.WriteString(";\n\n")
		sb.WriteString(fmt.Sprintf("      expect(response.headers['content-type']).toMatch(/%s/);\n", strings.ReplaceAll(ep.Response.ContentType, "/", "\\/")))
		sb.WriteString("    });\n\n")
	}

	// Test: Response body schema
	if ep.Response.Body != nil {
		sb.WriteString("    test('should match response schema', async () => {\n")
		sb.WriteString(fmt.Sprintf("      const response = await request(app)\n"))
		sb.WriteString(fmt.Sprintf("        .%s('%s')", strings.ToLower(ep.Method), testPath))
		for headerName := range ep.Request.Headers {
			sb.WriteString(fmt.Sprintf("\n        .set('%s', 'test-value')", headerName))
		}
		if ep.Request.Body != nil && (ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH") {
			sb.WriteString("\n        .send({ test: 'data' })")
		}
		sb.WriteString(";\n\n")

		// Generate schema validator
		schemaJSON, _ := json.MarshalIndent(ep.Response.Body, "      ", "  ")
		sb.WriteString(fmt.Sprintf("      const schema = %s;\n", string(schemaJSON)))
		sb.WriteString("      const validate = ajv.compile(schema);\n")
		sb.WriteString("      const valid = validate(response.body);\n")
		sb.WriteString("      expect(valid).toBe(true);\n")
		sb.WriteString("    });\n\n")
	}

	// Test: Required headers
	for headerName, spec := range ep.Request.Headers {
		if spec.Required {
			sb.WriteString(fmt.Sprintf("    test('should require %s header', async () => {\n", headerName))
			sb.WriteString(fmt.Sprintf("      const response = await request(app)\n"))
			sb.WriteString(fmt.Sprintf("        .%s('%s');\n\n", strings.ToLower(ep.Method), testPath))
			sb.WriteString("      expect(response.status).toBe(401); // or 400\n")
			sb.WriteString("    });\n\n")
		}
	}

	sb.WriteString("  });\n")

	return sb.String()
}

func (ctg *ContractTestGenerator) generatePytestContractTests(contract *Contract) string {
	var sb strings.Builder

	sb.WriteString("import pytest\n")
	sb.WriteString("from fastapi.testclient import TestClient\n")
	sb.WriteString("from jsonschema import validate, ValidationError\n")
	sb.WriteString("from main import app\n\n")

	sb.WriteString("client = TestClient(app)\n\n")

	sb.WriteString(fmt.Sprintf("\"\"\"Contract Tests for %s - Version %s\"\"\"\n\n", contract.Name, contract.Version))

	for _, ep := range contract.Endpoints {
		sb.WriteString(ctg.generatePytestEndpointContractTest(ep))
	}

	return sb.String()
}

func (ctg *ContractTestGenerator) generatePytestEndpointContractTest(ep EndpointContract) string {
	var sb strings.Builder

	// Convert path params
	testPath := ep.Path
	for paramName := range ep.Request.PathParams {
		testPath = strings.ReplaceAll(testPath, ":"+paramName, "123")
		testPath = strings.ReplaceAll(testPath, "{"+paramName+"}", "123")
	}

	funcName := strings.ToLower(ep.Method) + "_" + strings.ReplaceAll(strings.ReplaceAll(testPath, "/", "_"), ":", "")
	funcName = strings.ReplaceAll(funcName, "__", "_")
	funcName = strings.Trim(funcName, "_")

	// Test: Status code
	sb.WriteString(fmt.Sprintf("\ndef test_contract_%s_status():\n", funcName))
	sb.WriteString(fmt.Sprintf("    \"\"\"Contract: %s %s should return %d\"\"\"\n", ep.Method, ep.Path, ep.Response.StatusCode))

	headers := "headers={}"
	if len(ep.Request.Headers) > 0 {
		headers = "headers={"
		for h := range ep.Request.Headers {
			headers += fmt.Sprintf("'%s': 'test-value', ", h)
		}
		headers += "}"
	}

	switch ep.Method {
	case "GET":
		sb.WriteString(fmt.Sprintf("    response = client.get('%s', %s)\n", testPath, headers))
	case "POST":
		sb.WriteString(fmt.Sprintf("    response = client.post('%s', json={'test': 'data'}, %s)\n", testPath, headers))
	case "PUT":
		sb.WriteString(fmt.Sprintf("    response = client.put('%s', json={'test': 'data'}, %s)\n", testPath, headers))
	case "DELETE":
		sb.WriteString(fmt.Sprintf("    response = client.delete('%s', %s)\n", testPath, headers))
	}

	sb.WriteString(fmt.Sprintf("    assert response.status_code == %d\n\n", ep.Response.StatusCode))

	// Test: Response schema
	if ep.Response.Body != nil {
		sb.WriteString(fmt.Sprintf("\ndef test_contract_%s_schema():\n", funcName))
		sb.WriteString(fmt.Sprintf("    \"\"\"Contract: %s %s response should match schema\"\"\"\n", ep.Method, ep.Path))

		switch ep.Method {
		case "GET":
			sb.WriteString(fmt.Sprintf("    response = client.get('%s', %s)\n", testPath, headers))
		case "POST":
			sb.WriteString(fmt.Sprintf("    response = client.post('%s', json={'test': 'data'}, %s)\n", testPath, headers))
		case "PUT":
			sb.WriteString(fmt.Sprintf("    response = client.put('%s', json={'test': 'data'}, %s)\n", testPath, headers))
		}

		schemaJSON, _ := json.MarshalIndent(ep.Response.Body, "    ", "  ")
		sb.WriteString(fmt.Sprintf("    schema = %s\n", string(schemaJSON)))
		sb.WriteString("    try:\n")
		sb.WriteString("        validate(instance=response.json(), schema=schema)\n")
		sb.WriteString("    except ValidationError as e:\n")
		sb.WriteString("        pytest.fail(f'Schema validation failed: {e.message}')\n\n")
	}

	return sb.String()
}

func (ctg *ContractTestGenerator) generateGoContractTests(contract *Contract) string {
	var sb strings.Builder

	sb.WriteString("package contract_test\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"encoding/json\"\n")
	sb.WriteString("\t\"net/http\"\n")
	sb.WriteString("\t\"net/http/httptest\"\n")
	sb.WriteString("\t\"testing\"\n")
	sb.WriteString("\n")
	sb.WriteString("\t\"github.com/stretchr/testify/assert\"\n")
	sb.WriteString(")\n\n")

	sb.WriteString(fmt.Sprintf("// Contract Tests for %s - Version %s\n\n", contract.Name, contract.Version))

	for _, ep := range contract.Endpoints {
		sb.WriteString(ctg.generateGoEndpointContractTest(ep))
	}

	return sb.String()
}

func (ctg *ContractTestGenerator) generateGoEndpointContractTest(ep EndpointContract) string {
	var sb strings.Builder

	// Convert path params
	testPath := ep.Path
	for paramName := range ep.Request.PathParams {
		testPath = strings.ReplaceAll(testPath, ":"+paramName, "123")
		testPath = strings.ReplaceAll(testPath, "{"+paramName+"}", "123")
	}

	funcName := "TestContract_" + ep.Method + "_" + strings.ReplaceAll(strings.ReplaceAll(testPath, "/", "_"), ":", "")
	funcName = strings.ReplaceAll(funcName, "__", "_")
	funcName = strings.Trim(funcName, "_")

	sb.WriteString(fmt.Sprintf("func %s_Status(t *testing.T) {\n", funcName))
	sb.WriteString("\t// Contract: Should return expected status code\n")
	sb.WriteString(fmt.Sprintf("\treq, _ := http.NewRequest(\"%s\", \"%s\", nil)\n", ep.Method, testPath))

	for h := range ep.Request.Headers {
		sb.WriteString(fmt.Sprintf("\treq.Header.Set(\"%s\", \"test-value\")\n", h))
	}

	sb.WriteString("\trr := httptest.NewRecorder()\n")
	sb.WriteString("\thandler.ServeHTTP(rr, req)\n\n")
	sb.WriteString(fmt.Sprintf("\tassert.Equal(t, %d, rr.Code)\n", ep.Response.StatusCode))
	sb.WriteString("}\n\n")

	// Test: Response body structure
	if ep.Response.Body != nil {
		sb.WriteString(fmt.Sprintf("func %s_Schema(t *testing.T) {\n", funcName))
		sb.WriteString("\t// Contract: Response should have expected structure\n")
		sb.WriteString(fmt.Sprintf("\treq, _ := http.NewRequest(\"%s\", \"%s\", nil)\n", ep.Method, testPath))

		for h := range ep.Request.Headers {
			sb.WriteString(fmt.Sprintf("\treq.Header.Set(\"%s\", \"test-value\")\n", h))
		}

		sb.WriteString("\trr := httptest.NewRecorder()\n")
		sb.WriteString("\thandler.ServeHTTP(rr, req)\n\n")
		sb.WriteString("\tvar response map[string]interface{}\n")
		sb.WriteString("\terr := json.Unmarshal(rr.Body.Bytes(), &response)\n")
		sb.WriteString("\tassert.NoError(t, err)\n")
		sb.WriteString("\t// Add schema validation here\n")
		sb.WriteString("}\n\n")
	}

	return sb.String()
}
