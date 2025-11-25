package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
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

	// GitHub OAuth
	GitHubOAuth GitHubOAuthConfig

	// GitHub App (for organization-wide access)
	GitHubApp GitHubAppConfig

	// Rate Limiting
	RateLimit RateLimitConfig

	// Security
	Security SecurityConfig

	// Telemetry (OpenTelemetry tracing)
	Telemetry TelemetryConfig

	// Security Headers
	SecurityHeaders SecurityHeadersConfig
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	// CORS
	CORSAllowedOrigins   []string
	CORSAllowCredentials bool

	// JWT
	JWTPrivateKeyPath string
	JWTPrivateKeyPEM  string
	JWTPublicKeyPath  string
	JWTIssuer         string
	JWTAccessTTL      time.Duration
	JWTRefreshTTL     time.Duration
	JWTKeyID          string

	// Account lockout
	LockoutMaxAttempts    int
	LockoutDuration       time.Duration
	LockoutWindowDuration time.Duration

	// API Keys
	APIKeyDefaultExpiryDays int
	APIKeyGracePeriod       time.Duration

	// Security headers
	HSTSEnabled bool // Enable HTTP Strict Transport Security (production with HTTPS only)
	HSTSMaxAge  int  // HSTS max-age in seconds (default: 31536000 = 1 year)
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled          bool
	DefaultPerMinute int
	DefaultPerHour   int
	IPPerMinute      int
	IPPerHour        int
	StorageBackend   string // "memory" or "redis"
}

// GitHubOAuthConfig holds GitHub OAuth configuration
type GitHubOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// GitHubAppConfig holds GitHub App configuration for organization-wide access
type GitHubAppConfig struct {
	// AppID is the GitHub App's ID
	AppID int64
	// PrivateKeyPath is the path to the App's private key PEM file
	PrivateKeyPath string
	// PrivateKey is the raw PEM content (alternative to path)
	PrivateKey string
	// WebhookSecret is the secret for validating webhook payloads
	WebhookSecret string
	// Enabled indicates if GitHub App auth is configured
	Enabled bool
}

// TelemetryConfig holds OpenTelemetry tracing configuration
type TelemetryConfig struct {
	Enabled      bool
	OTLPEndpoint string
	SamplingRate float64
	ServiceName  string
}

