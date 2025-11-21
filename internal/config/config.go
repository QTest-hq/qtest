package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	// Server
	Port int
	Env  string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// NATS
	NATSURL string

	// LLM
	LLM LLMConfig

	// GitHub
	GitHubToken string
}

// LLMConfig holds LLM-related configuration
type LLMConfig struct {
	// Default provider: ollama, anthropic, openai
	DefaultProvider string

	// Ollama settings
	OllamaURL   string
	OllamaTier1 string
	OllamaTier2 string

	// Anthropic settings
	AnthropicKey   string
	AnthropicTier3 string

	// OpenAI settings (fallback)
	OpenAIKey string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:        getEnvInt("PORT", 8080),
		Env:         getEnv("ENV", "development"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://qtest:qtest@localhost:5432/qtest?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		NATSURL:     getEnv("NATS_URL", "nats://localhost:4222"),
		GitHubToken: getEnv("GITHUB_TOKEN", ""),

		LLM: LLMConfig{
			DefaultProvider: getEnv("LLM_DEFAULT_PROVIDER", "ollama"),
			OllamaURL:       getEnv("OLLAMA_URL", "http://localhost:11434"),
			OllamaTier1:     getEnv("OLLAMA_TIER1_MODEL", "qwen2.5-coder:7b"),
			OllamaTier2:     getEnv("OLLAMA_TIER2_MODEL", "deepseek-coder-v2:16b"),
			AnthropicKey:    getEnv("ANTHROPIC_API_KEY", ""),
			AnthropicTier3:  getEnv("ANTHROPIC_TIER3_MODEL", "claude-3-5-sonnet-20241022"),
			OpenAIKey:       getEnv("OPENAI_API_KEY", ""),
		},
	}

	return cfg, nil
}

// Validate checks if required configuration is present
func (c *Config) Validate() error {
	// LLM validation - need at least one provider
	if c.LLM.DefaultProvider == "ollama" {
		// Ollama is local, just need URL
		if c.LLM.OllamaURL == "" {
			return fmt.Errorf("OLLAMA_URL required when using ollama provider")
		}
	} else if c.LLM.DefaultProvider == "anthropic" {
		if c.LLM.AnthropicKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY required when using anthropic provider")
		}
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
