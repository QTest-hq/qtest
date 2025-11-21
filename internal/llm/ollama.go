package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaClient implements the Client interface for Ollama
type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
	models     map[Tier]string
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL string, models map[Tier]string) *OllamaClient {
	return &OllamaClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // LLM calls can be slow
		},
		models: models,
	}
}

func (c *OllamaClient) Name() Provider {
	return ProviderOllama
}

func (c *OllamaClient) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// ollamaRequest represents the Ollama API request format
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64  `json:"temperature,omitempty"`
	NumPredict  int      `json:"num_predict,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

// ollamaResponse represents the Ollama API response format
type ollamaResponse struct {
	Model           string        `json:"model"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	DoneReason      string        `json:"done_reason,omitempty"`
	PromptEvalCount int           `json:"prompt_eval_count,omitempty"`
	EvalCount       int           `json:"eval_count,omitempty"`
}

func (c *OllamaClient) Complete(ctx context.Context, req *Request) (*Response, error) {
	model, ok := c.models[req.Tier]
	if !ok {
		return nil, fmt.Errorf("no model configured for tier %d", req.Tier)
	}

	// Build messages
	messages := make([]ollamaMessage, 0, len(req.Messages)+1)

	// Add system message if present
	if req.System != "" {
		messages = append(messages, ollamaMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	// Add user/assistant messages
	for _, m := range req.Messages {
		messages = append(messages, ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Build request
	ollamaReq := ollamaRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	// Add options if specified
	if req.Temperature > 0 || req.MaxTokens > 0 || len(req.Stop) > 0 {
		ollamaReq.Options = &ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
			Stop:        req.Stop,
		}
	}

	// Serialize request
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response with context awareness
	var ollamaResp ollamaResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&ollamaResp); err != nil {
		// Check if decoding failed due to context cancellation
		if ctx.Err() != nil {
			return nil, fmt.Errorf("decoding interrupted: %w", ctx.Err())
		}
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &Response{
		Content:      ollamaResp.Message.Content,
		Model:        ollamaResp.Model,
		Provider:     ProviderOllama,
		InputTokens:  ollamaResp.PromptEvalCount,
		OutputTokens: ollamaResp.EvalCount,
		FinishReason: ollamaResp.DoneReason,
	}, nil
}

// ListModels returns available models from Ollama
func (c *OllamaClient) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]string, len(result.Models))
	for i, m := range result.Models {
		models[i] = m.Name
	}

	return models, nil
}
