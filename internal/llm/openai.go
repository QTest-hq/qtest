package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

const openaiAPIURL = "https://api.openai.com/v1/chat/completions"

// sanitizeOpenAIError removes potentially sensitive data from error responses
func sanitizeOpenAIError(body string) string {
	// Remove anything that looks like an API key (sk-*, api key patterns)
	patterns := []string{
		`sk-[a-zA-Z0-9_-]{8,}`,
		`"api_key"\s*:\s*"[^"]+"`}

	result := body
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, "[REDACTED]")
	}

	return result
}

// OpenAIClient implements the Client interface for OpenAI
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	models     map[Tier]string
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey string, models map[Tier]string) *OpenAIClient {
	return &OpenAIClient{
		apiKey:  apiKey,
		baseURL: openaiAPIURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		models: models,
	}
}

// NewOpenAIClientWithURL creates a new OpenAI client with a custom base URL
// Useful for Azure OpenAI or other OpenAI-compatible APIs
func NewOpenAIClientWithURL(apiKey, baseURL string, models map[Tier]string) *OpenAIClient {
	client := NewOpenAIClient(apiKey, models)
	if baseURL != "" {
		client.baseURL = baseURL
	}
	return client
}

func (c *OpenAIClient) Name() Provider {
	return ProviderOpenAI
}

func (c *OpenAIClient) Available() bool {
	return c.apiKey != ""
}

// openaiRequest represents the OpenAI API request format
type openaiRequest struct {
	Model            string          `json:"model"`
	Messages         []openaiMessage `json:"messages"`
	MaxTokens        int             `json:"max_tokens,omitempty"`
	Temperature      float64         `json:"temperature,omitempty"`
	Stop             []string        `json:"stop,omitempty"`
	ResponseFormat   *responseFormat `json:"response_format,omitempty"`
	FrequencyPenalty float64         `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64         `json:"presence_penalty,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

// openaiResponse represents the OpenAI API response format
type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openaiError represents the OpenAI API error format
type openaiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (c *OpenAIClient) Complete(ctx context.Context, req *Request) (*Response, error) {
	model, ok := c.models[req.Tier]
	if !ok {
		return nil, fmt.Errorf("no model configured for tier %d", req.Tier)
	}

	// Build messages - OpenAI uses system as a message with role "system"
	messages := make([]openaiMessage, 0, len(req.Messages)+1)

	// Add system message if present
	if req.System != "" {
		messages = append(messages, openaiMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	// Add user/assistant messages
	for _, m := range req.Messages {
		messages = append(messages, openaiMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Build request
	openaiReq := openaiRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Stop:        req.Stop,
	}

	// Enable JSON mode if requested (supported by GPT-4 and GPT-3.5-turbo)
	if req.JSONMode {
		openaiReq.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	// Serialize request
	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check context after request completes
	if ctx.Err() != nil {
		return nil, fmt.Errorf("context cancelled after request: %w", ctx.Err())
	}

	if resp.StatusCode != http.StatusOK {
		// Limit error body reading to 1KB to prevent memory issues
		limitedReader := io.LimitReader(resp.Body, 1024)
		bodyBytes, _ := io.ReadAll(limitedReader)

		// Try to parse as OpenAI error
		var oaiErr openaiError
		if err := json.Unmarshal(bodyBytes, &oaiErr); err == nil && oaiErr.Error.Message != "" {
			return nil, fmt.Errorf("openai API error (status %d): %s (%s)", resp.StatusCode, oaiErr.Error.Message, oaiErr.Error.Type)
		}

		// Sanitize error message
		return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, sanitizeOpenAIError(string(bodyBytes)))
	}

	// Parse response with context awareness
	var openaiResp openaiResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&openaiResp); err != nil {
		// Check if decoding failed due to context cancellation
		if ctx.Err() != nil {
			return nil, fmt.Errorf("decoding interrupted: %w", ctx.Err())
		}
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract content from first choice
	var content string
	var finishReason string
	if len(openaiResp.Choices) > 0 {
		content = openaiResp.Choices[0].Message.Content
		finishReason = openaiResp.Choices[0].FinishReason
	}

	return &Response{
		Content:      content,
		Model:        openaiResp.Model,
		Provider:     ProviderOpenAI,
		InputTokens:  openaiResp.Usage.PromptTokens,
		OutputTokens: openaiResp.Usage.CompletionTokens,
		FinishReason: finishReason,
	}, nil
}

// DefaultOpenAIModels returns the default model configuration for OpenAI
func DefaultOpenAIModels() map[Tier]string {
	return map[Tier]string{
		Tier1: "gpt-4o-mini",     // Fast, cheap - good for boilerplate
		Tier2: "gpt-4o",          // Balanced - good for test logic
		Tier3: "gpt-4-turbo",     // Thorough - good for complex reasoning
	}
}
