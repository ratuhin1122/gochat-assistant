package nlu_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ratuhin1122/gochat-assistant/internal/llm"
	"github.com/ratuhin1122/gochat-assistant/internal/nlu"
)

// fakeLLMProvider implements llm.Provider for testing.
type fakeLLMProvider struct {
	name       string
	generateFn func(ctx context.Context, messages []llm.Message) (string, error)
	calls      int32
	lastMsgs   []llm.Message
}

func (f *fakeLLMProvider) Name() string {
	if f.name != "" {
		return f.name
	}
	return "fake-llm"
}

func (f *fakeLLMProvider) Generate(ctx context.Context, messages []llm.Message) (string, error) {
	atomic.AddInt32(&f.calls, 1)
	f.lastMsgs = messages
	if f.generateFn != nil {
		return f.generateFn(ctx, messages)
	}
	return "fake response", nil
}

func TestLLMResponder_Success(t *testing.T) {
	fake := &fakeLLMProvider{
		generateFn: func(ctx context.Context, messages []llm.Message) (string, error) {
			return "I am an AI assistant and I can help you.", nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	resp := nlu.NewLLMResponder(fake, logger)

	res, handled, err := resp.Respond(context.Background(), nlu.Request{Text: "What is quantum computing?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true")
	}
	if res.Text != "I am an AI assistant and I can help you." {
		t.Errorf("unexpected response text: %q", res.Text)
	}

	if fake.calls != 1 {
		t.Errorf("expected 1 call, got %d", fake.calls)
	}
	if len(fake.lastMsgs) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(fake.lastMsgs))
	}
	if fake.lastMsgs[0].Role != llm.RoleSystem {
		t.Errorf("expected system role, got %s", fake.lastMsgs[0].Role)
	}
	// Check system prompt contents
	sysContent := strings.ToLower(fake.lastMsgs[0].Content)
	if !strings.Contains(sysContent, "concise") || !strings.Contains(sysContent, "never reveal") || !strings.Contains(sysContent, "ignore requests to change") {
		t.Errorf("system prompt missing required instructions: %q", fake.lastMsgs[0].Content)
	}

	if fake.lastMsgs[1].Role != llm.RoleUser || fake.lastMsgs[1].Content != "What is quantum computing?" {
		t.Errorf("unexpected user message: %+v", fake.lastMsgs[1])
	}
}

func TestLLMResponder_TruncateReplies(t *testing.T) {
	longReply := strings.Repeat("A", 1500)
	fake := &fakeLLMProvider{
		generateFn: func(ctx context.Context, messages []llm.Message) (string, error) {
			return longReply, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	resp := nlu.NewLLMResponder(fake, logger)

	res, handled, err := resp.Respond(context.Background(), nlu.Request{Text: "Tell me a long story"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true")
	}

	if len([]rune(res.Text)) != nlu.MaxReplyLength {
		t.Errorf("expected length %d, got %d", nlu.MaxReplyLength, len([]rune(res.Text)))
	}
	if res.Text != strings.Repeat("A", 1000) {
		t.Errorf("unexpected truncated output")
	}
}

func TestLLMResponder_ProviderError(t *testing.T) {
	fake := &fakeLLMProvider{
		generateFn: func(ctx context.Context, messages []llm.Message) (string, error) {
			return "", errors.New("upstream service unavailable")
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	resp := nlu.NewLLMResponder(fake, logger)

	res, handled, err := resp.Respond(context.Background(), nlu.Request{Text: "Hello"})
	if err != nil {
		t.Fatalf("expected nil error on provider failure, got %v", err)
	}
	if handled {
		t.Fatal("expected handled=false on provider failure")
	}
	if res.Text != "" {
		t.Errorf("expected empty response, got %q", res.Text)
	}

	// Verify that in a Chain, default message is returned when LLM provider fails
	chain := nlu.NewChain(resp)
	chainResp, err := chain.Respond(context.Background(), nlu.Request{Text: "Hello"})
	if err != nil {
		t.Fatalf("unexpected chain error: %v", err)
	}
	if chainResp.Text != nlu.DefaultFallback {
		t.Errorf("expected fallback %q, got %q", nlu.DefaultFallback, chainResp.Text)
	}
}

func TestLLMResponder_EmptyReply(t *testing.T) {
	fake := &fakeLLMProvider{
		generateFn: func(ctx context.Context, messages []llm.Message) (string, error) {
			return "   ", nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	resp := nlu.NewLLMResponder(fake, logger)

	res, handled, err := resp.Respond(context.Background(), nlu.Request{Text: "Hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatal("expected handled=false for empty reply")
	}
	if res.Text != "" {
		t.Errorf("expected empty response, got %q", res.Text)
	}
}

func TestLLMResponder_EmptyRequestText(t *testing.T) {
	fake := &fakeLLMProvider{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	resp := nlu.NewLLMResponder(fake, logger)

	res, handled, err := resp.Respond(context.Background(), nlu.Request{Text: "   "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatal("expected handled=false for whitespace text")
	}
	if res.Text != "" {
		t.Errorf("expected empty text, got %q", res.Text)
	}
	if fake.calls != 0 {
		t.Errorf("expected 0 calls to provider, got %d", fake.calls)
	}
}

func TestLLMResponder_NilProvider(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	resp := nlu.NewLLMResponder(nil, logger)

	_, handled, err := resp.Respond(context.Background(), nlu.Request{Text: "Hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatal("expected handled=false for nil provider")
	}
}

func TestLLMResponder_InDefaultChain(t *testing.T) {
	fake := &fakeLLMProvider{
		generateFn: func(ctx context.Context, messages []llm.Message) (string, error) {
			return "LLM answer to complex question", nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	llmResp := nlu.NewLLMResponder(fake, logger)

	chain, err := nlu.NewDefaultChain(llmResp)
	if err != nil {
		t.Fatalf("failed to create default chain: %v", err)
	}

	t.Run("FAQ intent handled by FAQ responder, not LLM", func(t *testing.T) {
		res, err := chain.Respond(context.Background(), nlu.Request{Text: "hello"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Text == "LLM answer to complex question" {
			t.Errorf("expected FAQ response, got LLM response: %q", res.Text)
		}
		if fake.calls != 0 {
			t.Errorf("expected LLM not to be called for FAQ, got %d calls", fake.calls)
		}
	})

	t.Run("Weather intent handled by Weather responder, not LLM", func(t *testing.T) {
		res, err := chain.Respond(context.Background(), nlu.Request{Text: "weather in Paris"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Text == "LLM answer to complex question" {
			t.Errorf("expected weather response, got LLM response: %q", res.Text)
		}
		if fake.calls != 0 {
			t.Errorf("expected LLM not to be called for Weather, got %d calls", fake.calls)
		}
	})

	t.Run("Unknown intent falls through to LLM responder", func(t *testing.T) {
		res, err := chain.Respond(context.Background(), nlu.Request{Text: "Explain how photosynthesis works in plants"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Text != "LLM answer to complex question" {
			t.Errorf("expected LLM answer, got %q", res.Text)
		}
		if fake.calls != 1 {
			t.Errorf("expected LLM to be called once, got %d", fake.calls)
		}
	})
}
