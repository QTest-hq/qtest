package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewOpenAIClient(t *testing.T) {
	models := map[Tier]string{
		Tier1: "gpt-4o-mini",
		Tier2: "gpt-4o",
		Tier3: "gpt-4-turbo",
	}

	client := NewOpenAIClient("test-api-key", models)

	if client == nil {
		t.Fatal("NewOpenAIClient returned nil")
	}

	if client.apiKey != "test-api-key" {
		t.Errorf("apiKey = %s, want test-api-key", client.apiKey)
	}

	if client.baseURL != openaiAPIURL {
		t.Errorf("baseURL = %s, want %s", client.baseURL, openaiAPIURL)
	}

	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestNewOpenAIClientWithURL(t *testing.T) {
	models := map[Tier]string{
		Tier1: "gpt-4o-mini",
	}

	customURL := "https://custom.openai.azure.com/openai/deployments/gpt-4"
	client := NewOpenAIClientWithURL("test-key", customURL, models)

	if client.baseURL != customURL {
		t.Errorf("baseURL = %s, want %s", client.baseURL, customURL)
	}

	// Empty URL should use default
	client2 := NewOpenAIClientWithURL("test-key", "", models)
	if client2.baseURL != openaiAPIURL {
		t.Errorf("baseURL with empty should be default, got %s", client2.baseURL)
	}
}

func TestOpenAIClient_Name(t *testing.T) {
	client := NewOpenAIClient("test-key", nil)

	if client.Name() != ProviderOpenAI {
		t.Errorf("Name() = %s, want %s", client.Name(), ProviderOpenAI)
	}
}

func TestOpenAIClient_Available(t *testing.T) {
	tests := []struct {
		apiKey string
		want   bool
	}{
		{"test-key", true},
		{"", false},
	}

	for _, tt := range tests {
		client := NewOpenAIClient(tt.apiKey, nil)
		if got := client.Available(); got != tt.want {
			t.Errorf("Available() with key %q = %v, want %v", tt.apiKey, got, tt.want)
		}
	}
}

func TestOpenAIClient_Complete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}

		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Authorization header mismatch")
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
		}

		// Parse request to verify structure
		var req openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Model != "gpt-4o-mini" {
			t.Errorf("Model = %s, want gpt-4o-mini", req.Model)
		}

		// Return mock response
		resp := openaiResponse{
			ID:      "chatcmpl-test123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o-mini",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "Test response content",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClientWithURL("test-api-key", server.URL, map[Tier]string{
		Tier1: "gpt-4o-mini",
	})

	req := &Request{
		Tier:   Tier1,
		System: "You are a helpful assistant",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}

	if resp.Content != "Test response content" {
		t.Errorf("Content = %s, want 'Test response content'", resp.Content)
	}

	if resp.Model != "gpt-4o-mini" {
		t.Errorf("Model = %s, want gpt-4o-mini", resp.Model)
	}

	if resp.Provider != ProviderOpenAI {
		t.Errorf("Provider = %s, want %s", resp.Provider, ProviderOpenAI)
	}

	if resp.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", resp.InputTokens)
	}

	if resp.OutputTokens != 20 {
		t.Errorf("OutputTokens = %d, want 20", resp.OutputTokens)
	}

	if resp.FinishReason != "stop" {
		t.Errorf("FinishReason = %s, want stop", resp.FinishReason)
	}
}

func TestOpenAIClient_Complete_WithJSONMode(t *testing.T) {
	var receivedReq openaiRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)

		resp := openaiResponse{
			ID:      "test",
			Model:   "gpt-4o-mini",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Content: `{"key": "value"}`,
					},
					FinishReason: "stop",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClientWithURL("test-key", server.URL, map[Tier]string{
		Tier1: "gpt-4o-mini",
	})

	req := &Request{
		Tier:     Tier1,
		Messages: []Message{{Role: "user", Content: "Generate JSON"}},
		JSONMode: true,
	}

	_, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}

	if receivedReq.ResponseFormat == nil {
		t.Error("ResponseFormat should be set when JSONMode is true")
	} else if receivedReq.ResponseFormat.Type != "json_object" {
		t.Errorf("ResponseFormat.Type = %s, want json_object", receivedReq.ResponseFormat.Type)
	}
}

