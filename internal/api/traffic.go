package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TrafficAnalyzer analyzes network traffic to infer API endpoints.
type TrafficAnalyzer struct {
	config *InferenceConfig
}

// NewTrafficAnalyzer creates a new traffic analyzer.
func NewTrafficAnalyzer(config *InferenceConfig) *TrafficAnalyzer {
	if config == nil {
		config = DefaultInferenceConfig()
	}
	return &TrafficAnalyzer{config: config}
}

// Analyze analyzes network requests and infers API endpoints.
func (t *TrafficAnalyzer) Analyze(requests []NetworkRequest) (*InferenceResult, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("no requests to analyze")
	}

	// Filter and limit requests
	filtered := t.filterRequests(requests)
	if len(filtered) > t.config.MaxRequestsToInfer {
		filtered = filtered[:t.config.MaxRequestsToInfer]
	}

	// Group requests by endpoint pattern
	groups := t.groupByEndpoint(filtered)

	// Build endpoints from groups
	endpoints := t.buildEndpoints(groups)

	// Infer base URL
	baseURL := t.inferBaseURL(filtered)

	result := &InferenceResult{
		Spec: &APISpec{
			ID:        uuid.New().String(),
			Name:      "Inferred API",
			BaseURL:   baseURL,
			Endpoints: endpoints,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Source:     "traffic",
		Confidence: t.calculateConfidence(endpoints),
	}

	return result, nil
}

func (t *TrafficAnalyzer) filterRequests(requests []NetworkRequest) []NetworkRequest {
	var filtered []NetworkRequest

	for _, req := range requests {
		// Parse URL
		parsed, err := url.Parse(req.URL)
		if err != nil {
			continue
		}

		// Check ignore patterns
		if t.shouldIgnore(parsed.Path) {
			continue
		}

		// Check include patterns
		if len(t.config.IncludePatterns) > 0 && !t.shouldInclude(parsed.Path) {
			continue
		}

		// Only include API-like requests
		if !t.isAPIRequest(req) {
			continue
		}

		filtered = append(filtered, req)
	}

	return filtered
}

func (t *TrafficAnalyzer) shouldIgnore(path string) bool {
	for _, pattern := range t.config.IgnorePatterns {
		if matched, _ := regexp.MatchString(pattern, path); matched {
			return true
		}
	}
	return false
}

func (t *TrafficAnalyzer) shouldInclude(path string) bool {
	for _, pattern := range t.config.IncludePatterns {
		if matched, _ := regexp.MatchString(pattern, path); matched {
			return true
		}
	}
	return false
}

func (t *TrafficAnalyzer) isAPIRequest(req NetworkRequest) bool {
	// Check if it looks like an API request

	// Check content type
	if req.ContentType != "" {
		ct := string(req.ContentType)
		if strings.Contains(ct, "json") || strings.Contains(ct, "xml") {
			return true
		}
	}

	// Check URL patterns
	urlLower := strings.ToLower(req.URL)
	apiPatterns := []string{"/api/", "/v1/", "/v2/", "/v3/", "/rest/", "/graphql"}
	for _, pattern := range apiPatterns {
		if strings.Contains(urlLower, pattern) {
			return true
		}
	}

	// Check response
	if req.StatusCode >= 200 && req.StatusCode < 300 {
		if strings.HasPrefix(strings.TrimSpace(req.ResponseBody), "{") ||
			strings.HasPrefix(strings.TrimSpace(req.ResponseBody), "[") {
			return true
		}
	}

	// Non-GET requests with body are likely API calls
	if req.Method != MethodGET && req.Body != "" {
		return true
	}

	return false
}

// endpointGroup represents a group of similar requests.
type endpointGroup struct {
	pattern  string
	method   HTTPMethod
	requests []NetworkRequest
}

func (t *TrafficAnalyzer) groupByEndpoint(requests []NetworkRequest) []endpointGroup {
	groups := make(map[string]*endpointGroup)

	for _, req := range requests {
		parsed, err := url.Parse(req.URL)
		if err != nil {
			continue
		}

		// Normalize path (replace IDs with placeholders)
		pattern := t.normalizePath(parsed.Path)
		key := fmt.Sprintf("%s %s", req.Method, pattern)

		if group, exists := groups[key]; exists {
			group.requests = append(group.requests, req)
		} else {
			groups[key] = &endpointGroup{
				pattern:  pattern,
				method:   req.Method,
				requests: []NetworkRequest{req},
			}
		}
	}

	// Convert to slice
	var result []endpointGroup
	for _, group := range groups {
		result = append(result, *group)
	}

	// Sort by frequency
	sort.Slice(result, func(i, j int) bool {
		return len(result[i].requests) > len(result[j].requests)
	})

	return result
}

