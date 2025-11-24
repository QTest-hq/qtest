// Package e2e provides end-to-end test generation capabilities.
package e2e

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/QTest-hq/qtest/internal/api"
)

// EndpointMerger merges API endpoints from different sources.
type EndpointMerger struct {
	config *MergeConfig
}

// MergeConfig configures endpoint merging behavior.
type MergeConfig struct {
	// PreferSource indicates which source to prefer when merging
	// Options: "code", "traffic", "combined"
	PreferSource string

	// MergeSchemas indicates whether to merge schemas from different sources
	MergeSchemas bool

	// MergeExamples indicates whether to combine examples
	MergeExamples bool

	// ConfidenceThreshold is the minimum confidence for traffic-inferred endpoints
	ConfidenceThreshold float64
}

// DefaultMergeConfig returns default merge configuration.
func DefaultMergeConfig() *MergeConfig {
	return &MergeConfig{
		PreferSource:        "combined",
		MergeSchemas:        true,
		MergeExamples:       true,
		ConfidenceThreshold: 0.3,
	}
}

// NewEndpointMerger creates a new endpoint merger.
func NewEndpointMerger(config *MergeConfig) *EndpointMerger {
	if config == nil {
		config = DefaultMergeConfig()
	}
	return &EndpointMerger{config: config}
}

// MergeResult represents the result of merging API specs.
type MergeResult struct {
	Spec       *api.APISpec `json:"spec"`
	Stats      MergeStats   `json:"stats"`
	Warnings   []string     `json:"warnings,omitempty"`
}

// MergeStats contains statistics about the merge operation.
type MergeStats struct {
	CodeEndpoints     int `json:"codeEndpoints"`
	TrafficEndpoints  int `json:"trafficEndpoints"`
	MergedEndpoints   int `json:"mergedEndpoints"`
	NewFromTraffic    int `json:"newFromTraffic"`
	EnrichedFromCode  int `json:"enrichedFromCode"`
	TotalEndpoints    int `json:"totalEndpoints"`
}

