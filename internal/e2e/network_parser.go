// Package e2e provides end-to-end test generation capabilities.
package e2e

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/QTest-hq/qtest/internal/api"
	"github.com/QTest-hq/qtest/internal/sidecar/playwright"
)

// NetworkParser parses captured network traffic and infers API specifications.
type NetworkParser struct {
	config *api.InferenceConfig
}

// NewNetworkParser creates a new network parser with the given configuration.
func NewNetworkParser(config *api.InferenceConfig) *NetworkParser {
	if config == nil {
		config = api.DefaultInferenceConfig()
	}
	return &NetworkParser{config: config}
}

// ParseCapturedRequests converts Playwright captured requests to NetworkRequests.
func (p *NetworkParser) ParseCapturedRequests(captured []playwright.CapturedNetworkRequest) []api.NetworkRequest {
	var requests []api.NetworkRequest

	for _, c := range captured {
		// Skip if should be ignored
		if p.shouldIgnore(c.URL, c.ResourceType) {
			continue
		}

		req := api.NetworkRequest{
			ID:          c.RequestID,
			URL:         c.URL,
			Method:      api.HTTPMethod(strings.ToUpper(c.Method)),
			Headers:     c.Headers,
			Body:        c.Body,
			ContentType: p.detectContentType(c.Headers),
			Timestamp:   time.UnixMilli(c.Timestamp),
		}

		// Parse query params from URL
		if parsedURL, err := url.Parse(c.URL); err == nil {
			req.QueryParams = make(map[string]string)
			for k, v := range parsedURL.Query() {
				if len(v) > 0 {
					req.QueryParams[k] = v[0]
				}
			}
		}

		// Add response info if available
		if c.Response != nil {
			req.StatusCode = int(c.Response.StatusCode)
			req.ResponseBody = c.Response.Body
		}

		requests = append(requests, req)
	}

	return requests
}

