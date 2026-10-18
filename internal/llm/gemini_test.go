package llm

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ratuhin1122/gochat-assistant/internal/config"
	"google.golang.org/genai"
)

// fakeGeminiClient implements Client for tests.
type fakeGeminiClient struct {
	generateFn func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
	calls      int32
	lastModel  string
}

func (f *fakeGeminiClient) GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	atomic.AddInt32(&f.calls, 1)
	f.lastModel = model
	if f.generateFn != nil {
		return f.generateFn(ctx, model, contents, config)
	}
	return mockResponse("default test response"), nil
}

func mockResponse(text string) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{Text: text},
					},
				},
			},
		},
	}
}

type customStatusError struct {
	code int
	msg  string
}

func (e *customStatusError) Error() string   { return e.msg }
func (e *customStatusError) StatusCode() int { return e.code }

func TestGeminiProvider_Success(t *testing.T) {
	var capturedContents []*genai.Content
	var capturedConfig *genai.GenerateContentConfig

	fake := &fakeGeminiClient{
		generateFn: func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			capturedContents = contents
			capturedConfig = cfg
			return mockResponse("Hello from Gemini!"), nil
		},
	}

	cfg := &config.Config{
		LLMProvider:  config.ProviderGemini,
		GeminiModel:  "gemini-1.5-flash",
		GeminiAPIKey: "fake-key",
	}

	p, err := NewGeminiProvider(cfg, WithClient(fake), WithInitialBackoff(time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	if p.Name() != "gemini" {
		t.Errorf("expected Name() to be 'gemini', got %q", p.Name())
	}

	messages := []Message{
		{Role: RoleSystem, Content: "You are a helpful assistant."},
		{Role: RoleUser, Content: "Hello!"},
		{Role: RoleAssistant, Content: "Hi there!"},
		{Role: RoleUser, Content: "Tell me a joke."},
	}

	reply, err := p.Generate(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected Generate error: %v", err)
	}

	if reply != "Hello from Gemini!" {
		t.Errorf("expected reply 'Hello from Gemini!', got %q", reply)
	}

	if fake.calls != 1 {
		t.Errorf("expected 1 call, got %d", fake.calls)
	}

	if fake.lastModel != "gemini-1.5-flash" {
		t.Errorf("expected model 'gemini-1.5-flash', got %q", fake.lastModel)
	}

	// Verify system instruction was separated into config
	if capturedConfig == nil || capturedConfig.SystemInstruction == nil {
		t.Fatal("expected SystemInstruction in config")
	}
	if len(capturedConfig.SystemInstruction.Parts) != 1 || capturedConfig.SystemInstruction.Parts[0].Text != "You are a helpful assistant." {
		t.Errorf("unexpected system instruction: %+v", capturedConfig.SystemInstruction)
	}

	// Verify user and assistant messages
	if len(capturedContents) != 3 {
		t.Fatalf("expected 3 non-system content entries, got %d", len(capturedContents))
	}
	if capturedContents[0].Role != "user" || capturedContents[0].Parts[0].Text != "Hello!" {
		t.Errorf("unexpected first content: %+v", capturedContents[0])
	}
	if capturedContents[1].Role != "model" || capturedContents[1].Parts[0].Text != "Hi there!" {
		t.Errorf("unexpected second content: %+v", capturedContents[1])
	}
	if capturedContents[2].Role != "user" || capturedContents[2].Parts[0].Text != "Tell me a joke." {
		t.Errorf("unexpected third content: %+v", capturedContents[2])
	}
}

func TestGeminiProvider_RetryOn429(t *testing.T) {
	var attempt int
	fake := &fakeGeminiClient{
		generateFn: func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			attempt++
			if attempt == 1 {
				return nil, &genai.APIError{Code: 429, Message: "Quota exceeded"}
			}
			return mockResponse("Success on retry"), nil
		},
	}

	cfg := &config.Config{
		LLMProvider:  config.ProviderGemini,
		GeminiModel:  "gemini-1.5-flash",
		GeminiAPIKey: "fake-key",
	}

	p, err := NewGeminiProvider(cfg, WithClient(fake), WithInitialBackoff(time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	reply, err := p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reply != "Success on retry" {
		t.Errorf("expected 'Success on retry', got %q", reply)
	}

	if fake.calls != 2 {
		t.Errorf("expected 2 calls, got %d", fake.calls)
	}
}

func TestGeminiProvider_RetryOn5xx(t *testing.T) {
	var attempt int
	fake := &fakeGeminiClient{
		generateFn: func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			attempt++
			if attempt == 1 {
				return nil, &genai.APIError{Code: 503, Message: "Service unavailable"}
			}
			if attempt == 2 {
				return nil, &genai.APIError{Code: 500, Message: "Internal server error"}
			}
			return mockResponse("Success on 3rd attempt"), nil
		},
	}

	cfg := &config.Config{
		LLMProvider:  config.ProviderGemini,
		GeminiModel:  "gemini-1.5-flash",
		GeminiAPIKey: "fake-key",
	}

	p, err := NewGeminiProvider(cfg, WithClient(fake), WithInitialBackoff(time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	reply, err := p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reply != "Success on 3rd attempt" {
		t.Errorf("expected 'Success on 3rd attempt', got %q", reply)
	}

	if fake.calls != 3 {
		t.Errorf("expected 3 calls, got %d", fake.calls)
	}
}

func TestGeminiProvider_MaxTriesExceeded(t *testing.T) {
	fake := &fakeGeminiClient{
		generateFn: func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			return nil, &genai.APIError{Code: 500, Message: "Persistent internal error"}
		},
	}

	cfg := &config.Config{
		LLMProvider:  config.ProviderGemini,
		GeminiModel:  "gemini-1.5-flash",
		GeminiAPIKey: "fake-key",
	}

	p, err := NewGeminiProvider(cfg, WithClient(fake), WithInitialBackoff(time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	_, err = p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
	if err == nil {
		t.Fatal("expected error after max tries exceeded, got nil")
	}

	if fake.calls != 3 {
		t.Errorf("expected exactly 3 attempts, got %d", fake.calls)
	}
}

func TestGeminiProvider_NoRetryOn4xx(t *testing.T) {
	fake := &fakeGeminiClient{
		generateFn: func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			return nil, &genai.APIError{Code: 400, Message: "Invalid argument"}
		},
	}

	cfg := &config.Config{
		LLMProvider:  config.ProviderGemini,
		GeminiModel:  "gemini-1.5-flash",
		GeminiAPIKey: "fake-key",
	}

	p, err := NewGeminiProvider(cfg, WithClient(fake), WithInitialBackoff(time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	_, err = p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if fake.calls != 1 {
		t.Errorf("expected exactly 1 call (no retries for 400), got %d", fake.calls)
	}
}

func TestGeminiProvider_CustomStatusCodeError(t *testing.T) {
	var attempt int
	fake := &fakeGeminiClient{
		generateFn: func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			attempt++
			if attempt == 1 {
				return nil, &customStatusError{code: 429, msg: "too many requests"}
			}
			return mockResponse("ok"), nil
		},
	}

	cfg := &config.Config{
		LLMProvider:  config.ProviderGemini,
		GeminiModel:  "gemini-1.5-flash",
		GeminiAPIKey: "fake-key",
	}

	p, err := NewGeminiProvider(cfg, WithClient(fake), WithInitialBackoff(time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	reply, err := p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "ok" {
		t.Errorf("expected 'ok', got %q", reply)
	}
	if fake.calls != 2 {
		t.Errorf("expected 2 calls, got %d", fake.calls)
	}
}

func TestGeminiProvider_Timeout(t *testing.T) {
	fake := &fakeGeminiClient{
		generateFn: func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	cfg := &config.Config{
		LLMProvider:  config.ProviderGemini,
		GeminiModel:  "gemini-1.5-flash",
		GeminiAPIKey: "fake-key",
	}

	// Configure with a short timeout to verify deadline cancellation
	p, err := NewGeminiProvider(cfg, WithClient(fake), WithTimeout(20*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	_, err = p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
	if err == nil {
		t.Fatal("expected deadline exceeded error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestGeminiProvider_EmptyMessages(t *testing.T) {
	fake := &fakeGeminiClient{}
	cfg := &config.Config{
		LLMProvider:  config.ProviderGemini,
		GeminiModel:  "gemini-1.5-flash",
		GeminiAPIKey: "fake-key",
	}

	p, err := NewGeminiProvider(cfg, WithClient(fake))
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	_, err = p.Generate(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil messages, got nil")
	}

	_, err = p.Generate(context.Background(), []Message{})
	if err == nil {
		t.Fatal("expected error for empty messages, got nil")
	}

	if fake.calls != 0 {
		t.Errorf("expected 0 calls for empty messages, got %d", fake.calls)
	}
}

func TestGeminiProvider_NilOrEmptyResponse(t *testing.T) {
	t.Run("nil response", func(t *testing.T) {
		fake := &fakeGeminiClient{
			generateFn: func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
				return nil, nil
			},
		}

		cfg := &config.Config{LLMProvider: config.ProviderGemini, GeminiAPIKey: "key"}
		p, _ := NewGeminiProvider(cfg, WithClient(fake))
		_, err := p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
		if err == nil {
			t.Fatal("expected error for nil response, got nil")
		}
	})

	t.Run("empty candidates", func(t *testing.T) {
		fake := &fakeGeminiClient{
			generateFn: func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
				return &genai.GenerateContentResponse{Candidates: nil}, nil
			},
		}

		cfg := &config.Config{LLMProvider: config.ProviderGemini, GeminiAPIKey: "key"}
		p, _ := NewGeminiProvider(cfg, WithClient(fake))
		reply, err := p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reply != "" {
			t.Errorf("expected empty string, got %q", reply)
		}
	})
}

func TestGeminiProvider_ModelFromConfig(t *testing.T) {
	fake := &fakeGeminiClient{}
	cfg := &config.Config{
		LLMProvider:  config.ProviderGemini,
		GeminiModel:  "gemini-2.0-flash-exp",
		GeminiAPIKey: "fake-key",
	}

	p, err := NewGeminiProvider(cfg, WithClient(fake))
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	_, err = p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Test"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.lastModel != "gemini-2.0-flash-exp" {
		t.Errorf("expected model 'gemini-2.0-flash-exp', got %q", fake.lastModel)
	}
}

func TestNewGeminiProvider_Validation(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := NewGeminiProvider(nil)
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("missing api key", func(t *testing.T) {
		cfg := &config.Config{
			LLMProvider:  config.ProviderGemini,
			GeminiAPIKey: "",
		}
		_, err := NewGeminiProvider(cfg)
		if err == nil {
			t.Fatal("expected error for missing api key")
		}
	})
}
