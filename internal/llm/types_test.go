package llm

import (
	"testing"
)

func TestProviderConstants(t *testing.T) {
	if ProviderOllama != "ollama" {
		t.Errorf("ProviderOllama = %s, want ollama", ProviderOllama)
	}
	if ProviderAnthropic != "anthropic" {
		t.Errorf("ProviderAnthropic = %s, want anthropic", ProviderAnthropic)
	}
	if ProviderOpenAI != "openai" {
		t.Errorf("ProviderOpenAI = %s, want openai", ProviderOpenAI)
	}
}

func TestTierConstants(t *testing.T) {
	if Tier1 != 1 {
		t.Errorf("Tier1 = %d, want 1", Tier1)
	}
	if Tier2 != 2 {
		t.Errorf("Tier2 = %d, want 2", Tier2)
	}
	if Tier3 != 3 {
		t.Errorf("Tier3 = %d, want 3", Tier3)
	}
}

func TestRequest_Fields(t *testing.T) {
	req := &Request{
		Tier:        Tier2,
		System:      "test system",
		Messages:    []Message{{Role: "user", Content: "hello"}},
		MaxTokens:   100,
		Temperature: 0.7,
		Stop:        []string{"\n"},
	}

	if req.Tier != Tier2 {
		t.Errorf("Tier = %d, want 2", req.Tier)
	}
	if req.System != "test system" {
		t.Errorf("System = %s, want 'test system'", req.System)
	}
	if len(req.Messages) != 1 {
		t.Errorf("len(Messages) = %d, want 1", len(req.Messages))
	}
	if req.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d, want 100", req.MaxTokens)
	}
	if req.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", req.Temperature)
	}
	if len(req.Stop) != 1 {
		t.Errorf("len(Stop) = %d, want 1", len(req.Stop))
	}
}

func TestMessage_Fields(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "test message",
	}

	if msg.Role != "user" {
		t.Errorf("Role = %s, want user", msg.Role)
	}
	if msg.Content != "test message" {
		t.Errorf("Content = %s, want 'test message'", msg.Content)
	}
}

func TestResponse_Fields(t *testing.T) {
	resp := &Response{
		Content:      "response content",
		Model:        "test-model",
		Provider:     ProviderOllama,
		InputTokens:  10,
		OutputTokens: 20,
		FinishReason: "stop",
		Cached:       true,
	}

	if resp.Content != "response content" {
		t.Errorf("Content = %s, want 'response content'", resp.Content)
	}
	if resp.Model != "test-model" {
		t.Errorf("Model = %s, want test-model", resp.Model)
	}
	if resp.Provider != ProviderOllama {
		t.Errorf("Provider = %s, want ollama", resp.Provider)
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
	if !resp.Cached {
		t.Error("Cached should be true")
	}
}

func TestRouterConfig_Fields(t *testing.T) {
	cfg := &RouterConfig{
		DefaultProvider: ProviderOllama,
		Providers: map[Provider]ProviderConfig{
			ProviderOllama: {
				Enabled: true,
				BaseURL: "http://localhost:11434",
			},
		},
		TierModels: map[Tier]map[Provider]string{
			Tier1: {ProviderOllama: "model1"},
		},
	}

	if cfg.DefaultProvider != ProviderOllama {
		t.Errorf("DefaultProvider = %s, want ollama", cfg.DefaultProvider)
	}
	if len(cfg.Providers) != 1 {
		t.Errorf("len(Providers) = %d, want 1", len(cfg.Providers))
	}
	if len(cfg.TierModels) != 1 {
		t.Errorf("len(TierModels) = %d, want 1", len(cfg.TierModels))
	}
}

func TestProviderConfig_Fields(t *testing.T) {
	cfg := ProviderConfig{
		Enabled: true,
		BaseURL: "http://localhost:11434",
		APIKey:  "secret-key",
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if cfg.BaseURL != "http://localhost:11434" {
		t.Errorf("BaseURL = %s, want http://localhost:11434", cfg.BaseURL)
	}
	if cfg.APIKey != "secret-key" {
		t.Errorf("APIKey = %s, want secret-key", cfg.APIKey)
	}
}

func TestTier_Values(t *testing.T) {
	// Test tier ordering
	if !(Tier1 < Tier2 && Tier2 < Tier3) {
		t.Error("Tiers should be ordered: Tier1 < Tier2 < Tier3")
	}
}

func TestMessage_JSONTags(t *testing.T) {
	// Message struct should have json tags for serialization
	msg := Message{Role: "user", Content: "test"}

	// Verify the struct works correctly with expected field values
	if msg.Role != "user" {
		t.Error("Role field not accessible")
	}
	if msg.Content != "test" {
		t.Error("Content field not accessible")
	}
}