// InferAPISpec infers an API specification from network requests.
func (p *NetworkParser) InferAPISpec(requests []api.NetworkRequest, baseURL string) *api.InferenceResult {
	result := &api.InferenceResult{
		Source: "traffic",
	}

	if len(requests) == 0 {
		result.Warnings = append(result.Warnings, "No requests to analyze")
		return result
	}

	// Filter to API requests only
	apiRequests := p.filterAPIRequests(requests)
	if len(apiRequests) == 0 {
		result.Warnings = append(result.Warnings, "No API requests found in traffic")
		return result
	}

	// Group requests by endpoint pattern
	endpointGroups := p.groupByEndpoint(apiRequests)

	// Infer base URL if not provided
	if baseURL == "" {
		baseURL = p.inferBaseURL(apiRequests)
	}

	// Build API spec
	spec := &api.APISpec{
		ID:        uuid.New().String(),
		Name:      "Inferred API",
		BaseURL:   baseURL,
		Endpoints: make([]api.Endpoint, 0),
		Schemas:   make(map[string]api.Schema),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Convert groups to endpoints
	for pattern, group := range endpointGroups {
		endpoint := p.inferEndpoint(pattern, group)
		spec.Endpoints = append(spec.Endpoints, endpoint)
	}

	// Sort endpoints by path
	sort.Slice(spec.Endpoints, func(i, j int) bool {
		return spec.Endpoints[i].Path < spec.Endpoints[j].Path
	})

	// Infer global auth if configured
	if p.config.InferAuth {
		spec.Auth = p.inferAuth(apiRequests)
	}

	// Calculate overall confidence
	result.Spec = spec
	result.Confidence = p.calculateConfidence(apiRequests, spec)

	return result
}

// shouldIgnore checks if a request should be ignored based on patterns.
func (p *NetworkParser) shouldIgnore(reqURL string, resourceType string) bool {
	// Ignore non-API resource types
	apiResourceTypes := map[string]bool{
		"xhr":      true,
		"fetch":    true,
		"document": true, // Sometimes APIs return document type
	}
	if resourceType != "" && !apiResourceTypes[strings.ToLower(resourceType)] {
		return true
	}

	// Check ignore patterns
	for _, pattern := range p.config.IgnorePatterns {
		if matched, _ := regexp.MatchString(pattern, reqURL); matched {
			return true
		}
	}

	// Check include patterns (if specified, only include matching)
	if len(p.config.IncludePatterns) > 0 {
		for _, pattern := range p.config.IncludePatterns {
			if matched, _ := regexp.MatchString(pattern, reqURL); matched {
				return false
			}
		}
		return true // Not matching any include pattern
	}

	return false
}

// filterAPIRequests filters requests to only API-like requests.
func (p *NetworkParser) filterAPIRequests(requests []api.NetworkRequest) []api.NetworkRequest {
	var apiRequests []api.NetworkRequest

	for _, req := range requests {
		// Check content type indicates API
		if p.isAPIContentType(req.ContentType) {
			apiRequests = append(apiRequests, req)
			continue
		}

		// Check response looks like JSON
		if req.ResponseBody != "" && p.looksLikeJSON(req.ResponseBody) {
			apiRequests = append(apiRequests, req)
			continue
		}

		// Check URL patterns that suggest API
		if p.isAPIURL(req.URL) {
			apiRequests = append(apiRequests, req)
			continue
		}
	}

	return apiRequests
}

// groupByEndpoint groups requests by their normalized endpoint pattern.
func (p *NetworkParser) groupByEndpoint(requests []api.NetworkRequest) map[string][]api.NetworkRequest {
	groups := make(map[string][]api.NetworkRequest)

	for _, req := range requests {
		pattern := p.normalizeEndpoint(req.URL, req.Method)
		groups[pattern] = append(groups[pattern], req)
	}

	return groups
}

// normalizeEndpoint extracts the endpoint pattern from a URL.
func (p *NetworkParser) normalizeEndpoint(reqURL string, method api.HTTPMethod) string {
	parsed, err := url.Parse(reqURL)
	if err != nil {
		return string(method) + " " + reqURL
	}

	// Get path and normalize IDs
	path := parsed.Path
	path = p.normalizePathParams(path)

	return string(method) + " " + path
}

// normalizePathParams replaces likely ID segments with parameter placeholders.
func (p *NetworkParser) normalizePathParams(path string) string {
	segments := strings.Split(path, "/")
	var normalized []string

	for _, seg := range segments {
		if seg == "" {
			continue
		}

		// UUID pattern
		if isUUID(seg) {
			normalized = append(normalized, "{id}")
			continue
		}

		// Numeric ID pattern
		if isNumericID(seg) {
			normalized = append(normalized, "{id}")
			continue
		}

		// MongoDB ObjectId pattern
		if isMongoID(seg) {
			normalized = append(normalized, "{id}")
			continue
		}

		normalized = append(normalized, seg)
	}

	return "/" + strings.Join(normalized, "/")
}

// inferEndpoint creates an Endpoint from a group of similar requests.
func (p *NetworkParser) inferEndpoint(pattern string, requests []api.NetworkRequest) api.Endpoint {
	parts := strings.SplitN(pattern, " ", 2)
	method := api.HTTPMethod(parts[0])
	path := parts[1]

	endpoint := api.Endpoint{
		ID:         uuid.New().String(),
		Path:       path,
		Method:     method,
		Source:     "traffic",
		CreatedAt:  time.Now(),
		Confidence: p.calculateEndpointConfidence(requests),
	}

	// Generate summary
	endpoint.Summary = p.generateSummary(method, path)

	// Infer path parameters
	endpoint.Parameters = p.inferPathParams(path)

	// Infer query parameters from all requests
	queryParams := p.inferQueryParams(requests)
	endpoint.Parameters = append(endpoint.Parameters, queryParams...)

	// Infer request body schema
	if method == api.MethodPOST || method == api.MethodPUT || method == api.MethodPATCH {
		endpoint.RequestBody = p.inferRequestBody(requests)
	}

	// Infer responses
	endpoint.Responses = p.inferResponses(requests)

	// Add example from first request
	if len(requests) > 0 {
		endpoint.Examples = []api.RequestExample{
			p.createExample(requests[0]),
		}
	}

	return endpoint
}

// inferPathParams extracts path parameters from the endpoint path.
func (p *NetworkParser) inferPathParams(path string) []api.Parameter {
	var params []api.Parameter

	// Find {param} patterns
	re := regexp.MustCompile(`\{(\w+)\}`)
	matches := re.FindAllStringSubmatch(path, -1)

	for _, match := range matches {
		params = append(params, api.Parameter{
			Name:        match[1],
			Location:    api.ParamLocationPath,
			Required:    true,
			Type:        api.DataTypeString,
			Description: "Path parameter",
		})
	}

	return params
}

// inferQueryParams infers query parameters from requests.
func (p *NetworkParser) inferQueryParams(requests []api.NetworkRequest) []api.Parameter {
	paramMap := make(map[string]*paramStats)

	for _, req := range requests {
		for name, value := range req.QueryParams {
			if _, ok := paramMap[name]; !ok {
				paramMap[name] = &paramStats{
					values: make(map[string]int),
				}
			}
			paramMap[name].count++
			paramMap[name].values[value]++
			paramMap[name].inferType(value)
		}
	}

	var params []api.Parameter
	for name, stats := range paramMap {
		param := api.Parameter{
			Name:     name,
			Location: api.ParamLocationQuery,
			Required: stats.count == len(requests),
			Type:     stats.bestType(),
		}

		// Add example from most common value
		param.Example = stats.mostCommonValue()

		params = append(params, param)
	}

	return params
}

// inferRequestBody infers the request body schema from requests.
func (p *NetworkParser) inferRequestBody(requests []api.NetworkRequest) *api.RequestBody {
	var bodies []string
	for _, req := range requests {
		if req.Body != "" {
			bodies = append(bodies, req.Body)
		}
	}

	if len(bodies) == 0 {
		return nil
	}

	// Try to parse as JSON and infer schema
	schema := p.inferJSONSchema(bodies)
	if schema == nil {
		return nil
	}

	return &api.RequestBody{
		ContentType: api.ContentTypeJSON,
		Required:    true,
		Schema:      schema,
		Example:     p.parseJSON(bodies[0]),
	}
}

// inferResponses infers response schemas from requests.
func (p *NetworkParser) inferResponses(requests []api.NetworkRequest) []api.Response {
	// Group by status code
	statusGroups := make(map[int][]string)
	for _, req := range requests {
		if req.StatusCode > 0 && req.ResponseBody != "" {
			statusGroups[req.StatusCode] = append(statusGroups[req.StatusCode], req.ResponseBody)
		}
	}

	var responses []api.Response
	for statusCode, bodies := range statusGroups {
		resp := api.Response{
			StatusCode:  statusCode,
			Description: httpStatusDescription(statusCode),
			ContentType: api.ContentTypeJSON,
		}

		if p.config.InferSchemas && len(bodies) > 0 {
			resp.Schema = p.inferJSONSchema(bodies)
			resp.Example = p.parseJSON(bodies[0])
		}

		responses = append(responses, resp)
	}

	// Sort by status code
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].StatusCode < responses[j].StatusCode
	})

	return responses
}