// Merge combines endpoints from code analysis and traffic capture.
func (m *EndpointMerger) Merge(codeSpec, trafficSpec *api.APISpec) *MergeResult {
	result := &MergeResult{
		Spec: &api.APISpec{
			ID:        uuid.New().String(),
			Name:      "Merged API",
			Schemas:   make(map[string]api.Schema),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	// Handle nil inputs
	if codeSpec == nil && trafficSpec == nil {
		result.Warnings = append(result.Warnings, "Both specs are nil")
		return result
	}

	if codeSpec == nil {
		result.Spec = trafficSpec
		result.Stats.TrafficEndpoints = len(trafficSpec.Endpoints)
		result.Stats.TotalEndpoints = len(trafficSpec.Endpoints)
		return result
	}

	if trafficSpec == nil {
		result.Spec = codeSpec
		result.Stats.CodeEndpoints = len(codeSpec.Endpoints)
		result.Stats.TotalEndpoints = len(codeSpec.Endpoints)
		return result
	}

	// Set metadata from code spec (more authoritative)
	result.Spec.Name = codeSpec.Name
	result.Spec.Description = codeSpec.Description
	result.Spec.Version = codeSpec.Version
	result.Spec.BaseURL = codeSpec.BaseURL
	if result.Spec.BaseURL == "" {
		result.Spec.BaseURL = trafficSpec.BaseURL
	}

	// Merge auth (prefer code)
	if codeSpec.Auth != nil {
		result.Spec.Auth = codeSpec.Auth
	} else if trafficSpec.Auth != nil {
		result.Spec.Auth = trafficSpec.Auth
	}

	// Build index of code endpoints
	codeIndex := m.buildEndpointIndex(codeSpec.Endpoints)
	trafficIndex := m.buildEndpointIndex(trafficSpec.Endpoints)

	result.Stats.CodeEndpoints = len(codeSpec.Endpoints)
	result.Stats.TrafficEndpoints = len(trafficSpec.Endpoints)

	// Track which endpoints we've processed
	processed := make(map[string]bool)

	// Process code endpoints first
	for key, codeEndpoint := range codeIndex {
		if trafficEndpoint, found := trafficIndex[key]; found {
			// Merge matching endpoints
			merged := m.mergeEndpoints(codeEndpoint, trafficEndpoint)
			result.Spec.Endpoints = append(result.Spec.Endpoints, merged)
			result.Stats.MergedEndpoints++
			processed[key] = true
		} else {
			// Code-only endpoint
			result.Spec.Endpoints = append(result.Spec.Endpoints, *codeEndpoint)
			processed[key] = true
		}
	}

	// Add traffic-only endpoints (if confidence threshold met)
	for key, trafficEndpoint := range trafficIndex {
		if !processed[key] {
			if trafficEndpoint.Confidence >= m.config.ConfidenceThreshold {
				trafficEndpoint.Source = "traffic"
				result.Spec.Endpoints = append(result.Spec.Endpoints, *trafficEndpoint)
				result.Stats.NewFromTraffic++
			} else {
				result.Warnings = append(result.Warnings,
					"Skipped low-confidence endpoint: "+key)
			}
		}
	}

	// Merge schemas
	if m.config.MergeSchemas {
		for name, schema := range codeSpec.Schemas {
			result.Spec.Schemas[name] = schema
		}
		for name, schema := range trafficSpec.Schemas {
			if _, exists := result.Spec.Schemas[name]; !exists {
				result.Spec.Schemas[name] = schema
			}
		}
	}

	// Sort endpoints
	sort.Slice(result.Spec.Endpoints, func(i, j int) bool {
		if result.Spec.Endpoints[i].Path == result.Spec.Endpoints[j].Path {
			return result.Spec.Endpoints[i].Method < result.Spec.Endpoints[j].Method
		}
		return result.Spec.Endpoints[i].Path < result.Spec.Endpoints[j].Path
	})

	result.Stats.TotalEndpoints = len(result.Spec.Endpoints)

	return result
}

// buildEndpointIndex creates an index of endpoints by method+path.
func (m *EndpointMerger) buildEndpointIndex(endpoints []api.Endpoint) map[string]*api.Endpoint {
	index := make(map[string]*api.Endpoint)
	for i := range endpoints {
		key := m.endpointKey(&endpoints[i])
		index[key] = &endpoints[i]
	}
	return index
}

// endpointKey generates a unique key for an endpoint.
func (m *EndpointMerger) endpointKey(ep *api.Endpoint) string {
	// Normalize path for comparison
	path := m.normalizePath(ep.Path)
	return string(ep.Method) + " " + path
}

// normalizePath normalizes a path for comparison.
func (m *EndpointMerger) normalizePath(path string) string {
	// Normalize different parameter styles
	// e.g., ":id" -> "{id}", "<id>" -> "{id}"
	path = strings.ReplaceAll(path, ":", "{")
	if strings.Contains(path, "{") && !strings.Contains(path, "}") {
		// Handle :param style by adding closing brace before next /
		segments := strings.Split(path, "/")
		for i, seg := range segments {
			if strings.HasPrefix(seg, "{") && !strings.HasSuffix(seg, "}") {
				segments[i] = seg + "}"
			}
		}
		path = strings.Join(segments, "/")
	}

	// Normalize trailing slashes
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}

	return path
}

// mergeEndpoints merges two endpoints from different sources.
func (m *EndpointMerger) mergeEndpoints(code, traffic *api.Endpoint) api.Endpoint {
	merged := api.Endpoint{
		ID:          code.ID,
		Path:        code.Path,
		Method:      code.Method,
		Source:      "combined",
		CreatedAt:   code.CreatedAt,
		Metadata:    make(map[string]string),
	}

	// Prefer code for documentation
	merged.Summary = m.preferNonEmpty(code.Summary, traffic.Summary)
	merged.Description = m.preferNonEmpty(code.Description, traffic.Description)
	merged.Tags = m.mergeTags(code.Tags, traffic.Tags)

	// Merge parameters
	merged.Parameters = m.mergeParameters(code.Parameters, traffic.Parameters)

	// Merge request body
	merged.RequestBody = m.mergeRequestBody(code.RequestBody, traffic.RequestBody)

	// Merge responses
	merged.Responses = m.mergeResponses(code.Responses, traffic.Responses)

	// Prefer code auth
	if code.Auth != nil {
		merged.Auth = code.Auth
	} else {
		merged.Auth = traffic.Auth
	}

	// Merge examples
	if m.config.MergeExamples {
		merged.Examples = m.mergeExamples(code.Examples, traffic.Examples)
	} else {
		merged.Examples = code.Examples
	}

	// Calculate combined confidence
	merged.Confidence = (code.Confidence + traffic.Confidence) / 2
	if code.Confidence > 0 && traffic.Confidence > 0 {
		// Boost confidence if both sources agree
		merged.Confidence = 0.9
	}

	// Track merge in metadata
	merged.Metadata["codeSource"] = code.Source
	merged.Metadata["trafficSource"] = traffic.Source

	return merged
}

// mergeParameters merges parameter lists from two sources.
func (m *EndpointMerger) mergeParameters(code, traffic []api.Parameter) []api.Parameter {
	paramMap := make(map[string]*api.Parameter)

	// Add code parameters first (authoritative)
	for i := range code {
		key := code[i].Name + ":" + string(code[i].Location)
		paramMap[key] = &code[i]
	}

	// Enrich with traffic parameters
	for i := range traffic {
		key := traffic[i].Name + ":" + string(traffic[i].Location)
		if existing, found := paramMap[key]; found {
			// Enrich existing parameter
			if existing.Example == nil && traffic[i].Example != nil {
				existing.Example = traffic[i].Example
			}
			if existing.Default == nil && traffic[i].Default != nil {
				existing.Default = traffic[i].Default
			}
		} else {
			// New parameter from traffic
			paramMap[key] = &traffic[i]
		}
	}

	// Convert back to slice
	var result []api.Parameter
	for _, p := range paramMap {
		result = append(result, *p)
	}

	// Sort by location then name
	sort.Slice(result, func(i, j int) bool {
		if result[i].Location == result[j].Location {
			return result[i].Name < result[j].Name
		}
		return result[i].Location < result[j].Location
	})

	return result
}

// mergeRequestBody merges request bodies from two sources.
func (m *EndpointMerger) mergeRequestBody(code, traffic *api.RequestBody) *api.RequestBody {
	if code == nil {
		return traffic
	}
	if traffic == nil {
		return code
	}

	merged := &api.RequestBody{
		ContentType: code.ContentType,
		Required:    code.Required || traffic.Required,
	}

	// Merge schemas
	merged.Schema = m.mergeSchema(code.Schema, traffic.Schema)

	// Prefer traffic example (real data)
	if traffic.Example != nil {
		merged.Example = traffic.Example
	} else {
		merged.Example = code.Example
	}

	return merged
}

// mergeResponses merges response lists from two sources.
func (m *EndpointMerger) mergeResponses(code, traffic []api.Response) []api.Response {
	respMap := make(map[int]*api.Response)

	// Add code responses
	for i := range code {
		respMap[code[i].StatusCode] = &code[i]
	}

	// Merge with traffic responses
	for i := range traffic {
		if existing, found := respMap[traffic[i].StatusCode]; found {
			// Enrich schema with traffic data
			existing.Schema = m.mergeSchema(existing.Schema, traffic[i].Schema)
			// Use traffic example if available
			if traffic[i].Example != nil {
				existing.Example = traffic[i].Example
			}
		} else {
			// New response from traffic
			respMap[traffic[i].StatusCode] = &traffic[i]
		}
	}

	// Convert to slice and sort
	var result []api.Response
	for _, r := range respMap {
		result = append(result, *r)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StatusCode < result[j].StatusCode
	})

	return result
}

