// Package llm defines the LLM provider interface and its implementations.
package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/ratuhin1122/gochat-assistant/internal/config"
)

// Role describes who authored a message in a conversation.
type Role string

const (
	// RoleUser represents a message from the human user.
	RoleUser Role = "user"
	// RoleAssistant represents a message from the AI assistant.
	RoleAssistant Role = "assistant"
	// RoleSystem represents an instructional system message.
	RoleSystem Role = "system"
)

// Message is a single turn in a conversation.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Provider is the interface all LLM backends must implement.
type Provider interface {
	// Name returns the human-readable name of the provider (e.g., "noop", "gemini").
	Name() string
	// Generate sends the conversation history to the model and returns its reply.
	Generate(ctx context.Context, messages []Message) (string, error)
}

// ErrNotImplemented is returned by factory stubs for providers that are not yet built.
var ErrNotImplemented = errors.New("llm provider not implemented")

// ── Noop provider ────────────────────────────────────────────────────────────

// Noop is a do-nothing LLM provider used when LLM_PROVIDER=none.
// It always returns an empty string without making any network calls.
type Noop struct{}

// Name implements Provider.
func (Noop) Name() string { return "noop" }

// Generate implements Provider. It always returns an empty string.
func (Noop) Generate(_ context.Context, _ []Message) (string, error) { return "", nil }

// ── Factory ───────────────────────────────────────────────────────────────────

// NewProvider constructs the appropriate Provider for the given config.
func NewProvider(cfg *config.Config) (Provider, error) {
	if cfg == nil {
		return nil, errors.New("llm: config must not be nil")
	}

	switch cfg.LLMProvider {
	case config.ProviderNone:
		return Noop{}, nil
	case config.ProviderGemini:
		return NewGeminiProvider(cfg)
	case config.ProviderOllama:
		return NewOllamaProvider(cfg)
	default:
		return nil, fmt.Errorf("llm: unknown provider %q: %w", cfg.LLMProvider, ErrNotImplemented)
	}
}