// inferJSONSchema infers a JSON schema from multiple JSON samples.
func (p *NetworkParser) inferJSONSchema(samples []string) *api.Schema {
	if len(samples) == 0 {
		return nil
	}

	// Parse first sample to determine structure
	var data interface{}
	if err := json.Unmarshal([]byte(samples[0]), &data); err != nil {
		return nil
	}

	return p.schemaFromValue(data)
}

// schemaFromValue creates a schema from a Go value.
func (p *NetworkParser) schemaFromValue(v interface{}) *api.Schema {
	if v == nil {
		return &api.Schema{Type: api.DataTypeString}
	}

	switch val := v.(type) {
	case bool:
		return &api.Schema{Type: api.DataTypeBoolean}

	case float64:
		// JSON numbers are float64; check if integer
		if val == float64(int64(val)) {
			return &api.Schema{Type: api.DataTypeInteger}
		}
		return &api.Schema{Type: api.DataTypeNumber}

	case string:
		schema := &api.Schema{Type: api.DataTypeString}
		schema.Format = p.inferStringFormat(val)
		return schema

	case []interface{}:
		schema := &api.Schema{Type: api.DataTypeArray}
		if len(val) > 0 {
			schema.Items = p.schemaFromValue(val[0])
		}
		return schema

	case map[string]interface{}:
		schema := &api.Schema{
			Type:       api.DataTypeObject,
			Properties: make(map[string]*api.Schema),
		}
		for k, v := range val {
			schema.Properties[k] = p.schemaFromValue(v)
		}
		return schema

	default:
		return &api.Schema{Type: api.DataTypeString}
	}
}