// mergeSchema merges two schemas.
func (m *EndpointMerger) mergeSchema(code, traffic *api.Schema) *api.Schema {
	if code == nil {
		return traffic
	}
	if traffic == nil {
		return code
	}

	merged := &api.Schema{
		Type:       code.Type,
		Format:     code.Format,
		Required:   code.Required,
		Properties: make(map[string]*api.Schema),
	}

	// Merge properties
	if code.Properties != nil {
		for name, prop := range code.Properties {
			merged.Properties[name] = prop
		}
	}
	if traffic.Properties != nil {
		for name, prop := range traffic.Properties {
			if existing, found := merged.Properties[name]; found {
				// Recursively merge
				merged.Properties[name] = m.mergeSchema(existing, prop)
			} else {
				merged.Properties[name] = prop
			}
		}
	}

	// Merge items for arrays
	merged.Items = m.mergeSchema(code.Items, traffic.Items)

	// Use traffic example if available
	if traffic.Example != nil {
		merged.Example = traffic.Example
	} else {
		merged.Example = code.Example
	}

	return merged
}

// mergeExamples combines examples from both sources.
func (m *EndpointMerger) mergeExamples(code, traffic []api.RequestExample) []api.RequestExample {
	var result []api.RequestExample
	result = append(result, code...)

	// Add traffic examples with "Captured" prefix
	for _, ex := range traffic {
		if !strings.HasPrefix(ex.Name, "Captured") {
			ex.Name = "Captured: " + ex.Name
		}
		result = append(result, ex)
	}

	return result
}

// mergeTags combines tags from both sources.
func (m *EndpointMerger) mergeTags(a, b []string) []string {
	tagSet := make(map[string]bool)
	for _, t := range a {
		tagSet[t] = true
	}
	for _, t := range b {
		tagSet[t] = true
	}

	var result []string
	for t := range tagSet {
		result = append(result, t)
	}
	sort.Strings(result)
	return result
}

// preferNonEmpty returns the first non-empty string.
func (m *EndpointMerger) preferNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
