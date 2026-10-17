// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
)

// LLMProvider is the name of the configured LLM backend.
type LLMProvider string

const (
	ProviderGemini LLMProvider = "gemini"
	ProviderOllama LLMProvider = "ollama"
	ProviderNone   LLMProvider = "none"
)

// Rate limit constants for WebSocket connections and messaging.
const (
	RateLimitConnMessagesPerSec = 5
	RateLimitConnBurst          = 10
	RateLimitIPUpgradesPerMin   = 10
	RateLimitIPBurst            = 10
)

// Config holds all runtime configuration for gochat-assistant.
// Values are populated from environment variables; see Load for defaults.
type Config struct {
	// HTTP
	Port           string
	AllowedOrigins []string

	// Logging
	LogLevel string

	// LLM
	LLMProvider  LLMProvider
	GeminiAPIKey string
	GeminiModel  string
	OllamaURL    string
	OllamaModel  string

	// Database
	DatabaseURL string
}

// Load reads environment variables and returns a Config with defaults applied.
// It does NOT validate the configuration; call Validate after Load.
func Load() *Config {
	c := &Config{
		Port:        getEnv("PORT", "8080"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		LLMProvider: LLMProvider(getEnv("LLM_PROVIDER", string(ProviderNone))),
		GeminiModel: getEnv("GEMINI_MODEL", "gemini-1.5-flash"),
		OllamaModel: getEnv("OLLAMA_MODEL", "llama3"),
		OllamaURL:   getEnv("OLLAMA_URL", "http://localhost:11434"),

		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
	}

	if raw := os.Getenv("ALLOWED_ORIGINS"); raw != "" {
		c.AllowedOrigins = splitTrim(raw, ",")
	}

	return c
}

// Validate returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	switch c.LLMProvider {
	case ProviderGemini:
		if c.GeminiAPIKey == "" {
			return fmt.Errorf("config: LLM_PROVIDER=gemini requires GEMINI_API_KEY to be set")
		}
	case ProviderOllama:
		if c.OllamaURL == "" {
			return fmt.Errorf("config: LLM_PROVIDER=ollama requires OLLAMA_URL to be set")
		}
	case ProviderNone:
		// valid – rules-only mode
	default:
		return fmt.Errorf("config: unknown LLM_PROVIDER %q; valid values: gemini, ollama, none", c.LLMProvider)
	}

	if c.Port == "" {
		return fmt.Errorf("config: PORT must not be empty")
	}

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[strings.ToLower(c.LogLevel)] {
		return fmt.Errorf("config: unknown LOG_LEVEL %q; valid values: debug, info, warn, error", c.LogLevel)
	}

	return nil
}

// getEnv returns the value of the environment variable named by key,
// or fallback if the variable is unset or empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// splitTrim splits s by sep and trims whitespace from each element.
func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