func (t *TrafficAnalyzer) normalizePath(path string) string {
	segments := strings.Split(path, "/")
	var normalized []string

	for _, seg := range segments {
		if seg == "" {
			continue
		}

		// Replace UUIDs
		if isUUID(seg) {
			normalized = append(normalized, "{id}")
			continue
		}

		// Replace numeric IDs
		if isNumericID(seg) {
			normalized = append(normalized, "{id}")
			continue
		}

		// Replace MongoDB ObjectIds
		if isObjectID(seg) {
			normalized = append(normalized, "{id}")
			continue
		}

		normalized = append(normalized, seg)
	}

	return "/" + strings.Join(normalized, "/")
}

func (t *TrafficAnalyzer) buildEndpoints(groups []endpointGroup) []Endpoint {
	var endpoints []Endpoint

	for _, group := range groups {
		endpoint := Endpoint{
			ID:         uuid.New().String(),
			Path:       group.pattern,
			Method:     group.method,
			Source:     "traffic",
			Confidence: t.calculateEndpointConfidence(group),
			CreatedAt:  time.Now(),
		}

		// Infer parameters
		endpoint.Parameters = t.inferParameters(group)

		// Infer request body
		endpoint.RequestBody = t.inferRequestBody(group)

		// Infer responses
		endpoint.Responses = t.inferResponses(group)

		// Infer auth
		if t.config.InferAuth {
			endpoint.Auth = t.inferAuth(group)
		}

		// Generate examples
		endpoint.Examples = t.generateExamples(group)

		// Generate summary
		endpoint.Summary = t.generateSummary(group)

		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

func (t *TrafficAnalyzer) inferParameters(group endpointGroup) []Parameter {
	var params []Parameter

	// Collect all query parameters
	queryParams := make(map[string][]string)

	for _, req := range group.requests {
		parsed, err := url.Parse(req.URL)
		if err != nil {
			continue
		}

		for key, values := range parsed.Query() {
			queryParams[key] = append(queryParams[key], values...)
		}
	}

	// Build parameters
	for name, values := range queryParams {
		param := Parameter{
			Name:     name,
			Location: ParamLocationQuery,
			Type:     t.inferDataType(values),
			Required: len(values) == len(group.requests), // Required if present in all requests
		}

		if len(values) > 0 {
			param.Example = values[0]
		}

		params = append(params, param)
	}

	// Infer path parameters
	pathParams := t.extractPathParams(group.pattern)
	for _, name := range pathParams {
		params = append(params, Parameter{
			Name:     name,
			Location: ParamLocationPath,
			Type:     DataTypeString,
			Required: true,
		})
	}

	return params
}

func (t *TrafficAnalyzer) extractPathParams(pattern string) []string {
	var params []string
	re := regexp.MustCompile(`\{([^}]+)\}`)
	matches := re.FindAllStringSubmatch(pattern, -1)
	for _, match := range matches {
		if len(match) > 1 {
			params = append(params, match[1])
		}
	}
	return params
}

func (t *TrafficAnalyzer) inferDataType(values []string) DataType {
	if len(values) == 0 {
		return DataTypeString
	}

	allInts := true
	allFloats := true
	allBools := true

	for _, v := range values {
		// Check integer
		if !regexp.MustCompile(`^-?\d+$`).MatchString(v) {
			allInts = false
		}

		// Check float
		if !regexp.MustCompile(`^-?\d+\.?\d*$`).MatchString(v) {
			allFloats = false
		}

		// Check bool
		vLower := strings.ToLower(v)
		if vLower != "true" && vLower != "false" && v != "0" && v != "1" {
			allBools = false
		}
	}

	if allBools {
		return DataTypeBoolean
	}
	if allInts {
		return DataTypeInteger
	}
	if allFloats {
		return DataTypeNumber
	}
	return DataTypeString
}

func (t *TrafficAnalyzer) inferRequestBody(group endpointGroup) *RequestBody {
	if group.method == MethodGET || group.method == MethodDELETE {
		return nil
	}

	// Find a request with body
	for _, req := range group.requests {
		if req.Body == "" {
			continue
		}

		rb := &RequestBody{
			ContentType: req.ContentType,
			Required:    true,
		}

		if rb.ContentType == "" {
			rb.ContentType = ContentTypeJSON
		}

		// Try to infer schema from body
		if t.config.InferSchemas {
			rb.Schema = t.inferSchemaFromJSON(req.Body)
		}

		rb.Example = req.Body

		return rb
	}

	return nil
}

func (t *TrafficAnalyzer) inferSchemaFromJSON(body string) *Schema {
	var data interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil
	}

	return t.buildSchemaFromValue(data)
}

func (t *TrafficAnalyzer) buildSchemaFromValue(v interface{}) *Schema {
	switch val := v.(type) {
	case map[string]interface{}:
		schema := &Schema{
			Type:       DataTypeObject,
			Properties: make(map[string]*Schema),
		}
		for key, value := range val {
			schema.Properties[key] = t.buildSchemaFromValue(value)
		}
		return schema

	case []interface{}:
		schema := &Schema{Type: DataTypeArray}
		if len(val) > 0 {
			schema.Items = t.buildSchemaFromValue(val[0])
		}
		return schema

	case string:
		schema := &Schema{Type: DataTypeString}
		// Detect format
		if isUUID(val) {
			schema.Format = "uuid"
		} else if isEmail(val) {
			schema.Format = "email"
		} else if isDateTime(val) {
			schema.Format = "date-time"
		}
		return schema

	case float64:
		if val == float64(int64(val)) {
			return &Schema{Type: DataTypeInteger}
		}
		return &Schema{Type: DataTypeNumber}

	case bool:
		return &Schema{Type: DataTypeBoolean}

	case nil:
		return &Schema{Type: DataTypeString}

	default:
		return &Schema{Type: DataTypeString}
	}
}

func (t *TrafficAnalyzer) inferResponses(group endpointGroup) []Response {
	responsesByCode := make(map[int][]NetworkRequest)

	for _, req := range group.requests {
		if req.StatusCode > 0 {
			responsesByCode[req.StatusCode] = append(responsesByCode[req.StatusCode], req)
		}
	}

	var responses []Response

	for code, reqs := range responsesByCode {
		resp := Response{
			StatusCode:  code,
			Description: httpStatusDescription(code),
		}

		// Find example response
		for _, req := range reqs {
			if req.ResponseBody != "" {
				resp.ContentType = ContentTypeJSON
				resp.Example = req.ResponseBody

				if t.config.InferSchemas {
					resp.Schema = t.inferSchemaFromJSON(req.ResponseBody)
				}
				break
			}
		}

		responses = append(responses, resp)
	}

	// Sort by status code
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].StatusCode < responses[j].StatusCode
	})

	return responses
}

