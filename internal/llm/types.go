package llm

import "context"

// Provider represents an LLM provider
type Provider string

const (
	ProviderOllama    Provider = "ollama"
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
)

// Tier represents the LLM tier for routing
type Tier int

const (
	Tier1 Tier = 1 // Fast, cheap - boilerplate, summaries
	Tier2 Tier = 2 // Balanced - test logic, assertions
	Tier3 Tier = 3 // Thorough - complex reasoning, critics
)

// Request represents an LLM completion request
type Request struct {
	Tier        Tier
	System      string
	Messages    []Message
	MaxTokens   int
	Temperature float64
	Stop        []string
	JSONMode    bool // Force JSON output (supported by Ollama)
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response represents an LLM completion response
type Response struct {
	Content      string
	Model        string
	Provider     Provider
	InputTokens  int
	OutputTokens int
	FinishReason string
	Cached       bool // True if response was served from cache
}

// Client is the interface for LLM providers
type Client interface {
	Complete(ctx context.Context, req *Request) (*Response, error)
	Name() Provider
	Available() bool
}

// RouterConfig holds router configuration
type RouterConfig struct {
	DefaultProvider Provider
	Providers       map[Provider]ProviderConfig
	TierModels      map[Tier]map[Provider]string
}

// ProviderConfig holds provider-specific configuration
type ProviderConfig struct {
	Enabled bool
	BaseURL string
	APIKey  string
}
