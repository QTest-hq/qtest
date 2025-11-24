package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// OpenAPIParser parses OpenAPI/Swagger specifications.
type OpenAPIParser struct {
	config *InferenceConfig
}

// NewOpenAPIParser creates a new OpenAPI parser.
func NewOpenAPIParser(config *InferenceConfig) *OpenAPIParser {
	if config == nil {
		config = DefaultInferenceConfig()
	}
	return &OpenAPIParser{config: config}
}

// ParseFile parses an OpenAPI spec from a file.
func (p *OpenAPIParser) ParseFile(path string) (*InferenceResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return p.Parse(data)
}

// Parse parses an OpenAPI spec from bytes.
func (p *OpenAPIParser) Parse(data []byte) (*InferenceResult, error) {
	// Try to parse as JSON first, then YAML
	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		if err := yaml.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("failed to parse as JSON or YAML: %w", err)
		}
	}

	// Detect version
	version := p.detectVersion(spec)

	var result *InferenceResult
	var err error

	switch version {
	case "3.0", "3.1":
		result, err = p.parseOpenAPI3(spec)
	case "2.0":
		result, err = p.parseSwagger2(spec)
	default:
		return nil, fmt.Errorf("unsupported OpenAPI version: %s", version)
	}

	if err != nil {
		return nil, err
	}

	result.Source = "openapi"
	result.Confidence = 0.95 // High confidence for explicit specs

	return result, nil
}

func (p *OpenAPIParser) detectVersion(spec map[string]interface{}) string {
	if v, ok := spec["openapi"].(string); ok {
		if strings.HasPrefix(v, "3.1") {
			return "3.1"
		}
		if strings.HasPrefix(v, "3.") {
			return "3.0"
		}
	}
	if v, ok := spec["swagger"].(string); ok {
		if strings.HasPrefix(v, "2.") {
			return "2.0"
		}
	}
	return ""
}

func (p *OpenAPIParser) parseOpenAPI3(spec map[string]interface{}) (*InferenceResult, error) {
	result := &InferenceResult{
		Spec: &APISpec{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Schemas:   make(map[string]Schema),
		},
	}

	// Parse info
	if info, ok := spec["info"].(map[string]interface{}); ok {
		result.Spec.Name = getString(info, "title")
		result.Spec.Description = getString(info, "description")
		result.Spec.Version = getString(info, "version")
	}

	// Parse servers
	if servers, ok := spec["servers"].([]interface{}); ok && len(servers) > 0 {
		if server, ok := servers[0].(map[string]interface{}); ok {
			result.Spec.BaseURL = getString(server, "url")
		}
	}

	// Parse security schemes
	if components, ok := spec["components"].(map[string]interface{}); ok {
		if securitySchemes, ok := components["securitySchemes"].(map[string]interface{}); ok {
			result.Spec.Auth = p.parseSecuritySchemes(securitySchemes)
		}
		// Parse schemas
		if schemas, ok := components["schemas"].(map[string]interface{}); ok {
			result.Spec.Schemas = p.parseSchemas(schemas)
		}
	}

	// Parse paths
	if paths, ok := spec["paths"].(map[string]interface{}); ok {
		endpoints, warnings := p.parsePaths(paths, result.Spec.Schemas)
		result.Spec.Endpoints = endpoints
		result.Warnings = warnings
	}

	return result, nil
}

func (p *OpenAPIParser) parseSwagger2(spec map[string]interface{}) (*InferenceResult, error) {
	result := &InferenceResult{
		Spec: &APISpec{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Schemas:   make(map[string]Schema),
		},
	}

	// Parse info
	if info, ok := spec["info"].(map[string]interface{}); ok {
		result.Spec.Name = getString(info, "title")
		result.Spec.Description = getString(info, "description")
		result.Spec.Version = getString(info, "version")
	}

	// Build base URL
	host := getString(spec, "host")
	basePath := getString(spec, "basePath")
	schemes := []string{"https"}
	if s, ok := spec["schemes"].([]interface{}); ok && len(s) > 0 {
		if scheme, ok := s[0].(string); ok {
			schemes = []string{scheme}
		}
	}
	if host != "" {
		result.Spec.BaseURL = fmt.Sprintf("%s://%s%s", schemes[0], host, basePath)
	}

	// Parse definitions as schemas
	if definitions, ok := spec["definitions"].(map[string]interface{}); ok {
		result.Spec.Schemas = p.parseSchemas(definitions)
	}

	// Parse security definitions
	if securityDefs, ok := spec["securityDefinitions"].(map[string]interface{}); ok {
		result.Spec.Auth = p.parseSecuritySchemes(securityDefs)
	}

	// Parse paths
	if paths, ok := spec["paths"].(map[string]interface{}); ok {
		endpoints, warnings := p.parsePaths(paths, result.Spec.Schemas)
		result.Spec.Endpoints = endpoints
		result.Warnings = warnings
	}

	return result, nil
}

