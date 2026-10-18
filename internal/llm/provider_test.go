package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/ratuhin1122/gochat-assistant/internal/config"
)

func TestNewProvider_Factory(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		p, err := NewProvider(nil)
		if err == nil {
			t.Fatal("expected error for nil config, got nil")
		}
		if p != nil {
			t.Errorf("expected nil provider, got %v", p)
		}
	})

	t.Run("none provider returns Noop", func(t *testing.T) {
		cfg := &config.Config{
			LLMProvider: config.ProviderNone,
		}
		p, err := NewProvider(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Name() != "noop" {
			t.Errorf("expected name 'noop', got %q", p.Name())
		}

		reply, err := p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hello"}})
		if err != nil {
			t.Fatalf("unexpected generate error: %v", err)
		}
		if reply != "" {
			t.Errorf("expected empty reply from Noop, got %q", reply)
		}
	})

	t.Run("gemini provider with missing api key", func(t *testing.T) {
		cfg := &config.Config{
			LLMProvider:  config.ProviderGemini,
			GeminiAPIKey: "",
		}
		_, err := NewProvider(cfg)
		if err == nil {
			t.Fatal("expected error when GEMINI_API_KEY is empty")
		}
	})

	t.Run("gemini provider with valid api key", func(t *testing.T) {
		cfg := &config.Config{
			LLMProvider:  config.ProviderGemini,
			GeminiAPIKey: "fake-test-key",
			GeminiModel:  "gemini-1.5-flash",
		}
		p, err := NewProvider(cfg)
		if err != nil {
			t.Fatalf("unexpected error creating Gemini provider: %v", err)
		}
		if p == nil {
			t.Fatal("expected non-nil provider")
		}
		if p.Name() != "gemini" {
			t.Errorf("expected Name() == 'gemini', got %q", p.Name())
		}
	})

	t.Run("ollama provider returns ErrNotImplemented", func(t *testing.T) {
		cfg := &config.Config{
			LLMProvider: config.ProviderOllama,
		}
		p, err := NewProvider(cfg)
		if err == nil {
			t.Fatal("expected ErrNotImplemented for ollama, got nil")
		}
		if !errors.Is(err, ErrNotImplemented) {
			t.Errorf("expected ErrNotImplemented, got %v", err)
		}
		if p != nil {
			t.Errorf("expected nil provider, got %v", p)
		}
	})

	t.Run("unknown provider returns ErrNotImplemented", func(t *testing.T) {
		cfg := &config.Config{
			LLMProvider: "unsupported",
		}
		p, err := NewProvider(cfg)
		if err == nil {
			t.Fatal("expected ErrNotImplemented for unknown provider, got nil")
		}
		if !errors.Is(err, ErrNotImplemented) {
			t.Errorf("expected ErrNotImplemented, got %v", err)
		}
		if p != nil {
			t.Errorf("expected nil provider, got %v", p)
		}
	})
}
