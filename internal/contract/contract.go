package contract

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
)

// Contract represents an API contract
type Contract struct {
	Name      string             `json:"name"`
	Version   string             `json:"version"`
	Provider  string             `json:"provider"` // The API provider
	Consumer  string             `json:"consumer"` // The API consumer
	Endpoints []EndpointContract `json:"endpoints"`
}

// EndpointContract defines the contract for a single endpoint
type EndpointContract struct {
	ID          string               `json:"id"`
	Method      string               `json:"method"`
	Path        string               `json:"path"`
	Description string               `json:"description,omitempty"`
	Request     RequestContract      `json:"request"`
	Response    ResponseContract     `json:"response"`
	Examples    []InteractionExample `json:"examples,omitempty"`
}

// RequestContract defines expected request structure
type RequestContract struct {
	Headers     map[string]HeaderSpec `json:"headers,omitempty"`
	Query       map[string]ParamSpec  `json:"query,omitempty"`
	PathParams  map[string]ParamSpec  `json:"path_params,omitempty"`
	Body        *SchemaSpec           `json:"body,omitempty"`
	ContentType string                `json:"content_type,omitempty"`
}

// ResponseContract defines expected response structure
type ResponseContract struct {
	StatusCode  int                   `json:"status_code"`
	Headers     map[string]HeaderSpec `json:"headers,omitempty"`
	Body        *SchemaSpec           `json:"body,omitempty"`
	ContentType string                `json:"content_type,omitempty"`
}

// HeaderSpec defines header expectations
type HeaderSpec struct {
	Required bool     `json:"required"`
	Values   []string `json:"values,omitempty"` // Allowed values
}

// ParamSpec defines parameter expectations
type ParamSpec struct {
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Format   string   `json:"format,omitempty"`
	Values   []string `json:"values,omitempty"` // Allowed values
}

