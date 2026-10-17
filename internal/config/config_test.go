package config_test

import (
	"testing"

	"github.com/ratuhin1122/gochat-assistant/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Ensure no env vars leak in from the host during this sub-test.
	clearEnv(t)

	cfg := config.Load()

	if cfg.Port != "8080" {
		t.Errorf("Port: got %q, want %q", cfg.Port, "8080")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.LLMProvider != config.ProviderNone {
		t.Errorf("LLMProvider: got %q, want %q", cfg.LLMProvider, config.ProviderNone)
	}
	if cfg.GeminiModel != "gemini-1.5-flash" {
		t.Errorf("GeminiModel: got %q, want %q", cfg.GeminiModel, "gemini-1.5-flash")
	}
	if cfg.OllamaModel != "llama3" {
		t.Errorf("OllamaModel: got %q, want %q", cfg.OllamaModel, "llama3")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{
			name:    "defaults are valid (none provider)",
			env:     map[string]string{},
			wantErr: false,
		},
		{
			name: "gemini without api key",
			env: map[string]string{
				"LLM_PROVIDER": "gemini",
			},
			wantErr: true,
		},
		{
			name: "gemini with api key",
			env: map[string]string{
				"LLM_PROVIDER":   "gemini",
				"GEMINI_API_KEY": "sk-test",
			},
			wantErr: false,
		},
		{
			name: "ollama with url",
			env: map[string]string{
				"LLM_PROVIDER": "ollama",
				"OLLAMA_URL":   "http://localhost:11434",
			},
			wantErr: false,
		},
		{
			name: "unknown provider",
			env: map[string]string{
				"LLM_PROVIDER": "openai",
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			env: map[string]string{
				"LOG_LEVEL": "verbose",
			},
			wantErr: true,
		},
		{
			name: "valid log level warn",
			env: map[string]string{
				"LOG_LEVEL": "warn",
			},
			wantErr: false,
		},
		{
			name: "valid log level debug",
			env: map[string]string{
				"LOG_LEVEL": "debug",
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			cfg := config.Load()
			err := cfg.Validate()

			if tc.wantErr && err == nil {
				t.Error("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoad_AllowedOrigins(t *testing.T) {
	clearEnv(t)
	t.Setenv("ALLOWED_ORIGINS", "https://a.example.com, https://b.example.com")

	cfg := config.Load()

	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("AllowedOrigins: got %d elements, want 2", len(cfg.AllowedOrigins))
	}
	if cfg.AllowedOrigins[0] != "https://a.example.com" {
		t.Errorf("AllowedOrigins[0]: got %q", cfg.AllowedOrigins[0])
	}
	if cfg.AllowedOrigins[1] != "https://b.example.com" {
		t.Errorf("AllowedOrigins[1]: got %q", cfg.AllowedOrigins[1])
	}
}

// clearEnv unsets all env vars used by config.Load for the duration of the test.
func clearEnv(t *testing.T) {
	t.Helper()
	vars := []string{
		"PORT", "LOG_LEVEL", "ALLOWED_ORIGINS",
		"LLM_PROVIDER", "GEMINI_API_KEY", "GEMINI_MODEL",
		"OLLAMA_URL", "OLLAMA_MODEL", "DATABASE_URL",
	}
	for _, v := range vars {
		t.Setenv(v, "") // t.Setenv restores the original value on cleanup
	}
}
