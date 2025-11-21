package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOllamaClient(t *testing.T) {
	models := map[Tier]string{
		Tier1: "model1",
		Tier2: "model2",
	}

	client := NewOllamaClient("http://localhost:11434", models)

	if client == nil {
		t.Fatal("NewOllamaClient returned nil")
	}
	if client.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %s, want http://localhost:11434", client.baseURL)
	}
	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
	if len(client.models) != 2 {
		t.Errorf("len(models) = %d, want 2", len(client.models))
	}
}

func TestOllamaClient_Name(t *testing.T) {
	client := NewOllamaClient("http://localhost:11434", nil)
	if client.Name() != ProviderOllama {
		t.Errorf("Name() = %s, want ollama", client.Name())
	}
}

func TestOllamaClient_Available(t *testing.T) {
	// Create test server that returns 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"models": []interface{}{}})
		}
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, nil)
	if !client.Available() {
		t.Error("Available() should return true for working server")
	}
}

func TestOllamaClient_Available_ServerDown(t *testing.T) {
	// Use invalid URL
	client := NewOllamaClient("http://localhost:99999", nil)
	if client.Available() {
		t.Error("Available() should return false for unreachable server")
	}
}

func TestOllamaClient_Available_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, nil)
	if client.Available() {
		t.Error("Available() should return false for server error")
	}
}

func TestOllamaClient_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		// Verify request body
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if req.Model != "test-model" {
			t.Errorf("model = %s, want test-model", req.Model)
		}
		if req.Stream {
			t.Error("stream should be false")
		}

		// Return response
		resp := ollamaResponse{
			Model:   "test-model",
			Message: ollamaMessage{Role: "assistant", Content: "test response"},
			Done:    true,
			PromptEvalCount: 10,
			EvalCount:       20,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, map[Tier]string{Tier1: "test-model"})

	req := &Request{
		Tier:     Tier1,
		System:   "You are a test assistant",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	if resp.Content != "test response" {
		t.Errorf("Content = %s, want 'test response'", resp.Content)
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
}

func TestOllamaClient_Complete_NoModelForTier(t *testing.T) {
	client := NewOllamaClient("http://localhost:11434", map[Tier]string{})

	req := &Request{Tier: Tier1}
	_, err := client.Complete(context.Background(), req)

	if err == nil {
		t.Error("Complete() should return error when no model configured")
	}
}

func TestOllamaClient_Complete_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, map[Tier]string{Tier1: "model"})

	req := &Request{Tier: Tier1}
	_, err := client.Complete(context.Background(), req)

	if err == nil {
		t.Error("Complete() should return error on server error")
	}
}

func TestOllamaClient_Complete_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(ollamaResponse{})
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, map[Tier]string{Tier1: "model"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &Request{Tier: Tier1}
	_, err := client.Complete(ctx, req)

	if err == nil {
		t.Error("Complete() should return error on cancelled context")
	}
}

func TestOllamaClient_Complete_WithOptions(t *testing.T) {
	var receivedReq ollamaRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		json.NewEncoder(w).Encode(ollamaResponse{Message: ollamaMessage{Content: "ok"}})
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, map[Tier]string{Tier1: "model"})

	req := &Request{
		Tier:        Tier1,
		Temperature: 0.7,
		MaxTokens:   100,
		Stop:        []string{"\n", "END"},
	}

	_, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	if receivedReq.Options == nil {
		t.Fatal("Options should not be nil")
	}
	if receivedReq.Options.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", receivedReq.Options.Temperature)
	}
	if receivedReq.Options.NumPredict != 100 {
		t.Errorf("NumPredict = %d, want 100", receivedReq.Options.NumPredict)
	}
	if len(receivedReq.Options.Stop) != 2 {
		t.Errorf("len(Stop) = %d, want 2", len(receivedReq.Options.Stop))
	}
}

func TestOllamaClient_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}{
			Models: []struct {
				Name string `json:"name"`
			}{
				{Name: "model1"},
				{Name: "model2"},
				{Name: "model3"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, nil)

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error: %v", err)
	}

	if len(models) != 3 {
		t.Errorf("len(models) = %d, want 3", len(models))
	}
	if models[0] != "model1" {
		t.Errorf("models[0] = %s, want model1", models[0])
	}
}

func TestOllamaClient_ListModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, nil)

	_, err := client.ListModels(context.Background())
	if err == nil {
		t.Error("ListModels() should return error on server error")
	}
}