// SchemaSpec defines body schema
type SchemaSpec struct {
	Type       string                 `json:"type"`
	Properties map[string]*SchemaSpec `json:"properties,omitempty"`
	Items      *SchemaSpec            `json:"items,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Format     string                 `json:"format,omitempty"`
	Enum       []interface{}          `json:"enum,omitempty"`
}

// InteractionExample provides concrete examples
type InteractionExample struct {
	Name     string                 `json:"name"`
	Request  map[string]interface{} `json:"request"`
	Response map[string]interface{} `json:"response"`
}

// ContractGenerator generates contracts from SystemModel
type ContractGenerator struct{}

// NewContractGenerator creates a contract generator
func NewContractGenerator() *ContractGenerator {
	return &ContractGenerator{}
}

// GenerateFromModel generates contracts from a SystemModel
func (cg *ContractGenerator) GenerateFromModel(sysModel *model.SystemModel) *Contract {
	contract := &Contract{
		Name:      sysModel.Repository + " API Contract",
		Version:   "1.0.0",
		Provider:  sysModel.Repository,
		Endpoints: make([]EndpointContract, 0),
	}

	for _, ep := range sysModel.Endpoints {
		endpointContract := cg.generateEndpointContract(ep)
		contract.Endpoints = append(contract.Endpoints, endpointContract)
	}

	return contract
}

func (cg *ContractGenerator) generateEndpointContract(ep model.Endpoint) EndpointContract {
	ec := EndpointContract{
		ID:          ep.ID,
		Method:      ep.Method,
		Path:        ep.Path,
		Description: ep.Handler,
		Request:     cg.inferRequestContract(ep),
		Response:    cg.inferResponseContract(ep),
	}

	return ec
}

func (cg *ContractGenerator) inferRequestContract(ep model.Endpoint) RequestContract {
	rc := RequestContract{
		Headers:    make(map[string]HeaderSpec),
		Query:      make(map[string]ParamSpec),
		PathParams: make(map[string]ParamSpec),
	}

	// Infer path parameters from path pattern (e.g., /users/:id)
	pathParts := strings.Split(ep.Path, "/")
	for _, part := range pathParts {
		if strings.HasPrefix(part, ":") {
			paramName := strings.TrimPrefix(part, ":")
			rc.PathParams[paramName] = ParamSpec{
				Type:     "string",
				Required: true,
			}
		}
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			paramName := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			rc.PathParams[paramName] = ParamSpec{
				Type:     "string",
				Required: true,
			}
		}
	}

	// Infer body for POST/PUT/PATCH
	if ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH" {
		rc.ContentType = "application/json"
		rc.Body = &SchemaSpec{Type: "object"}
	}

	// Common headers
	if strings.Contains(ep.Handler, "auth") || strings.Contains(strings.ToLower(ep.Handler), "protected") {
		rc.Headers["Authorization"] = HeaderSpec{Required: true}
	}

	return rc
}

func (cg *ContractGenerator) inferResponseContract(ep model.Endpoint) ResponseContract {
	rc := ResponseContract{
		Headers:     make(map[string]HeaderSpec),
		ContentType: "application/json",
	}

	// Infer status code from method
	switch ep.Method {
	case "POST":
		rc.StatusCode = http.StatusCreated
	case "DELETE":
		rc.StatusCode = http.StatusNoContent
	default:
		rc.StatusCode = http.StatusOK
	}

	// Basic response body
	rc.Body = &SchemaSpec{Type: "object"}

	return rc
}

// ContractValidator validates API responses against contracts
type ContractValidator struct{}

// NewContractValidator creates a validator
func NewContractValidator() *ContractValidator {
	return &ContractValidator{}
}

// ValidationResult holds validation results
type ValidationResult struct {
	Valid      bool                `json:"valid"`
	Endpoint   string              `json:"endpoint"`
	Violations []ContractViolation `json:"violations,omitempty"`
}

// ContractViolation describes a contract violation
type ContractViolation struct {
	Type     string `json:"type"` // "status", "header", "body", "schema"
	Path     string `json:"path"` // JSON path to violation
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Message  string `json:"message"`
}

// ValidateResponse validates an HTTP response against a contract
func (cv *ContractValidator) ValidateResponse(contract EndpointContract, statusCode int, headers http.Header, body []byte) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Endpoint: fmt.Sprintf("%s %s", contract.Method, contract.Path),
	}

	// Validate status code
	if statusCode != contract.Response.StatusCode {
		result.Valid = false
		result.Violations = append(result.Violations, ContractViolation{
			Type:     "status",
			Expected: fmt.Sprintf("%d", contract.Response.StatusCode),
			Actual:   fmt.Sprintf("%d", statusCode),
			Message:  "Status code mismatch",
		})
	}

	// Validate headers
	for headerName, spec := range contract.Response.Headers {
		headerValue := headers.Get(headerName)
		if spec.Required && headerValue == "" {
			result.Valid = false
			result.Violations = append(result.Violations, ContractViolation{
				Type:     "header",
				Path:     headerName,
				Expected: "present",
				Actual:   "missing",
				Message:  fmt.Sprintf("Required header '%s' is missing", headerName),
			})
		}
	}

	// Validate body schema
	if contract.Response.Body != nil && len(body) > 0 {
		var bodyData interface{}
		if err := json.Unmarshal(body, &bodyData); err != nil {
			result.Valid = false
			result.Violations = append(result.Violations, ContractViolation{
				Type:    "body",
				Message: fmt.Sprintf("Invalid JSON: %s", err.Error()),
			})
		} else {
			violations := cv.validateSchema("$", contract.Response.Body, bodyData)
			if len(violations) > 0 {
				result.Valid = false
				result.Violations = append(result.Violations, violations...)
			}
		}
	}

	return result
}

func (cv *ContractValidator) validateSchema(path string, schema *SchemaSpec, data interface{}) []ContractViolation {
	var violations []ContractViolation

	if data == nil {
		return violations
	}

	switch schema.Type {
	case "object":
		obj, ok := data.(map[string]interface{})
		if !ok {
			violations = append(violations, ContractViolation{
				Type:     "schema",
				Path:     path,
				Expected: "object",
				Actual:   fmt.Sprintf("%T", data),
				Message:  "Expected object type",
			})
			return violations
		}

		// Check required fields
		for _, reqField := range schema.Required {
			if _, exists := obj[reqField]; !exists {
				violations = append(violations, ContractViolation{
					Type:     "schema",
					Path:     path + "." + reqField,
					Expected: "present",
					Actual:   "missing",
					Message:  fmt.Sprintf("Required field '%s' is missing", reqField),
				})
			}
		}

		// Validate properties
		for propName, propSchema := range schema.Properties {
			if propData, exists := obj[propName]; exists {
				propViolations := cv.validateSchema(path+"."+propName, propSchema, propData)
				violations = append(violations, propViolations...)
			}
		}

	case "array":
		arr, ok := data.([]interface{})
		if !ok {
			violations = append(violations, ContractViolation{
				Type:     "schema",
				Path:     path,
				Expected: "array",
				Actual:   fmt.Sprintf("%T", data),
				Message:  "Expected array type",
			})
			return violations
		}

		// Validate items
		if schema.Items != nil {
			for i, item := range arr {
				itemPath := fmt.Sprintf("%s[%d]", path, i)
				itemViolations := cv.validateSchema(itemPath, schema.Items, item)
				violations = append(violations, itemViolations...)
			}
		}

	case "string":
		if _, ok := data.(string); !ok {
			violations = append(violations, ContractViolation{
				Type:     "schema",
				Path:     path,
				Expected: "string",
				Actual:   fmt.Sprintf("%T", data),
				Message:  "Expected string type",
			})
		}

	case "integer", "number":
		if !isNumeric(data) {
			violations = append(violations, ContractViolation{
				Type:     "schema",
				Path:     path,
				Expected: schema.Type,
				Actual:   fmt.Sprintf("%T", data),
				Message:  fmt.Sprintf("Expected %s type", schema.Type),
			})
		}

	case "boolean":
		if _, ok := data.(bool); !ok {
			violations = append(violations, ContractViolation{
				Type:     "schema",
				Path:     path,
				Expected: "boolean",
				Actual:   fmt.Sprintf("%T", data),
				Message:  "Expected boolean type",
			})
		}
	}

	// Validate enum
	if len(schema.Enum) > 0 {
		found := false
		for _, enumVal := range schema.Enum {
			if reflect.DeepEqual(data, enumVal) {
				found = true
				break
			}
		}
		if !found {
			violations = append(violations, ContractViolation{
				Type:     "schema",
				Path:     path,
				Expected: fmt.Sprintf("one of %v", schema.Enum),
				Actual:   fmt.Sprintf("%v", data),
				Message:  "Value not in enum",
			})
		}
	}

	return violations
}

func isNumeric(data interface{}) bool {
	switch data.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32, float64:
		return true
	}
	return false
}
