package api

import (
	"time"
)

// HTTPMethod represents an HTTP method.
type HTTPMethod string

const (
	MethodGET     HTTPMethod = "GET"
	MethodPOST    HTTPMethod = "POST"
	MethodPUT     HTTPMethod = "PUT"
	MethodPATCH   HTTPMethod = "PATCH"
	MethodDELETE  HTTPMethod = "DELETE"
	MethodOPTIONS HTTPMethod = "OPTIONS"
	MethodHEAD    HTTPMethod = "HEAD"
)

// ContentType represents common content types.
type ContentType string

const (
	ContentTypeJSON      ContentType = "application/json"
	ContentTypeForm      ContentType = "application/x-www-form-urlencoded"
	ContentTypeMultipart ContentType = "multipart/form-data"
	ContentTypeXML       ContentType = "application/xml"
	ContentTypeText      ContentType = "text/plain"
	ContentTypeHTML      ContentType = "text/html"
)

// ParameterLocation represents where a parameter is located.
type ParameterLocation string

const (
	ParamLocationPath   ParameterLocation = "path"
	ParamLocationQuery  ParameterLocation = "query"
	ParamLocationHeader ParameterLocation = "header"
	ParamLocationCookie ParameterLocation = "cookie"
	ParamLocationBody   ParameterLocation = "body"
)

// DataType represents parameter/field data types.
type DataType string

const (
	DataTypeString  DataType = "string"
	DataTypeInteger DataType = "integer"
	DataTypeNumber  DataType = "number"
	DataTypeBoolean DataType = "boolean"
	DataTypeArray   DataType = "array"
	DataTypeObject  DataType = "object"
)

// AuthType represents authentication types.
type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeBasic  AuthType = "basic"
	AuthTypeBearer AuthType = "bearer"
	AuthTypeAPIKey AuthType = "apikey"
	AuthTypeOAuth2 AuthType = "oauth2"
	AuthTypeCookie AuthType = "cookie"
)

// Endpoint represents an API endpoint.
type Endpoint struct {
	ID          string            `json:"id" yaml:"id"`
	Path        string            `json:"path" yaml:"path"`
	Method      HTTPMethod        `json:"method" yaml:"method"`
	Summary     string            `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Parameters  []Parameter       `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *RequestBody      `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   []Response        `json:"responses,omitempty" yaml:"responses,omitempty"`
	Auth        *AuthRequirement  `json:"auth,omitempty" yaml:"auth,omitempty"`
	Examples    []RequestExample  `json:"examples,omitempty" yaml:"examples,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Source      string            `json:"source,omitempty" yaml:"source,omitempty"` // openapi, traffic, llm
	Confidence  float64           `json:"confidence" yaml:"confidence"`
	CreatedAt   time.Time         `json:"createdAt" yaml:"createdAt"`
}

// Parameter represents an API parameter.
type Parameter struct {
	Name        string            `json:"name" yaml:"name"`
	Location    ParameterLocation `json:"in" yaml:"in"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool              `json:"required" yaml:"required"`
	Type        DataType          `json:"type" yaml:"type"`
	Format      string            `json:"format,omitempty" yaml:"format,omitempty"`
	Default     interface{}       `json:"default,omitempty" yaml:"default,omitempty"`
	Enum        []interface{}     `json:"enum,omitempty" yaml:"enum,omitempty"`
	Pattern     string            `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Example     interface{}       `json:"example,omitempty" yaml:"example,omitempty"`
}