// inferStringFormat detects common string formats.
func (p *NetworkParser) inferStringFormat(s string) string {
	// Email
	if strings.Contains(s, "@") && strings.Contains(s, ".") {
		return "email"
	}

	// UUID
	if isUUID(s) {
		return "uuid"
	}

	// Date-time (ISO 8601)
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return "date-time"
	}

	// URL
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return "uri"
	}

	return ""
}

// inferAuth infers authentication from request headers.
func (p *NetworkParser) inferAuth(requests []api.NetworkRequest) *api.AuthRequirement {
	authCounts := make(map[string]int)

	for _, req := range requests {
		if auth := req.Headers["Authorization"]; auth != "" {
			if strings.HasPrefix(auth, "Bearer ") {
				authCounts["bearer"]++
			} else if strings.HasPrefix(auth, "Basic ") {
				authCounts["basic"]++
			}
		}
		if req.Headers["X-API-Key"] != "" {
			authCounts["apikey"]++
		}
		if req.Headers["X-Auth-Token"] != "" {
			authCounts["apikey"]++
		}
	}

	// Find most common auth type
	maxCount := 0
	authType := api.AuthTypeNone
	for t, count := range authCounts {
		if count > maxCount {
			maxCount = count
			switch t {
			case "bearer":
				authType = api.AuthTypeBearer
			case "basic":
				authType = api.AuthTypeBasic
			case "apikey":
				authType = api.AuthTypeAPIKey
			}
		}
	}

	if authType == api.AuthTypeNone {
		return nil
	}

	return &api.AuthRequirement{
		Type: authType,
		In:   "header",
	}
}

// inferBaseURL determines the base URL from requests.
func (p *NetworkParser) inferBaseURL(requests []api.NetworkRequest) string {
	urlCounts := make(map[string]int)

	for _, req := range requests {
		parsed, err := url.Parse(req.URL)
		if err != nil {
			continue
		}
		base := parsed.Scheme + "://" + parsed.Host
		urlCounts[base]++
	}

	// Return most common base URL
	maxCount := 0
	baseURL := ""
	for u, count := range urlCounts {
		if count > maxCount {
			maxCount = count
			baseURL = u
		}
	}

	return baseURL
}

// createExample creates a request example from a network request.
func (p *NetworkParser) createExample(req api.NetworkRequest) api.RequestExample {
	example := api.RequestExample{
		Name:        "Captured request",
		QueryParams: req.QueryParams,
		Headers:     make(map[string]string),
	}

	// Only include relevant headers
	relevantHeaders := []string{"Content-Type", "Accept", "Authorization"}
	for _, h := range relevantHeaders {
		if v := req.Headers[h]; v != "" {
			example.Headers[h] = v
		}
	}

	// Add body
	if req.Body != "" {
		example.Body = p.parseJSON(req.Body)
	}

	return example
}

// generateSummary generates a human-readable summary for an endpoint.
func (p *NetworkParser) generateSummary(method api.HTTPMethod, path string) string {
	// Extract resource name from path
	segments := strings.Split(strings.Trim(path, "/"), "/")
	var resource string
	for i := len(segments) - 1; i >= 0; i-- {
		if !strings.HasPrefix(segments[i], "{") {
			resource = segments[i]
			break
		}
	}

	if resource == "" {
		resource = "resource"
	}

	// Generate action-based summary
	switch method {
	case api.MethodGET:
		if strings.Contains(path, "{") {
			return "Get " + singularize(resource)
		}
		return "List " + resource
	case api.MethodPOST:
		return "Create " + singularize(resource)
	case api.MethodPUT:
		return "Update " + singularize(resource)
	case api.MethodPATCH:
		return "Partial update " + singularize(resource)
	case api.MethodDELETE:
		return "Delete " + singularize(resource)
	default:
		return string(method) + " " + resource
	}
}

// calculateConfidence calculates overall inference confidence.
func (p *NetworkParser) calculateConfidence(requests []api.NetworkRequest, spec *api.APISpec) float64 {
	if len(requests) == 0 || len(spec.Endpoints) == 0 {
		return 0.0
	}

	// Factors:
	// 1. Number of requests per endpoint (more = higher confidence)
	// 2. Response success rate
	// 3. Schema inference success

	totalConfidence := 0.0
	for _, ep := range spec.Endpoints {
		totalConfidence += ep.Confidence
	}

	return totalConfidence / float64(len(spec.Endpoints))
}

