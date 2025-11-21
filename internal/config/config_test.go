package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear relevant env vars to test defaults
	envVars := []string{
		"PORT", "ENV", "DATABASE_URL", "REDIS_URL", "NATS_URL",
		"GITHUB_TOKEN", "LLM_DEFAULT_PROVIDER", "OLLAMA_URL",
		"OLLAMA_TIER1_MODEL", "OLLAMA_TIER2_MODEL",
		"ANTHROPIC_API_KEY", "ANTHROPIC_TIER3_MODEL", "OPENAI_API_KEY",
	}
	for _, v := range envVars {
		t.Setenv(v, "")
		os.Unsetenv(v)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check defaults
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %s, want development", cfg.Env)
	}
	if cfg.DatabaseURL != "postgres://qtest:qtest@localhost:5432/qtest?sslmode=disable" {
		t.Errorf("DatabaseURL = %s, want default", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "redis://localhost:6379" {
		t.Errorf("RedisURL = %s, want redis://localhost:6379", cfg.RedisURL)
	}
	if cfg.NATSURL != "nats://localhost:4222" {
		t.Errorf("NATSURL = %s, want nats://localhost:4222", cfg.NATSURL)
	}
	if cfg.GitHubToken != "" {
		t.Errorf("GitHubToken = %s, want empty", cfg.GitHubToken)
	}
}

func TestLoad_LLMDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.LLM.DefaultProvider != "ollama" {
		t.Errorf("LLM.DefaultProvider = %s, want ollama", cfg.LLM.DefaultProvider)
	}
	if cfg.LLM.OllamaURL != "http://localhost:11434" {
		t.Errorf("LLM.OllamaURL = %s, want http://localhost:11434", cfg.LLM.OllamaURL)
	}
	if cfg.LLM.OllamaTier1 != "qwen2.5-coder:7b" {
		t.Errorf("LLM.OllamaTier1 = %s, want qwen2.5-coder:7b", cfg.LLM.OllamaTier1)
	}
	if cfg.LLM.OllamaTier2 != "deepseek-coder-v2:16b" {
		t.Errorf("LLM.OllamaTier2 = %s, want deepseek-coder-v2:16b", cfg.LLM.OllamaTier2)
	}
	if cfg.LLM.AnthropicTier3 != "claude-3-5-sonnet-20241022" {
		t.Errorf("LLM.AnthropicTier3 = %s, want claude-3-5-sonnet-20241022", cfg.LLM.AnthropicTier3)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/mydb")
	t.Setenv("REDIS_URL", "redis://redis:6379")
	t.Setenv("NATS_URL", "nats://nats:4222")
	t.Setenv("GITHUB_TOKEN", "ghp_test_token")
	t.Setenv("LLM_DEFAULT_PROVIDER", "anthropic")
	t.Setenv("OLLAMA_URL", "http://ollama:11434")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want 9000", cfg.Port)
	}
	if cfg.Env != "production" {
		t.Errorf("Env = %s, want production", cfg.Env)
	}
	if cfg.DatabaseURL != "postgres://user:pass@db:5432/mydb" {
		t.Errorf("DatabaseURL mismatch")
	}
	if cfg.RedisURL != "redis://redis:6379" {
		t.Errorf("RedisURL mismatch")
	}
	if cfg.NATSURL != "nats://nats:4222" {
		t.Errorf("NATSURL mismatch")
	}
	if cfg.GitHubToken != "ghp_test_token" {
		t.Errorf("GitHubToken mismatch")
	}
	if cfg.LLM.DefaultProvider != "anthropic" {
		t.Errorf("LLM.DefaultProvider = %s, want anthropic", cfg.LLM.DefaultProvider)
	}
	if cfg.LLM.OllamaURL != "http://ollama:11434" {
		t.Errorf("LLM.OllamaURL mismatch")
	}
	if cfg.LLM.AnthropicKey != "sk-ant-test" {
		t.Errorf("LLM.AnthropicKey mismatch")
	}
}

func TestValidate_OllamaProvider(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			DefaultProvider: "ollama",
			OllamaURL:       "http://localhost:11434",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_OllamaProvider_NoURL(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			DefaultProvider: "ollama",
			OllamaURL:       "",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should return error when OllamaURL is empty")
	}
}

func TestValidate_AnthropicProvider(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			DefaultProvider: "anthropic",
			AnthropicKey:    "sk-ant-test",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_AnthropicProvider_NoKey(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			DefaultProvider: "anthropic",
			AnthropicKey:    "",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should return error when AnthropicKey is empty")
	}
}

func TestValidate_OtherProvider(t *testing.T) {
	// Other providers (like openai) don't have validation yet
	cfg := &Config{
		LLM: LLMConfig{
			DefaultProvider: "openai",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue string
		want         string
	}{
		{"returns env value", "TEST_VAR_1", "custom", "default", "custom"},
		{"returns default when empty", "TEST_VAR_2", "", "default", "default"},
		{"returns default when unset", "TEST_VAR_UNSET", "", "fallback", "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv(%s, %s) = %s, want %s", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue int
		want         int
	}{
		{"returns parsed int", "TEST_INT_1", "42", 0, 42},
		{"returns default when empty", "TEST_INT_2", "", 100, 100},
		{"returns default when invalid", "TEST_INT_3", "not-a-number", 50, 50},
		{"handles negative numbers", "TEST_INT_4", "-10", 0, -10},
		{"handles zero", "TEST_INT_5", "0", 99, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			}

			got := getEnvInt(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvInt(%s, %d) = %d, want %d", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestConfig_Fields(t *testing.T) {
	cfg := &Config{
		Port:        8080,
		Env:         "test",
		DatabaseURL: "postgres://localhost/test",
		RedisURL:    "redis://localhost",
		NATSURL:     "nats://localhost",
		GitHubToken: "token",
		LLM: LLMConfig{
			DefaultProvider: "ollama",
			OllamaURL:       "http://localhost:11434",
			OllamaTier1:     "model1",
			OllamaTier2:     "model2",
			AnthropicKey:    "key",
			AnthropicTier3:  "model3",
			OpenAIKey:       "openai-key",
		},
	}

	if cfg.Port != 8080 {
		t.Errorf("Port mismatch")
	}
	if cfg.Env != "test" {
		t.Errorf("Env mismatch")
	}
	if cfg.LLM.OpenAIKey != "openai-key" {
		t.Errorf("LLM.OpenAIKey mismatch")
	}
}

func TestLLMConfig_Fields(t *testing.T) {
	llm := LLMConfig{
		DefaultProvider: "anthropic",
		OllamaURL:       "http://ollama:11434",
		OllamaTier1:     "tier1",
		OllamaTier2:     "tier2",
		AnthropicKey:    "ant-key",
		AnthropicTier3:  "tier3",
		OpenAIKey:       "oai-key",
	}

	if llm.DefaultProvider != "anthropic" {
		t.Errorf("DefaultProvider = %s, want anthropic", llm.DefaultProvider)
	}
	if llm.OllamaURL != "http://ollama:11434" {
		t.Errorf("OllamaURL mismatch")
	}
	if llm.AnthropicKey != "ant-key" {
		t.Errorf("AnthropicKey mismatch")
	}
	if llm.OpenAIKey != "oai-key" {
		t.Errorf("OpenAIKey mismatch")
	}
}
