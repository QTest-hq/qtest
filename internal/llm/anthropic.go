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

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"

// sanitizeErrorBody removes potentially sensitive data from error responses
func sanitizeErrorBody(body string) string {
	// Remove anything that looks like an API key (sk-ant-*, sk-*, api key patterns)
	patterns := []string{
		`sk-ant-[a-zA-Z0-9_-]+`,
		`sk-[a-zA-Z0-9_-]{20,}`,
		`"x-api-key"\s*:\s*"[^"]+"`}

	result := body
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, "[REDACTED]")
	}

	return result
}

// AnthropicClient implements the Client interface for Anthropic
type AnthropicClient struct {
	apiKey     string
	httpClient *http.Client
	models     map[Tier]string
}

// NewAnthropicClient creates a new Anthropic client
func NewAnthropicClient(apiKey string, models map[Tier]string) *AnthropicClient {
	return &AnthropicClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		models: models,
	}
}

func (c *AnthropicClient) Name() Provider {
	return ProviderAnthropic
}

func (c *AnthropicClient) Available() bool {
	return c.apiKey != ""
}

// anthropicRequest represents the Anthropic API request format
type anthropicRequest struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens"`
	System        string             `json:"system,omitempty"`
	Messages      []anthropicMessage `json:"messages"`
	Temperature   float64            `json:"temperature,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse represents the Anthropic API response format
type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (c *AnthropicClient) Complete(ctx context.Context, req *Request) (*Response, error) {
	model, ok := c.models[req.Tier]
	if !ok {
		return nil, fmt.Errorf("no model configured for tier %d", req.Tier)
	}

	// Build messages
	messages := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, anthropicMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Build request
	anthropicReq := anthropicRequest{
		Model:         model,
		MaxTokens:     maxTokens,
		System:        req.System,
		Messages:      messages,
		Temperature:   req.Temperature,
		StopSequences: req.Stop,
	}

	// Serialize request
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

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
		// Sanitize error message (don't include full request details that might have API key)
		return nil, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, sanitizeErrorBody(string(bodyBytes)))
	}

	// Parse response with context awareness
	var anthropicResp anthropicResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&anthropicResp); err != nil {
		// Check if decoding failed due to context cancellation
		if ctx.Err() != nil {
			return nil, fmt.Errorf("decoding interrupted: %w", ctx.Err())
		}
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract text content
	var content string
	for _, c := range anthropicResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return &Response{
		Content:      content,
		Model:        anthropicResp.Model,
		Provider:     ProviderAnthropic,
		InputTokens:  anthropicResp.Usage.InputTokens,
		OutputTokens: anthropicResp.Usage.OutputTokens,
		FinishReason: anthropicResp.StopReason,
	}, nil
}