func (p *OpenAPIParser) parsePaths(paths map[string]interface{}, schemas map[string]Schema) ([]Endpoint, []string) {
	var endpoints []Endpoint
	var warnings []string

	methods := []string{"get", "post", "put", "patch", "delete", "options", "head"}

	for path, pathItem := range paths {
		pathObj, ok := pathItem.(map[string]interface{})
		if !ok {
			continue
		}

		// Get path-level parameters
		pathParams := p.parseParameters(pathObj["parameters"])

		for _, method := range methods {
			operation, ok := pathObj[method].(map[string]interface{})
			if !ok {
				continue
			}

			endpoint := Endpoint{
				ID:         uuid.New().String(),
				Path:       path,
				Method:     HTTPMethod(strings.ToUpper(method)),
				Summary:    getString(operation, "summary"),
				Description: getString(operation, "description"),
				Confidence: 0.95,
				Source:     "openapi",
				CreatedAt:  time.Now(),
			}

			// Tags
			if tags, ok := operation["tags"].([]interface{}); ok {
				for _, t := range tags {
					if tag, ok := t.(string); ok {
						endpoint.Tags = append(endpoint.Tags, tag)
					}
				}
			}

			// Merge path and operation parameters
			opParams := p.parseParameters(operation["parameters"])
			endpoint.Parameters = append(pathParams, opParams...)

			// Request body (OpenAPI 3)
			if reqBody, ok := operation["requestBody"].(map[string]interface{}); ok {
				endpoint.RequestBody = p.parseRequestBody(reqBody)
			}

			// Legacy body parameter (Swagger 2)
			for _, param := range endpoint.Parameters {
				if param.Location == ParamLocationBody {
					endpoint.RequestBody = &RequestBody{
						ContentType: ContentTypeJSON,
						Required:    param.Required,
					}
				}
			}

			// Responses
			if responses, ok := operation["responses"].(map[string]interface{}); ok {
				endpoint.Responses = p.parseResponses(responses)
			}

			// Security
			if security, ok := operation["security"].([]interface{}); ok && len(security) > 0 {
				endpoint.Auth = p.parseOperationSecurity(security)
			}

			endpoints = append(endpoints, endpoint)
		}
	}

	return endpoints, warnings
}

func (p *OpenAPIParser) parseParameters(params interface{}) []Parameter {
	var result []Parameter

	paramsSlice, ok := params.([]interface{})
	if !ok {
		return result
	}

	for _, param := range paramsSlice {
		paramObj, ok := param.(map[string]interface{})
		if !ok {
			continue
		}

		p := Parameter{
			Name:        getString(paramObj, "name"),
			Description: getString(paramObj, "description"),
			Required:    getBool(paramObj, "required"),
		}

		// Location
		switch getString(paramObj, "in") {
		case "path":
			p.Location = ParamLocationPath
			p.Required = true // Path params are always required
		case "query":
			p.Location = ParamLocationQuery
		case "header":
			p.Location = ParamLocationHeader
		case "cookie":
			p.Location = ParamLocationCookie
		case "body":
			p.Location = ParamLocationBody
		default:
			p.Location = ParamLocationQuery
		}

		// Type
		if schema, ok := paramObj["schema"].(map[string]interface{}); ok {
			p.Type = DataType(getString(schema, "type"))
			p.Format = getString(schema, "format")
			p.Example = schema["example"]
			if enum, ok := schema["enum"].([]interface{}); ok {
				p.Enum = enum
			}
		} else {
			p.Type = DataType(getString(paramObj, "type"))
			p.Format = getString(paramObj, "format")
			p.Example = paramObj["example"]
		}

		if p.Type == "" {
			p.Type = DataTypeString
		}

		result = append(result, p)
	}

	return result
}

func (p *OpenAPIParser) parseRequestBody(reqBody map[string]interface{}) *RequestBody {
	rb := &RequestBody{
		Required: getBool(reqBody, "required"),
	}

	if content, ok := reqBody["content"].(map[string]interface{}); ok {
		// Prefer JSON
		for contentType, mediaType := range content {
			mt, ok := mediaType.(map[string]interface{})
			if !ok {
				continue
			}

			rb.ContentType = ContentType(contentType)
			if schema, ok := mt["schema"].(map[string]interface{}); ok {
				rb.Schema = p.parseSchema(schema)
			}
			rb.Example = mt["example"]

			// Prefer JSON if available
			if strings.Contains(contentType, "json") {
				break
			}
		}
	}

	return rb
}

