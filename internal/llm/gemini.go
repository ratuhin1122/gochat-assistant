// Package llm provides LLM provider integrations.
package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ratuhin1122/gochat-assistant/internal/config"
	"google.golang.org/genai"
)

const (
	// DefaultTimeout is the request timeout applied to Gemini calls.
	DefaultTimeout = 15 * time.Second
	// DefaultInitialBackoff is the starting delay before retrying.
	DefaultInitialBackoff = 500 * time.Millisecond
	// MaxTries is the maximum number of attempts (initial + retries).
	MaxTries = 3
)

// Client abstracts the Google Gen AI SDK model generation call.
// This allows tests to inject mock implementations without network access.
type Client interface {
	GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
}

// GeminiProvider implements Provider using the official Google Gen AI Go SDK.
type GeminiProvider struct {
	client         Client
	model          string
	timeout        time.Duration
	initialBackoff time.Duration
	maxTries       int
}

// Option configures a GeminiProvider.
type Option func(*GeminiProvider)

// WithClient overrides the client implementation (useful for testing with mocks).
func WithClient(c Client) Option {
	return func(p *GeminiProvider) {
		p.client = c
	}
}

// WithTimeout sets a custom request timeout.
func WithTimeout(d time.Duration) Option {
	return func(p *GeminiProvider) {
		if d > 0 {
			p.timeout = d
		}
	}
}

// WithInitialBackoff sets the initial backoff delay before retries.
func WithInitialBackoff(d time.Duration) Option {
	return func(p *GeminiProvider) {
		p.initialBackoff = d
	}
}

// WithMaxTries sets the maximum number of request attempts.
func WithMaxTries(n int) Option {
	return func(p *GeminiProvider) {
		if n > 0 {
			p.maxTries = n
		}
	}
}

// NewGeminiProvider constructs a GeminiProvider from application config.
func NewGeminiProvider(cfg *config.Config, opts ...Option) (*GeminiProvider, error) {
	if cfg == nil {
		return nil, errors.New("gemini: config must not be nil")
	}

	model := cfg.GeminiModel
	if model == "" {
		model = "gemini-1.5-flash"
	}

	p := &GeminiProvider{
		model:          model,
		timeout:        DefaultTimeout,
		initialBackoff: DefaultInitialBackoff,
		maxTries:       MaxTries,
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.client == nil {
		if cfg.GeminiAPIKey == "" {
			return nil, errors.New("gemini: GEMINI_API_KEY is required")
		}
		sdkClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{
			APIKey:  cfg.GeminiAPIKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return nil, fmt.Errorf("gemini: create client: %w", err)
		}
		p.client = sdkClient.Models
	}

	return p, nil
}

// Name implements Provider.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// Generate formats conversation messages, calls Gemini with a 15s timeout,
// and retries with exponential backoff on 429 and 5xx errors (up to 3 tries).
func (p *GeminiProvider) Generate(ctx context.Context, messages []Message) (string, error) {
	if len(messages) == 0 {
		return "", errors.New("gemini: empty messages")
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	contents, genConfig := p.prepareRequest(messages)

	backoff := p.initialBackoff
	var lastErr error

	for attempt := 1; attempt <= p.maxTries; attempt++ {
		resp, err := p.client.GenerateContent(ctx, p.model, contents, genConfig)
		if err == nil {
			if resp == nil {
				return "", errors.New("gemini: empty response from model")
			}
			return resp.Text(), nil
		}

		lastErr = err

		// If context was canceled or timed out, stop immediately.
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		// Only retry if error is retryable (429 or 5xx) and attempts remain.
		if attempt >= p.maxTries || !isRetryable(err) {
			break
		}

		if backoff > 0 {
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}

		backoff *= 2
	}

	return "", fmt.Errorf("gemini: generate content failed after attempts: %w", lastErr)
}

// prepareRequest converts internal Message slices into GenAI Content structures.
func (p *GeminiProvider) prepareRequest(messages []Message) ([]*genai.Content, *genai.GenerateContentConfig) {
	var contents []*genai.Content
	var systemParts []*genai.Part

	for _, msg := range messages {
		switch msg.Role {
		case RoleSystem:
			systemParts = append(systemParts, &genai.Part{Text: msg.Content})
		case RoleAssistant:
			contents = append(contents, &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: msg.Content}},
			})
		case RoleUser:
			fallthrough
		default:
			contents = append(contents, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: msg.Content}},
			})
		}
	}

	var genConfig *genai.GenerateContentConfig
	if len(systemParts) > 0 {
		genConfig = &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: systemParts,
			},
		}
	}

	// If only system messages were passed, treat them as user prompt content.
	if len(contents) == 0 && len(systemParts) > 0 {
		contents = append(contents, &genai.Content{
			Role:  "user",
			Parts: systemParts,
		})
		genConfig = nil
	}

	return contents, genConfig
}

// isRetryable returns true for 429 Too Many Requests and 5xx server errors.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var apiErr *genai.APIError
	if errors.As(err, &apiErr) {
		return isRetryableCode(apiErr.Code)
	}

	var apiErrVal genai.APIError
	if errors.As(err, &apiErrVal) {
		return isRetryableCode(apiErrVal.Code)
	}

	type statusCoder interface {
		StatusCode() int
	}
	var sc statusCoder
	if errors.As(err, &sc) {
		return isRetryableCode(sc.StatusCode())
	}

	type httpStatusCoder interface {
		HTTPStatus() int
	}
	var hsc httpStatusCoder
	if errors.As(err, &hsc) {
		return isRetryableCode(hsc.HTTPStatus())
	}

	// Fallback to error message string inspection for wrapped or standard errors.
	msg := err.Error()
	if strings.Contains(msg, "429") || strings.Contains(msg, "RESOURCE_EXHAUSTED") {
		return true
	}
	for _, code := range []string{"500", "502", "503", "504"} {
		if strings.Contains(msg, code) {
			return true
		}
	}

	return false
}

func isRetryableCode(code int) bool {
	return code == 429 || (code >= 500 && code <= 599)
}