// calculateEndpointConfidence calculates confidence for a single endpoint.
func (p *NetworkParser) calculateEndpointConfidence(requests []api.NetworkRequest) float64 {
	if len(requests) == 0 {
		return 0.0
	}

	confidence := 0.5 // Base confidence

	// More requests = higher confidence (up to 0.3 boost)
	requestBoost := float64(len(requests)) / 10.0
	if requestBoost > 0.3 {
		requestBoost = 0.3
	}
	confidence += requestBoost

	// Check response success rate
	successCount := 0
	for _, req := range requests {
		if req.StatusCode >= 200 && req.StatusCode < 400 {
			successCount++
		}
	}
	successRate := float64(successCount) / float64(len(requests))
	confidence += successRate * 0.2

	return confidence
}

// Helper functions

func (p *NetworkParser) detectContentType(headers map[string]string) api.ContentType {
	ct := headers["Content-Type"]
	if ct == "" {
		ct = headers["content-type"]
	}
	ct = strings.ToLower(ct)

	if strings.Contains(ct, "json") {
		return api.ContentTypeJSON
	}
	if strings.Contains(ct, "form-urlencoded") {
		return api.ContentTypeForm
	}
	if strings.Contains(ct, "multipart") {
		return api.ContentTypeMultipart
	}
	if strings.Contains(ct, "xml") {
		return api.ContentTypeXML
	}
	if strings.Contains(ct, "text/html") {
		return api.ContentTypeHTML
	}

	return api.ContentTypeJSON // Default to JSON for API inference
}

func (p *NetworkParser) isAPIContentType(ct api.ContentType) bool {
	return ct == api.ContentTypeJSON || ct == api.ContentTypeXML
}

func (p *NetworkParser) looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

func (p *NetworkParser) isAPIURL(u string) bool {
	lower := strings.ToLower(u)
	return strings.Contains(lower, "/api/") ||
		strings.Contains(lower, "/v1/") ||
		strings.Contains(lower, "/v2/") ||
		strings.Contains(lower, "/graphql")
}

func (p *NetworkParser) parseJSON(s string) interface{} {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
}

// paramStats tracks statistics about a parameter.
type paramStats struct {
	count       int
	values      map[string]int
	seenString  bool
	seenInt     bool
	seenFloat   bool
	seenBool    bool
}

func (ps *paramStats) inferType(value string) {
	// Check boolean
	if value == "true" || value == "false" {
		ps.seenBool = true
		return
	}

	// Check integer
	if isNumericID(value) {
		ps.seenInt = true
		return
	}

	// Check float
	if _, err := json.Number(value).Float64(); err == nil && strings.Contains(value, ".") {
		ps.seenFloat = true
		return
	}

	ps.seenString = true
}

func (ps *paramStats) bestType() api.DataType {
	if ps.seenString {
		return api.DataTypeString
	}
	if ps.seenFloat {
		return api.DataTypeNumber
	}
	if ps.seenInt {
		return api.DataTypeInteger
	}
	if ps.seenBool {
		return api.DataTypeBoolean
	}
	return api.DataTypeString
}

func (ps *paramStats) mostCommonValue() interface{} {
	maxCount := 0
	var result string
	for v, c := range ps.values {
		if c > maxCount {
			maxCount = c
			result = v
		}
	}
	return result
}

// isUUID checks if a string looks like a UUID.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	match, _ := regexp.MatchString(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`, s)
	return match
}

// isNumericID checks if a string is a numeric ID.
func isNumericID(s string) bool {
	if len(s) == 0 || len(s) > 20 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isMongoID checks if a string looks like a MongoDB ObjectId.
func isMongoID(s string) bool {
	if len(s) != 24 {
		return false
	}
	match, _ := regexp.MatchString(`^[0-9a-fA-F]{24}$`, s)
	return match
}

// singularize attempts to convert a plural word to singular.
func singularize(s string) string {
	if strings.HasSuffix(s, "ies") {
		return strings.TrimSuffix(s, "ies") + "y"
	}
	if strings.HasSuffix(s, "es") {
		return strings.TrimSuffix(s, "es")
	}
	if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// httpStatusDescription returns a description for an HTTP status code.
func httpStatusDescription(code int) string {
	descriptions := map[int]string{
		200: "OK",
		201: "Created",
		204: "No Content",
		400: "Bad Request",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not Found",
		409: "Conflict",
		422: "Unprocessable Entity",
		500: "Internal Server Error",
	}
	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return "Response"
}