// SecurityHeadersConfig holds security headers middleware configuration
type SecurityHeadersConfig struct {
	Enabled        bool
	HSTSEnabled    bool
	HSTSMaxAge     int
	FrameOptions   string
	ReferrerPolicy string
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

		GitHubOAuth: GitHubOAuthConfig{
			ClientID:     getEnv("GITHUB_OAUTH_CLIENT_ID", ""),
			ClientSecret: getEnv("GITHUB_OAUTH_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("GITHUB_OAUTH_REDIRECT_URL", "http://localhost:8080/auth/callback"),
		},

		GitHubApp: GitHubAppConfig{
			AppID:          getEnvInt64("GITHUB_APP_ID", 0),
			PrivateKeyPath: getEnv("GITHUB_APP_PRIVATE_KEY_PATH", ""),
			PrivateKey:     getEnv("GITHUB_APP_PRIVATE_KEY", ""),
			WebhookSecret:  getEnv("GITHUB_APP_WEBHOOK_SECRET", ""),
			Enabled:        getEnvInt64("GITHUB_APP_ID", 0) > 0,
		},

		LLM: LLMConfig{
			DefaultProvider: getEnv("LLM_DEFAULT_PROVIDER", "ollama"),
			OllamaURL:       getEnv("OLLAMA_URL", "http://localhost:11434"),
			OllamaTier1:     getEnv("OLLAMA_TIER1_MODEL", "qwen2.5-coder:7b"),
			OllamaTier2:     getEnv("OLLAMA_TIER2_MODEL", "deepseek-coder-v2:16b"),
			AnthropicKey:    getEnv("ANTHROPIC_API_KEY", ""),
			AnthropicTier3:  getEnv("ANTHROPIC_TIER3_MODEL", "claude-3-5-sonnet-20241022"),
			OpenAIKey:       getEnv("OPENAI_API_KEY", ""),
		},

		RateLimit: RateLimitConfig{
			Enabled:          getEnvBool("RATE_LIMIT_ENABLED", true),
			DefaultPerMinute: getEnvInt("RATE_LIMIT_DEFAULT_PER_MINUTE", 100),
			DefaultPerHour:   getEnvInt("RATE_LIMIT_DEFAULT_PER_HOUR", 1000),
			IPPerMinute:      getEnvInt("RATE_LIMIT_IP_PER_MINUTE", 20),
			IPPerHour:        getEnvInt("RATE_LIMIT_IP_PER_HOUR", 100),
			StorageBackend:   getEnv("RATE_LIMIT_STORAGE", "memory"),
		},

		Security: SecurityConfig{
			// CORS - allow localhost by default in development
			CORSAllowedOrigins:   getEnvStringSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:3001", "http://127.0.0.1:3000", "http://127.0.0.1:3001"}),
			CORSAllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", true),

			// JWT configuration
			JWTPrivateKeyPath: getEnv("JWT_PRIVATE_KEY_PATH", ""),
			JWTPrivateKeyPEM:  getEnv("JWT_PRIVATE_KEY_PEM", ""),
			JWTPublicKeyPath:  getEnv("JWT_PUBLIC_KEY_PATH", ""),
			JWTIssuer:         getEnv("JWT_ISSUER", "https://api.qtest.io"),
			JWTAccessTTL:      getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
			JWTRefreshTTL:     getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
			JWTKeyID:          getEnv("JWT_KEY_ID", "v1"),

			// Account lockout
			LockoutMaxAttempts:    getEnvInt("ACCOUNT_LOCKOUT_ATTEMPTS", 5),
			LockoutDuration:       getEnvDuration("ACCOUNT_LOCKOUT_DURATION", 15*time.Minute),
			LockoutWindowDuration: getEnvDuration("ACCOUNT_LOCKOUT_WINDOW", 15*time.Minute),

			// API Keys
			APIKeyDefaultExpiryDays: getEnvInt("API_KEY_DEFAULT_EXPIRY_DAYS", 90),
			APIKeyGracePeriod:       getEnvDuration("API_KEY_ROTATION_GRACE_PERIOD", 24*time.Hour),
		},

		Telemetry: TelemetryConfig{
			Enabled:      getEnvBool("OTEL_ENABLED", false),
			OTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			SamplingRate: getEnvFloat64("OTEL_SAMPLING_RATE", 1.0),
			ServiceName:  getEnv("OTEL_SERVICE_NAME", "qtest"),
		},

		SecurityHeaders: SecurityHeadersConfig{
			Enabled:        getEnvBool("SECURITY_HEADERS_ENABLED", true),
			HSTSEnabled:    getEnvBool("SECURITY_HSTS_ENABLED", false),
			HSTSMaxAge:     getEnvInt("SECURITY_HSTS_MAX_AGE", 31536000),
			FrameOptions:   getEnv("SECURITY_FRAME_OPTIONS", "DENY"),
			ReferrerPolicy: getEnv("SECURITY_REFERRER_POLICY", "strict-origin-when-cross-origin"),
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

	// Production-specific validation
	if c.Env == "production" {
		// Require explicit database URL in production (no defaults with hardcoded creds)
		if c.DatabaseURL == "" {
			return fmt.Errorf("DATABASE_URL required in production")
		}
		// Warn if SSL is disabled in production
		if strings.Contains(c.DatabaseURL, "sslmode=disable") {
			// Log warning but don't fail - some internal deployments may not need SSL
			fmt.Println("WARNING: Database SSL is disabled in production. Consider using sslmode=require")
		}

		// Require explicit CORS origins in production
		if len(c.Security.CORSAllowedOrigins) == 0 {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS required in production")
		}
	}

	return nil
}

// IsProductionMode returns true if running in production
func (c *Config) IsProductionMode() bool {
	return c.Env == "production"
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

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvFloat64(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Split by comma and trim whitespace
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