func (t *TrafficAnalyzer) inferAuth(group endpointGroup) *AuthRequirement {
	for _, req := range group.requests {
		// Check Authorization header
		if auth, ok := req.Headers["Authorization"]; ok {
			if strings.HasPrefix(auth, "Bearer ") {
				return &AuthRequirement{
					Type: AuthTypeBearer,
					In:   "header",
					Name: "Authorization",
				}
			}
			if strings.HasPrefix(auth, "Basic ") {
				return &AuthRequirement{
					Type: AuthTypeBasic,
					In:   "header",
					Name: "Authorization",
				}
			}
		}

		// Check API key headers
		for header := range req.Headers {
			headerLower := strings.ToLower(header)
			if strings.Contains(headerLower, "api-key") ||
				strings.Contains(headerLower, "apikey") ||
				strings.Contains(headerLower, "x-api-key") {
				return &AuthRequirement{
					Type: AuthTypeAPIKey,
					In:   "header",
					Name: header,
				}
			}
		}

		// Check cookies
		if _, ok := req.Headers["Cookie"]; ok {
			return &AuthRequirement{
				Type: AuthTypeCookie,
				In:   "cookie",
			}
		}
	}

	return nil
}

func (t *TrafficAnalyzer) generateExamples(group endpointGroup) []RequestExample {
	if len(group.requests) == 0 {
		return nil
	}

	// Use first request as example
	req := group.requests[0]
	parsed, _ := url.Parse(req.URL)

	example := RequestExample{
		Name:        fmt.Sprintf("Example %s", req.Method),
		Headers:     req.Headers,
		QueryParams: make(map[string]string),
	}

	if parsed != nil {
		for key, values := range parsed.Query() {
			if len(values) > 0 {
				example.QueryParams[key] = values[0]
			}
		}
	}

	if req.Body != "" {
		var body interface{}
		if json.Unmarshal([]byte(req.Body), &body) == nil {
			example.Body = body
		} else {
			example.Body = req.Body
		}
	}

	return []RequestExample{example}
}

