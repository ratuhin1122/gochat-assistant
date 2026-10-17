package nlu_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ratuhin1122/gochat-assistant/internal/nlu"
)

func TestNormalizeString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "Hello, World!", want: "hello world"},
		{input: "HOW ARE YOU???", want: "how are you"},
		{input: "  Multiple   spaces and punctuation...!! ", want: "multiple spaces and punctuation"},
		{input: "what's the weather?", want: "what s the weather"},
	}

	for _, tc := range tests {
		got := nlu.NormalizeString(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeString(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		s1   string
		s2   string
		want int
	}{
		{s1: "", s2: "", want: 0},
		{s1: "a", s2: "", want: 1},
		{s1: "", s2: "abc", want: 3},
		{s1: "hello", s2: "hello", want: 0},
		{s1: "helo", s2: "hello", want: 1},
		{s1: "kitten", s2: "sitting", want: 3},
	}

	for _, tc := range tests {
		got := nlu.Levenshtein(tc.s1, tc.s2)
		if got != tc.want {
			t.Errorf("Levenshtein(%q, %q) = %d, want %d", tc.s1, tc.s2, got, tc.want)
		}
	}
}

func TestFAQResponder_Respond(t *testing.T) {
	responder, err := nlu.NewFAQResponder()
	if err != nil {
		t.Fatalf("failed to create FAQResponder: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		wantHandled bool
		wantSubstr  string
	}{
		{
			name:        "exact match greeting",
			input:       "hello",
			wantHandled: true,
			wantSubstr:  "Hello! How can I assist you today?",
		},
		{
			name:        "typo in greeting - helo",
			input:       "helo",
			wantHandled: true,
			wantSubstr:  "Hello! How can I assist you today?",
		},
		{
			name:        "typo in greeting - hye there",
			input:       "hye there",
			wantHandled: true,
			wantSubstr:  "Hello! How can I assist you today?",
		},
		{
			name:        "punctuation and case in greeting",
			input:       "HELLO!!",
			wantHandled: true,
			wantSubstr:  "Hello! How can I assist you today?",
		},
		{
			name:        "how are you variation with question mark",
			input:       "How are you doing?",
			wantHandled: true,
			wantSubstr:  "I'm doing well, thank you!",
		},
		{
			name:        "who built you with typo",
			input:       "who bult you?",
			wantHandled: true,
			wantSubstr:  "GoChat team",
		},
		{
			name:        "joke request",
			input:       "tell me a joke!",
			wantHandled: true,
			wantSubstr:  "Because they don't C#!",
		},
		{
			name:        "goodbye greeting",
			input:       "see you later",
			wantHandled: true,
			wantSubstr:  "Goodbye!",
		},
		{
			name:        "unrelated text - quantum physics",
			input:       "explain quantum physics entanglement",
			wantHandled: false,
		},
		{
			name:        "unrelated text - stock market",
			input:       "buy shares in apple and microsoft stock",
			wantHandled: false,
		},
		{
			name:        "unrelated text - random characters",
			input:       "asdfghjk qwertyuiop zxcvbnm",
			wantHandled: false,
		},
		{
			name:        "unrelated text - cooking recipe",
			input:       "how to bake chocolate chip cookies",
			wantHandled: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, handled, err := responder.Respond(context.Background(), nlu.Request{Text: tc.input})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if handled != tc.wantHandled {
				t.Fatalf("Respond(%q) handled = %v, want %v", tc.input, handled, tc.wantHandled)
			}

			if tc.wantHandled && tc.wantSubstr != "" {
				if resp.Text != tc.wantSubstr && !strings.Contains(resp.Text, tc.wantSubstr) {
					t.Errorf("Respond(%q) response = %q, want substring %q", tc.input, resp.Text, tc.wantSubstr)
				}
			}
		})
	}
}

func TestDefaultChain_WithFAQFirst(t *testing.T) {
	chain, err := nlu.NewDefaultChain()
	if err != nil {
		t.Fatalf("failed to create default chain: %v", err)
	}

	// 1. FAQ handled query: "helo" typo matches greeting
	resp, err := chain.Respond(context.Background(), nlu.Request{Text: "helo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Text, "Hello!") {
		t.Errorf("expected greeting response, got %q", resp.Text)
	}

	// 2. Unrelated text falls back to default message
	resp, err = chain.Respond(context.Background(), nlu.Request{Text: "completely unrelated rocket science query"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != nlu.DefaultFallback {
		t.Errorf("expected fallback %q, got %q", nlu.DefaultFallback, resp.Text)
	}
}

func TestFAQResponder_ContextCanceled(t *testing.T) {
	responder, err := nlu.NewFAQResponder()
	if err != nil {
		t.Fatalf("failed to create responder: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err = responder.Respond(ctx, nlu.Request{Text: "hello"})
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
}
