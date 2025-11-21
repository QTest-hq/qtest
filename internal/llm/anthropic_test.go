package llm

import (
	"testing"
)

// =============================================================================
// sanitizeErrorBody Tests
// =============================================================================

func TestSanitizeErrorBody_NoSensitiveData(t *testing.T) {
	input := "Error: invalid request format"
	result := sanitizeErrorBody(input)

	if result != input {
		t.Errorf("sanitizeErrorBody() = %s, want %s (unchanged)", result, input)
	}
}

func TestSanitizeErrorBody_SkAntKey(t *testing.T) {
	input := "Error: invalid API key sk-ant-abc123xyz789-test"
	result := sanitizeErrorBody(input)

	if result == input {
		t.Error("sanitizeErrorBody() should redact sk-ant-* pattern")
	}
	if result != "Error: invalid API key [REDACTED]" {
		t.Errorf("sanitizeErrorBody() = %s", result)
	}
}

func TestSanitizeErrorBody_SkKey(t *testing.T) {
	input := "Error: key sk-abcdefghij1234567890xyz"
	result := sanitizeErrorBody(input)

	if result == input {
		t.Error("sanitizeErrorBody() should redact sk-* pattern")
	}
	if result != "Error: key [REDACTED]" {
		t.Errorf("sanitizeErrorBody() = %s", result)
	}
}

func TestSanitizeErrorBody_XApiKeyHeader(t *testing.T) {
	input := `{"error": "bad request", "x-api-key": "secret-key-value"}`
	result := sanitizeErrorBody(input)

	if result == input {
		t.Error("sanitizeErrorBody() should redact x-api-key header")
	}
	if result != `{"error": "bad request", [REDACTED]}` {
		t.Errorf("sanitizeErrorBody() = %s", result)
	}
}

func TestSanitizeErrorBody_MultiplePatterns(t *testing.T) {
	input := `sk-ant-api1234 and sk-testkey12345678901234 and "x-api-key": "secret"`
	result := sanitizeErrorBody(input)

	// All patterns should be redacted
	if result == input {
		t.Error("sanitizeErrorBody() should redact all patterns")
	}
}

func TestSanitizeErrorBody_ShortSkKey(t *testing.T) {
	// sk- followed by less than 20 characters should NOT be redacted by second pattern
	input := "Error: sk-short"
	result := sanitizeErrorBody(input)

	// This short key should not match the sk-* pattern (requires 20+ chars)
	if result != input {
		t.Errorf("sanitizeErrorBody() = %s, short sk- key should not be redacted", result)
	}
}

func TestSanitizeErrorBody_EmptyString(t *testing.T) {
	result := sanitizeErrorBody("")

	if result != "" {
		t.Errorf("sanitizeErrorBody() = %s, want empty string", result)
	}
}

// =============================================================================
// AnthropicClient Constructor Tests
// =============================================================================

func TestNewAnthropicClient(t *testing.T) {
	models := map[Tier]string{
		Tier1: "claude-instant-1",
		Tier2: "claude-2",
		Tier3: "claude-2.1",
	}

	client := NewAnthropicClient("test-api-key", models)

	if client == nil {
		t.Fatal("NewAnthropicClient() returned nil")
	}
	if client.apiKey != "test-api-key" {
		t.Errorf("apiKey = %s, want test-api-key", client.apiKey)
	}
	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
	if len(client.models) != 3 {
		t.Errorf("len(models) = %d, want 3", len(client.models))
	}
}

func TestNewAnthropicClient_EmptyModels(t *testing.T) {
	client := NewAnthropicClient("key", nil)

	if client == nil {
		t.Fatal("NewAnthropicClient() returned nil")
	}
	if client.models != nil {
		t.Error("models should be nil when passed nil")
	}
}

func TestNewAnthropicClient_EmptyApiKey(t *testing.T) {
	client := NewAnthropicClient("", map[Tier]string{Tier1: "model"})

	if client == nil {
		t.Fatal("NewAnthropicClient() returned nil")
	}
	if client.apiKey != "" {
		t.Errorf("apiKey = %s, want empty", client.apiKey)
	}
}

// =============================================================================
// AnthropicClient.Name Tests
// =============================================================================

func TestAnthropicClient_Name(t *testing.T) {
	client := NewAnthropicClient("key", nil)

	name := client.Name()

	if name != ProviderAnthropic {
		t.Errorf("Name() = %s, want %s", name, ProviderAnthropic)
	}
}

// =============================================================================
// AnthropicClient.Available Tests
// =============================================================================

func TestAnthropicClient_Available_WithKey(t *testing.T) {
	client := NewAnthropicClient("test-key", nil)

	if !client.Available() {
		t.Error("Available() = false, want true when API key is set")
	}
}

func TestAnthropicClient_Available_WithoutKey(t *testing.T) {
	client := NewAnthropicClient("", nil)

	if client.Available() {
		t.Error("Available() = true, want false when API key is empty")
	}
}

// =============================================================================
// anthropicRequest and anthropicResponse Struct Tests
// =============================================================================

func TestAnthropicRequest_Fields(t *testing.T) {
	req := anthropicRequest{
		Model:     "claude-2",
		MaxTokens: 4096,
		System:    "You are a helpful assistant",
		Messages: []anthropicMessage{
			{Role: "user", Content: "Hello"},
		},
		Temperature:   0.7,
		StopSequences: []string{"END"},
	}

	if req.Model != "claude-2" {
		t.Errorf("Model = %s, want claude-2", req.Model)
	}
	if req.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096", req.MaxTokens)
	}
	if len(req.Messages) != 1 {
		t.Errorf("len(Messages) = %d, want 1", len(req.Messages))
	}
	if req.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", req.Temperature)
	}
}

func TestAnthropicMessage_Fields(t *testing.T) {
	msg := anthropicMessage{
		Role:    "user",
		Content: "Test message",
	}

	if msg.Role != "user" {
		t.Errorf("Role = %s, want user", msg.Role)
	}
	if msg.Content != "Test message" {
		t.Errorf("Content = %s, want Test message", msg.Content)
	}
}

// =============================================================================
// Provider Constant Tests
// =============================================================================

func TestProviderAnthropic_Constant(t *testing.T) {
	if ProviderAnthropic == "" {
		t.Error("ProviderAnthropic should not be empty")
	}
	if string(ProviderAnthropic) != "anthropic" {
		t.Errorf("ProviderAnthropic = %s, want anthropic", ProviderAnthropic)
	}
}

// =============================================================================
// anthropicResponse Struct Tests
// =============================================================================

func TestAnthropicResponse_Fields(t *testing.T) {
	resp := anthropicResponse{
		ID:         "msg_123",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-2",
		StopReason: "end_turn",
	}

	if resp.ID != "msg_123" {
		t.Errorf("ID = %s, want msg_123", resp.ID)
	}
	if resp.Type != "message" {
		t.Errorf("Type = %s, want message", resp.Type)
	}
	if resp.Role != "assistant" {
		t.Errorf("Role = %s, want assistant", resp.Role)
	}
}