func (t *TrafficAnalyzer) generateSummary(group endpointGroup) string {
	method := strings.ToUpper(string(group.method))
	path := group.pattern

	// Extract resource name
	segments := strings.Split(strings.Trim(path, "/"), "/")
	resource := "resource"
	for i := len(segments) - 1; i >= 0; i-- {
		if !strings.HasPrefix(segments[i], "{") {
			resource = segments[i]
			break
		}
	}

	switch group.method {
	case MethodGET:
		if strings.HasSuffix(path, "/{id}") {
			return fmt.Sprintf("Get %s by ID", resource)
		}
		return fmt.Sprintf("List %s", resource)
	case MethodPOST:
		return fmt.Sprintf("Create %s", resource)
	case MethodPUT:
		return fmt.Sprintf("Update %s", resource)
	case MethodPATCH:
		return fmt.Sprintf("Partially update %s", resource)
	case MethodDELETE:
		return fmt.Sprintf("Delete %s", resource)
	default:
		return fmt.Sprintf("%s %s", method, resource)
	}
}

func (t *TrafficAnalyzer) inferBaseURL(requests []NetworkRequest) string {
	if len(requests) == 0 {
		return ""
	}

	// Count host occurrences
	hosts := make(map[string]int)
	for _, req := range requests {
		parsed, err := url.Parse(req.URL)
		if err != nil {
			continue
		}
		baseURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
		hosts[baseURL]++
	}

	// Return most common host
	var maxHost string
	maxCount := 0
	for host, count := range hosts {
		if count > maxCount {
			maxCount = count
			maxHost = host
		}
	}

	return maxHost
}

func (t *TrafficAnalyzer) calculateConfidence(endpoints []Endpoint) float64 {
	if len(endpoints) == 0 {
		return 0
	}

	total := 0.0
	for _, ep := range endpoints {
		total += ep.Confidence
	}
	return total / float64(len(endpoints))
}

func (t *TrafficAnalyzer) calculateEndpointConfidence(group endpointGroup) float64 {
	// Base confidence
	confidence := 0.5

	// More requests = higher confidence
	if len(group.requests) > 5 {
		confidence += 0.2
	} else if len(group.requests) > 2 {
		confidence += 0.1
	}

	// Consistent responses = higher confidence
	statusCodes := make(map[int]int)
	for _, req := range group.requests {
		statusCodes[req.StatusCode]++
	}
	if len(statusCodes) == 1 {
		confidence += 0.15
	}

	// Has JSON body = higher confidence
	for _, req := range group.requests {
		if req.ContentType == ContentTypeJSON {
			confidence += 0.1
			break
		}
	}

	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}

// Helper functions

func isUUID(s string) bool {
	re := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	return re.MatchString(s)
}

func isNumericID(s string) bool {
	re := regexp.MustCompile(`^\d+$`)
	return re.MatchString(s) && len(s) <= 20
}

func isObjectID(s string) bool {
	re := regexp.MustCompile(`^[0-9a-fA-F]{24}$`)
	return re.MatchString(s)
}

func isEmail(s string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(s)
}

func isDateTime(s string) bool {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		if _, err := time.Parse(format, s); err == nil {
			return true
		}
	}
	return false
}

func httpStatusDescription(code int) string {
	descriptions := map[int]string{
		200: "Successful response",
		201: "Resource created",
		204: "No content",
		400: "Bad request",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not found",
		409: "Conflict",
		422: "Unprocessable entity",
		500: "Internal server error",
	}
	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return fmt.Sprintf("HTTP %d", code)
}
