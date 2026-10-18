// Package llm provides LLM provider integrations.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ratuhin1122/gochat-assistant/internal/config"
)

const (
	// DefaultOllamaTimeout is the default timeout for Ollama generation requests.
	DefaultOllamaTimeout = 60 * time.Second
	// DefaultOllamaURL is the default base URL for local Ollama server.
	DefaultOllamaURL = "http://localhost:11434"
	// DefaultOllamaModel is the default model name if none is specified.
	DefaultOllamaModel = "llama3"
)

// OllamaProvider implements Provider by sending requests to Ollama's HTTP API.
type OllamaProvider struct {
	baseURL    string
	model      string
	timeout    time.Duration
	httpClient *http.Client
}

// OllamaOption allows customizing OllamaProvider configuration.
type OllamaOption func(*OllamaProvider)

// WithOllamaHTTPClient overrides the default HTTP client (useful for httptest).
func WithOllamaHTTPClient(client *http.Client) OllamaOption {
	return func(p *OllamaProvider) {
		if client != nil {
			p.httpClient = client
		}
	}
}

// WithOllamaTimeout sets a custom request timeout.
func WithOllamaTimeout(d time.Duration) OllamaOption {
	return func(p *OllamaProvider) {
		if d > 0 {
			p.timeout = d
		}
	}
}

// NewOllamaProvider constructs a new OllamaProvider using application config.
func NewOllamaProvider(cfg *config.Config, opts ...OllamaOption) (*OllamaProvider, error) {
	if cfg == nil {
		return nil, errors.New("ollama: config must not be nil")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.OllamaURL), "/")
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}

	model := strings.TrimSpace(cfg.OllamaModel)
	if model == "" {
		model = DefaultOllamaModel
	}

	p := &OllamaProvider{
		baseURL:    baseURL,
		model:      model,
		timeout:    DefaultOllamaTimeout,
		httpClient: &http.Client{},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

// Name implements Provider.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// ollamaChatRequest models the payload expected by Ollama /api/chat.
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaChatResponse models the JSON returned by Ollama /api/chat when stream=false.
type ollamaChatResponse struct {
	Model      string        `json:"model"`
	CreatedAt  string        `json:"created_at"`
	Message    ollamaMessage `json:"message"`
	Done       bool          `json:"done"`
	Error      string        `json:"error,omitempty"`
}

// Generate sends messages to Ollama /api/chat with stream=false and a 60s timeout.
func (p *OllamaProvider) Generate(ctx context.Context, messages []Message) (string, error) {
	if len(messages) == 0 {
		return "", errors.New("ollama: empty messages")
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	ollamaMsgs := make([]ollamaMessage, 0, len(messages))
	for _, m := range messages {
		role := string(m.Role)
		if role == "" {
			role = string(RoleUser)
		}
		ollamaMsgs = append(ollamaMsgs, ollamaMessage{
			Role:    role,
			Content: m.Content,
		})
	}

	reqPayload := ollamaChatRequest{
		Model:    p.model,
		Messages: ollamaMsgs,
		Stream:   false,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("ollama: marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/chat", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("ollama: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("ollama: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("ollama: decode response: %w", err)
	}

	if chatResp.Error != "" {
		return "", fmt.Errorf("ollama: server error: %s", chatResp.Error)
	}

	return chatResp.Message.Content, nil
}