// RequestBody represents the request body schema.
type RequestBody struct {
	ContentType ContentType `json:"contentType" yaml:"contentType"`
	Required    bool        `json:"required" yaml:"required"`
	Schema      *Schema     `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example     interface{} `json:"example,omitempty" yaml:"example,omitempty"`
}

// Schema represents a data schema.
type Schema struct {
	Type       DataType           `json:"type" yaml:"type"`
	Format     string             `json:"format,omitempty" yaml:"format,omitempty"`
	Properties map[string]*Schema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Items      *Schema            `json:"items,omitempty" yaml:"items,omitempty"`
	Required   []string           `json:"required,omitempty" yaml:"required,omitempty"`
	Enum       []interface{}      `json:"enum,omitempty" yaml:"enum,omitempty"`
	Example    interface{}        `json:"example,omitempty" yaml:"example,omitempty"`
	Ref        string             `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// Response represents an API response.
type Response struct {
	StatusCode  int         `json:"statusCode" yaml:"statusCode"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	ContentType ContentType `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Schema      *Schema     `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example     interface{} `json:"example,omitempty" yaml:"example,omitempty"`
}

// AuthRequirement represents authentication requirements.
type AuthRequirement struct {
	Type    AuthType `json:"type" yaml:"type"`
	Name    string   `json:"name,omitempty" yaml:"name,omitempty"`
	In      string   `json:"in,omitempty" yaml:"in,omitempty"` // header, query, cookie
	Scopes  []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	Example string   `json:"example,omitempty" yaml:"example,omitempty"`
}

// RequestExample represents a request example.
type RequestExample struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Headers     map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	PathParams  map[string]string `json:"pathParams,omitempty" yaml:"pathParams,omitempty"`
	QueryParams map[string]string `json:"queryParams,omitempty" yaml:"queryParams,omitempty"`
	Body        interface{}       `json:"body,omitempty" yaml:"body,omitempty"`
}

// APISpec represents a complete API specification.
type APISpec struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string            `json:"version,omitempty" yaml:"version,omitempty"`
	BaseURL     string            `json:"baseUrl" yaml:"baseUrl"`
	Endpoints   []Endpoint        `json:"endpoints" yaml:"endpoints"`
	Auth        *AuthRequirement  `json:"auth,omitempty" yaml:"auth,omitempty"`
	Schemas     map[string]Schema `json:"schemas,omitempty" yaml:"schemas,omitempty"`
	CreatedAt   time.Time         `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt" yaml:"updatedAt"`
}

// NetworkRequest represents a captured network request.
type NetworkRequest struct {
	ID           string            `json:"id"`
	URL          string            `json:"url"`
	Method       HTTPMethod        `json:"method"`
	Headers      map[string]string `json:"headers,omitempty"`
	QueryParams  map[string]string `json:"queryParams,omitempty"`
	Body         string            `json:"body,omitempty"`
	ContentType  ContentType       `json:"contentType,omitempty"`
	StatusCode   int               `json:"statusCode,omitempty"`
	ResponseBody string            `json:"responseBody,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}

// InferenceResult represents the result of API inference.
type InferenceResult struct {
	Spec       *APISpec `json:"spec"`
	Source     string   `json:"source"` // openapi, traffic, llm, combined
	Confidence float64  `json:"confidence"`
	Warnings   []string `json:"warnings,omitempty"`
	Errors     []string `json:"errors,omitempty"`
}

// InferenceConfig configures API inference behavior.
type InferenceConfig struct {
	MinConfidence       float64  `json:"minConfidence" yaml:"minConfidence"`
	IgnorePatterns      []string `json:"ignorePatterns,omitempty" yaml:"ignorePatterns,omitempty"`
	IncludePatterns     []string `json:"includePatterns,omitempty" yaml:"includePatterns,omitempty"`
	InferAuth           bool     `json:"inferAuth" yaml:"inferAuth"`
	InferSchemas        bool     `json:"inferSchemas" yaml:"inferSchemas"`
	GroupByResource     bool     `json:"groupByResource" yaml:"groupByResource"`
	MaxRequestsToInfer  int      `json:"maxRequestsToInfer" yaml:"maxRequestsToInfer"`
}

// DefaultInferenceConfig returns default inference configuration.
func DefaultInferenceConfig() *InferenceConfig {
	return &InferenceConfig{
		MinConfidence:      0.5,
		InferAuth:          true,
		InferSchemas:       true,
		GroupByResource:    true,
		MaxRequestsToInfer: 1000,
		IgnorePatterns: []string{
			".*\\.(js|css|png|jpg|jpeg|gif|svg|ico|woff|woff2|ttf|eot)$",
			".*/static/.*",
			".*/assets/.*",
			".*/_next/.*",
			".*/webpack/.*",
		},
	}
}
