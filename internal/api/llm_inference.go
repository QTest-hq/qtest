package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/QTest-hq/qtest/internal/llm"
)

// LLMInference uses LLM to infer API endpoints from HTML/JS code.
type LLMInference struct {
	router *llm.Router
	config *InferenceConfig
}

// NewLLMInference creates a new LLM-based inference engine.
func NewLLMInference(router *llm.Router, config *InferenceConfig) *LLMInference {
	if config == nil {
		config = DefaultInferenceConfig()
	}
	return &LLMInference{
		router: router,
		config: config,
	}
}

// InferFromHTML analyzes HTML content to find API calls.
func (l *LLMInference) InferFromHTML(ctx context.Context, html string, baseURL string) (*InferenceResult, error) {
	prompt := l.buildHTMLPrompt(html)

	response, err := l.router.Complete(ctx, &llm.Request{
		System: apiInferenceSystemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		MaxTokens:   2000,
		Tier:        llm.Tier2,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM inference failed: %w", err)
	}

	return l.parseResponse(response.Content, baseURL)
}

// InferFromJS analyzes JavaScript code to find API calls.
func (l *LLMInference) InferFromJS(ctx context.Context, jsCode string, baseURL string) (*InferenceResult, error) {
	prompt := l.buildJSPrompt(jsCode)

	response, err := l.router.Complete(ctx, &llm.Request{
		System: apiInferenceSystemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		MaxTokens:   2000,
		Tier:        llm.Tier2,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM inference failed: %w", err)
	}

	return l.parseResponse(response.Content, baseURL)
}

// InferFromFetch analyzes fetch/axios calls in code.
func (l *LLMInference) InferFromFetch(ctx context.Context, code string, baseURL string) (*InferenceResult, error) {
	// Extract fetch/axios calls
	fetchCalls := l.extractFetchCalls(code)
	if len(fetchCalls) == 0 {
		return &InferenceResult{
			Spec: &APISpec{
				ID:        uuid.New().String(),
				BaseURL:   baseURL,
				Endpoints: []Endpoint{},
				CreatedAt: time.Now(),
			},
			Source:     "llm",
			Confidence: 0,
			Warnings:   []string{"No fetch/axios calls found"},
		}, nil
	}

	prompt := l.buildFetchPrompt(fetchCalls)

	response, err := l.router.Complete(ctx, &llm.Request{
		System: apiInferenceSystemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		MaxTokens:   2000,
		Tier:        llm.Tier2,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM inference failed: %w", err)
	}

	return l.parseResponse(response.Content, baseURL)
}

// InferFromFormActions analyzes HTML forms to infer API endpoints.
func (l *LLMInference) InferFromFormActions(forms []FormInfo) (*InferenceResult, error) {
	endpoints := make([]Endpoint, 0)

	for _, form := range forms {
		if form.Action == "" || strings.HasPrefix(form.Action, "#") {
			continue
		}

		method := HTTPMethod(strings.ToUpper(form.Method))
		if method == "" {
			method = MethodPOST
		}

		endpoint := Endpoint{
			ID:         uuid.New().String(),
			Path:       form.Action,
			Method:     method,
			Summary:    fmt.Sprintf("Form submission: %s", form.ID),
			Source:     "llm",
			Confidence: 0.6,
			CreatedAt:  time.Now(),
		}

		// Infer parameters from form fields
		for _, field := range form.Fields {
			param := Parameter{
				Name:     field.Name,
				Location: ParamLocationBody,
				Type:     l.fieldTypeToDataType(field.Type),
				Required: field.Required,
			}
			if field.Placeholder != "" {
				param.Description = field.Placeholder
			}
			endpoint.Parameters = append(endpoint.Parameters, param)
		}

		// Set request body
		if method == MethodPOST || method == MethodPUT {
			endpoint.RequestBody = &RequestBody{
				ContentType: ContentTypeForm,
				Required:    true,
			}
		}

		endpoints = append(endpoints, endpoint)
	}

	return &InferenceResult{
		Spec: &APISpec{
			ID:        uuid.New().String(),
			Endpoints: endpoints,
			CreatedAt: time.Now(),
		},
		Source:     "llm",
		Confidence: 0.6,
	}, nil
}

// FormInfo represents form information for inference.
type FormInfo struct {
	ID     string
	Action string
	Method string
	Fields []FormFieldInfo
}

// FormFieldInfo represents form field information.
type FormFieldInfo struct {
	Name        string
	Type        string
	Required    bool
	Placeholder string
}

func (l *LLMInference) buildHTMLPrompt(html string) string {
	// Truncate if too long
	if len(html) > 10000 {
		html = html[:10000] + "..."
	}

	return fmt.Sprintf(`Analyze this HTML and identify any API endpoints that might be called.
Look for:
1. Form actions with API endpoints
2. Data attributes with URLs
3. JavaScript that makes API calls
4. Links to API documentation

HTML:
%s

Return a JSON array of endpoints found:
[
  {
    "path": "/api/endpoint",
    "method": "POST",
    "description": "What this endpoint does",
    "parameters": [{"name": "param", "type": "string", "required": true}]
  }
]`, html)
}

func (l *LLMInference) buildJSPrompt(jsCode string) string {
	// Truncate if too long
	if len(jsCode) > 15000 {
		jsCode = jsCode[:15000] + "..."
	}

	return fmt.Sprintf(`Analyze this JavaScript code and identify all API endpoints being called.
Look for:
1. fetch() calls
2. axios requests
3. XMLHttpRequest usage
4. jQuery $.ajax calls
5. API client method calls

JavaScript:
%s

Return a JSON array of endpoints found:
[
  {
    "path": "/api/endpoint",
    "method": "POST",
    "description": "What this endpoint does",
    "parameters": [{"name": "param", "type": "string", "required": true}],
    "requestBody": {"contentType": "application/json", "fields": ["field1", "field2"]}
  }
]`, jsCode)
}

func (l *LLMInference) buildFetchPrompt(fetchCalls []string) string {
	calls := strings.Join(fetchCalls, "\n\n")

	return fmt.Sprintf(`Analyze these API calls and extract the endpoint information:

%s

For each call, identify:
1. The HTTP method
2. The endpoint path
3. Request parameters (query, path, body)
4. Request body structure

Return a JSON array of endpoints:
[
  {
    "path": "/api/endpoint",
    "method": "POST",
    "description": "What this endpoint does",
    "parameters": [{"name": "param", "type": "string", "in": "query"}],
    "requestBody": {"contentType": "application/json", "schema": {"field1": "string"}}
  }
]`, calls)
}

func (l *LLMInference) extractFetchCalls(code string) []string {
	var calls []string

	// Simple regex-based extraction for common patterns
	patterns := []string{
		`fetch\s*\([^)]+\)`,
		`axios\.[a-z]+\s*\([^)]+\)`,
		`\$\.ajax\s*\([^)]+\)`,
		`\.get\s*\(['"]/api[^)]+\)`,
		`\.post\s*\(['"]/api[^)]+\)`,
		`\.put\s*\(['"]/api[^)]+\)`,
		`\.delete\s*\(['"]/api[^)]+\)`,
	}

	// Simplified extraction - look for fetch/axios calls
	_ = patterns // patterns define what we're looking for

	start := 0
	for {
		idx := strings.Index(code[start:], "fetch")
		if idx == -1 {
			idx = strings.Index(code[start:], "axios")
		}
		if idx == -1 {
			break
		}

		// Extract the call (up to 500 chars)
		endIdx := start + idx + 500
		if endIdx > len(code) {
			endIdx = len(code)
		}
		snippet := code[start+idx : endIdx]

		// Find closing parenthesis
		depth := 0
		end := 0
		for i, c := range snippet {
			if c == '(' {
				depth++
			} else if c == ')' {
				depth--
				if depth == 0 {
					end = i + 1
					break
				}
			}
		}

		if end > 0 {
			calls = append(calls, snippet[:end])
		}

		start = start + idx + 1
	}

	return calls
}

func (l *LLMInference) parseResponse(response string, baseURL string) (*InferenceResult, error) {
	// Extract JSON from response
	jsonStr := extractJSONFromResponse(response)
	if jsonStr == "" {
		return &InferenceResult{
			Spec: &APISpec{
				ID:        uuid.New().String(),
				BaseURL:   baseURL,
				Endpoints: []Endpoint{},
				CreatedAt: time.Now(),
			},
			Source:     "llm",
			Confidence: 0,
			Warnings:   []string{"No valid JSON found in LLM response"},
		}, nil
	}

	var rawEndpoints []struct {
		Path        string `json:"path"`
		Method      string `json:"method"`
		Description string `json:"description"`
		Parameters  []struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			In       string `json:"in"`
			Required bool   `json:"required"`
		} `json:"parameters,omitempty"`
		RequestBody *struct {
			ContentType string                 `json:"contentType"`
			Schema      map[string]interface{} `json:"schema,omitempty"`
			Fields      []string               `json:"fields,omitempty"`
		} `json:"requestBody,omitempty"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawEndpoints); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	endpoints := make([]Endpoint, 0, len(rawEndpoints))

	for _, raw := range rawEndpoints {
		endpoint := Endpoint{
			ID:          uuid.New().String(),
			Path:        raw.Path,
			Method:      HTTPMethod(strings.ToUpper(raw.Method)),
			Description: raw.Description,
			Summary:     raw.Description,
			Source:      "llm",
			Confidence:  0.7,
			CreatedAt:   time.Now(),
		}

		// Convert parameters
		for _, p := range raw.Parameters {
			param := Parameter{
				Name:     p.Name,
				Type:     DataType(p.Type),
				Required: p.Required,
			}

			switch p.In {
			case "query":
				param.Location = ParamLocationQuery
			case "path":
				param.Location = ParamLocationPath
			case "header":
				param.Location = ParamLocationHeader
			case "body":
				param.Location = ParamLocationBody
			default:
				param.Location = ParamLocationQuery
			}

			endpoint.Parameters = append(endpoint.Parameters, param)
		}

		// Convert request body
		if raw.RequestBody != nil {
			endpoint.RequestBody = &RequestBody{
				ContentType: ContentType(raw.RequestBody.ContentType),
				Required:    true,
			}

			if raw.RequestBody.Schema != nil {
				endpoint.RequestBody.Schema = &Schema{
					Type:       DataTypeObject,
					Properties: make(map[string]*Schema),
				}
				for key := range raw.RequestBody.Schema {
					endpoint.RequestBody.Schema.Properties[key] = &Schema{Type: DataTypeString}
				}
			}
		}

		endpoints = append(endpoints, endpoint)
	}

	return &InferenceResult{
		Spec: &APISpec{
			ID:        uuid.New().String(),
			BaseURL:   baseURL,
			Endpoints: endpoints,
			CreatedAt: time.Now(),
		},
		Source:     "llm",
		Confidence: l.calculateConfidence(endpoints),
	}, nil
}

func (l *LLMInference) fieldTypeToDataType(fieldType string) DataType {
	switch strings.ToLower(fieldType) {
	case "number", "range":
		return DataTypeNumber
	case "checkbox":
		return DataTypeBoolean
	case "email", "tel", "url", "text", "password", "hidden":
		return DataTypeString
	default:
		return DataTypeString
	}
}

func (l *LLMInference) calculateConfidence(endpoints []Endpoint) float64 {
	if len(endpoints) == 0 {
		return 0
	}
	// LLM inference has lower confidence than explicit specs
	return 0.7
}

func extractJSONFromResponse(s string) string {
	// Find JSON array or object
	start := strings.Index(s, "[")
	if start == -1 {
		start = strings.Index(s, "{")
	}
	if start == -1 {
		return ""
	}

	openBracket := s[start]
	closeBracket := byte(']')
	if openBracket == '{' {
		closeBracket = '}'
	}

	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == openBracket {
			depth++
		} else if s[i] == closeBracket {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
}

const apiInferenceSystemPrompt = `You are an API endpoint analyzer. Your task is to identify API endpoints from code and HTML.

When analyzing:
1. Look for HTTP requests (fetch, axios, XMLHttpRequest, jQuery ajax)
2. Identify the HTTP method (GET, POST, PUT, DELETE, etc.)
3. Extract the endpoint path
4. Identify request parameters (query params, path params, body)
5. Infer the purpose of each endpoint

Always return valid JSON. Be precise about endpoint paths and methods.
If you're uncertain about an endpoint, indicate lower confidence.`
