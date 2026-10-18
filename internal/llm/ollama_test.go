package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ratuhin1122/gochat-assistant/internal/config"
)

func TestOllamaProvider_Success(t *testing.T) {
	var capturedReq ollamaChatRequest

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected /api/chat, got %s", r.URL.Path)
			http.Error(w, "invalid path", http.StatusNotFound)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}

		if err := json.NewDecoder(r.Body).Decode(&capturedReq); err != nil {
			t.Errorf("failed to decode request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := ollamaChatResponse{
			Model: capturedReq.Model,
			Message: ollamaMessage{
				Role:    "assistant",
				Content: "Hello from Ollama!",
			},
			Done: true,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	cfg := &config.Config{
		LLMProvider: config.ProviderOllama,
		OllamaURL:   ts.URL,
		OllamaModel: "mistral",
	}

	p, err := NewOllamaProvider(cfg, WithOllamaHTTPClient(ts.Client()))
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	if p.Name() != "ollama" {
		t.Errorf("expected Name() to be 'ollama', got %q", p.Name())
	}

	messages := []Message{
		{Role: RoleSystem, Content: "You are an assistant."},
		{Role: RoleUser, Content: "Hi there!"},
	}

	reply, err := p.Generate(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected Generate error: %v", err)
	}

	if reply != "Hello from Ollama!" {
		t.Errorf("expected 'Hello from Ollama!', got %q", reply)
	}

	if capturedReq.Model != "mistral" {
		t.Errorf("expected model 'mistral', got %q", capturedReq.Model)
	}
	if capturedReq.Stream {
		t.Error("expected stream=false, got true")
	}
	if len(capturedReq.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(capturedReq.Messages))
	}
	if capturedReq.Messages[0].Role != "system" || capturedReq.Messages[0].Content != "You are an assistant." {
		t.Errorf("unexpected msg 0: %+v", capturedReq.Messages[0])
	}
	if capturedReq.Messages[1].Role != "user" || capturedReq.Messages[1].Content != "Hi there!" {
		t.Errorf("unexpected msg 1: %+v", capturedReq.Messages[1])
	}
}

func TestOllamaProvider_Non200Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "model 'llama3' not found"}`))
	}))
	defer ts.Close()

	cfg := &config.Config{
		LLMProvider: config.ProviderOllama,
		OllamaURL:   ts.URL,
		OllamaModel: "llama3",
	}

	p, err := NewOllamaProvider(cfg, WithOllamaHTTPClient(ts.Client()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestOllamaProvider_ServerErrorInJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaChatResponse{
			Error: "out of GPU memory",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	cfg := &config.Config{
		LLMProvider: config.ProviderOllama,
		OllamaURL:   ts.URL,
	}

	p, err := NewOllamaProvider(cfg, WithOllamaHTTPClient(ts.Client()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
	if err == nil {
		t.Fatal("expected error when json has error field, got nil")
	}
}

func TestOllamaProvider_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"late"}}`))
	}))
	defer ts.Close()

	cfg := &config.Config{
		LLMProvider: config.ProviderOllama,
		OllamaURL:   ts.URL,
	}

	p, err := NewOllamaProvider(cfg, WithOllamaHTTPClient(ts.Client()), WithOllamaTimeout(20*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestOllamaProvider_EmptyMessages(t *testing.T) {
	cfg := &config.Config{
		LLMProvider: config.ProviderOllama,
	}
	p, err := NewOllamaProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Generate(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil messages, got nil")
	}

	_, err = p.Generate(context.Background(), []Message{})
	if err == nil {
		t.Fatal("expected error for empty messages, got nil")
	}
}

func TestOllamaProvider_TrailingSlash(t *testing.T) {
	var requestedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		resp := ollamaChatResponse{
			Message: ollamaMessage{Role: "assistant", Content: "ok"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	cfg := &config.Config{
		LLMProvider: config.ProviderOllama,
		OllamaURL:   ts.URL + "////",
	}

	p, err := NewOllamaProvider(cfg, WithOllamaHTTPClient(ts.Client()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestedPath != "/api/chat" {
		t.Errorf("expected path /api/chat, got %q", requestedPath)
	}
}

func TestOllamaProvider_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer ts.Close()

	cfg := &config.Config{
		LLMProvider: config.ProviderOllama,
		OllamaURL:   ts.URL,
	}

	p, err := NewOllamaProvider(cfg, WithOllamaHTTPClient(ts.Client()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Generate(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}})
	if err == nil {
		t.Fatal("expected error for invalid json, got nil")
	}
}

func TestOllamaProvider_Validation(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := NewOllamaProvider(nil)
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("defaults applied", func(t *testing.T) {
		cfg := &config.Config{
			LLMProvider: config.ProviderOllama,
		}
		p, err := NewOllamaProvider(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.baseURL != DefaultOllamaURL {
			t.Errorf("expected baseURL %q, got %q", DefaultOllamaURL, p.baseURL)
		}
		if p.model != DefaultOllamaModel {
			t.Errorf("expected model %q, got %q", DefaultOllamaModel, p.model)
		}
		if p.timeout != DefaultOllamaTimeout {
			t.Errorf("expected timeout %v, got %v", DefaultOllamaTimeout, p.timeout)
		}
	})
}