func TestOpenAIClient_Complete_NoModelForTier(t *testing.T) {
	client := NewOpenAIClient("test-key", map[Tier]string{
		Tier1: "gpt-4o-mini",
	})

	req := &Request{
		Tier:     Tier2, // No model configured for Tier2
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	_, err := client.Complete(context.Background(), req)
	if err == nil {
		t.Error("Expected error for unconfigured tier")
	}

	if !strings.Contains(err.Error(), "no model configured") {
		t.Errorf("Error should mention no model configured, got: %v", err)
	}
}

func TestOpenAIClient_Complete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(openaiError{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{
				Message: "Invalid API key",
				Type:    "invalid_request_error",
				Code:    "invalid_api_key",
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClientWithURL("bad-key", server.URL, map[Tier]string{
		Tier1: "gpt-4o-mini",
	})

	req := &Request{
		Tier:     Tier1,
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	_, err := client.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for API error response")
	}

	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("Error should contain API error message, got: %v", err)
	}

	if !strings.Contains(err.Error(), "invalid_request_error") {
		t.Errorf("Error should contain error type, got: %v", err)
	}
}

func TestOpenAIClient_Complete_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("Rate limit exceeded"))
	}))
	defer server.Close()

	client := NewOpenAIClientWithURL("test-key", server.URL, map[Tier]string{
		Tier1: "gpt-4o-mini",
	})

	req := &Request{
		Tier:     Tier1,
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	_, err := client.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for rate limit")
	}

	if !strings.Contains(err.Error(), "429") {
		t.Errorf("Error should contain status code 429, got: %v", err)
	}
}

func TestOpenAIClient_Complete_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Simulate slow response
		json.NewEncoder(w).Encode(openaiResponse{})
	}))
	defer server.Close()

	client := NewOpenAIClientWithURL("test-key", server.URL, map[Tier]string{
		Tier1: "gpt-4o-mini",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &Request{
		Tier:     Tier1,
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	_, err := client.Complete(ctx, req)
	if err == nil {
		t.Fatal("Expected error for cancelled context")
	}
}

func TestOpenAIClient_Complete_SystemMessage(t *testing.T) {
	var receivedReq openaiRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		json.NewEncoder(w).Encode(openaiResponse{
			Model: "gpt-4o-mini",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{Content: "OK"}},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClientWithURL("test-key", server.URL, map[Tier]string{
		Tier1: "gpt-4o-mini",
	})

	req := &Request{
		Tier:   Tier1,
		System: "You are a test generator",
		Messages: []Message{
			{Role: "user", Content: "Generate tests"},
		},
	}

	_, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}

	// Verify system message was added as first message
	if len(receivedReq.Messages) < 2 {
		t.Fatalf("Expected at least 2 messages, got %d", len(receivedReq.Messages))
	}

	if receivedReq.Messages[0].Role != "system" {
		t.Errorf("First message role = %s, want system", receivedReq.Messages[0].Role)
	}

	if receivedReq.Messages[0].Content != "You are a test generator" {
		t.Errorf("System message content mismatch")
	}
}

func TestOpenAIClient_Complete_DefaultMaxTokens(t *testing.T) {
	var receivedReq openaiRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		json.NewEncoder(w).Encode(openaiResponse{
			Model: "gpt-4o-mini",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{Content: "OK"}},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClientWithURL("test-key", server.URL, map[Tier]string{
		Tier1: "gpt-4o-mini",
	})

	req := &Request{
		Tier:      Tier1,
		Messages:  []Message{{Role: "user", Content: "Hello"}},
		MaxTokens: 0, // Should default to 4096
	}

	_, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}

	if receivedReq.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096 (default)", receivedReq.MaxTokens)
	}
}

func TestDefaultOpenAIModels(t *testing.T) {
	models := DefaultOpenAIModels()

	if models[Tier1] == "" {
		t.Error("Tier1 model should be set")
	}
	if models[Tier2] == "" {
		t.Error("Tier2 model should be set")
	}
	if models[Tier3] == "" {
		t.Error("Tier3 model should be set")
	}
}

func TestSanitizeOpenAIError(t *testing.T) {
	tests := []struct {
		input    string
		contains string
		absent   string
	}{
		{
			input:    `{"error": "sk-proj-abc123xyz789 is invalid"}`,
			contains: "[REDACTED]",
			absent:   "sk-proj-abc123xyz789",
		},
		{
			input:    `api_key: "sk-1234567890abcdefghij"`,
			contains: "[REDACTED]",
			absent:   "sk-1234567890abcdefghij",
		},
		{
			input:    `Just a normal error message`,
			contains: "normal error message",
			absent:   "",
		},
	}

	for _, tt := range tests {
		result := sanitizeOpenAIError(tt.input)

		if !strings.Contains(result, tt.contains) {
			t.Errorf("sanitizeOpenAIError(%q) should contain %q", tt.input, tt.contains)
		}

		if tt.absent != "" && strings.Contains(result, tt.absent) {
			t.Errorf("sanitizeOpenAIError(%q) should not contain %q", tt.input, tt.absent)
		}
	}
}

func TestOpenAIClient_Complete_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openaiResponse{
			Model:   "gpt-4o-mini",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{}, // Empty choices
		})
	}))
	defer server.Close()

	client := NewOpenAIClientWithURL("test-key", server.URL, map[Tier]string{
		Tier1: "gpt-4o-mini",
	})

	req := &Request{
		Tier:     Tier1,
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}

	// Should handle empty choices gracefully
	if resp.Content != "" {
		t.Errorf("Content should be empty for no choices, got %q", resp.Content)
	}
}