func (p *OpenAPIParser) parseResponses(responses map[string]interface{}) []Response {
	var result []Response

	for code, resp := range responses {
		respObj, ok := resp.(map[string]interface{})
		if !ok {
			continue
		}

		statusCode := 0
		fmt.Sscanf(code, "%d", &statusCode)
		if statusCode == 0 {
			// Handle "default"
			statusCode = 200
		}

		r := Response{
			StatusCode:  statusCode,
			Description: getString(respObj, "description"),
		}

		// OpenAPI 3 content
		if content, ok := respObj["content"].(map[string]interface{}); ok {
			for contentType, mediaType := range content {
				mt, ok := mediaType.(map[string]interface{})
				if !ok {
					continue
				}

				r.ContentType = ContentType(contentType)
				if schema, ok := mt["schema"].(map[string]interface{}); ok {
					r.Schema = p.parseSchema(schema)
				}
				r.Example = mt["example"]
				break
			}
		}

		// Swagger 2 schema
		if schema, ok := respObj["schema"].(map[string]interface{}); ok {
			r.Schema = p.parseSchema(schema)
			r.ContentType = ContentTypeJSON
		}

		result = append(result, r)
	}

	return result
}

func (p *OpenAPIParser) parseSchema(schema map[string]interface{}) *Schema {
	s := &Schema{
		Type:   DataType(getString(schema, "type")),
		Format: getString(schema, "format"),
		Ref:    getString(schema, "$ref"),
	}

	if s.Type == "" && s.Ref == "" {
		s.Type = DataTypeObject
	}

	// Parse properties
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		s.Properties = make(map[string]*Schema)
		for name, prop := range props {
			if propSchema, ok := prop.(map[string]interface{}); ok {
				s.Properties[name] = p.parseSchema(propSchema)
			}
		}
	}

	// Parse required
	if required, ok := schema["required"].([]interface{}); ok {
		for _, r := range required {
			if name, ok := r.(string); ok {
				s.Required = append(s.Required, name)
			}
		}
	}

	// Parse items (for arrays)
	if items, ok := schema["items"].(map[string]interface{}); ok {
		s.Items = p.parseSchema(items)
	}

	// Parse enum
	if enum, ok := schema["enum"].([]interface{}); ok {
		s.Enum = enum
	}

	s.Example = schema["example"]

	return s
}

func (p *OpenAPIParser) parseSchemas(schemas map[string]interface{}) map[string]Schema {
	result := make(map[string]Schema)

	for name, schema := range schemas {
		if schemaObj, ok := schema.(map[string]interface{}); ok {
			s := p.parseSchema(schemaObj)
			if s != nil {
				result[name] = *s
			}
		}
	}

	return result
}

func (p *OpenAPIParser) parseSecuritySchemes(schemes map[string]interface{}) *AuthRequirement {
	for name, scheme := range schemes {
		schemeObj, ok := scheme.(map[string]interface{})
		if !ok {
			continue
		}

		auth := &AuthRequirement{
			Name: name,
		}

		schemeType := getString(schemeObj, "type")
		switch schemeType {
		case "http":
			httpScheme := getString(schemeObj, "scheme")
			if httpScheme == "bearer" {
				auth.Type = AuthTypeBearer
			} else if httpScheme == "basic" {
				auth.Type = AuthTypeBasic
			}
		case "apiKey":
			auth.Type = AuthTypeAPIKey
			auth.In = getString(schemeObj, "in")
			auth.Name = getString(schemeObj, "name")
		case "oauth2":
			auth.Type = AuthTypeOAuth2
		case "basic":
			auth.Type = AuthTypeBasic
		}

		return auth
	}

	return nil
}

func (p *OpenAPIParser) parseOperationSecurity(security []interface{}) *AuthRequirement {
	if len(security) == 0 {
		return nil
	}

	secItem, ok := security[0].(map[string]interface{})
	if !ok {
		return nil
	}

	for name, scopes := range secItem {
		auth := &AuthRequirement{
			Name: name,
			Type: AuthTypeBearer, // Default assumption
		}

		if scopeList, ok := scopes.([]interface{}); ok {
			for _, scope := range scopeList {
				if s, ok := scope.(string); ok {
					auth.Scopes = append(auth.Scopes, s)
				}
			}
		}

		return auth
	}

	return nil
}

// ParseFromURL fetches and parses an OpenAPI spec from a URL.
func (p *OpenAPIParser) ParseFromURL(specURL string) (*InferenceResult, error) {
	_, err := url.Parse(specURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// TODO: Implement HTTP fetching
	return nil, fmt.Errorf("URL fetching not implemented yet")
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
