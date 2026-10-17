package nlu_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ratuhin1122/gochat-assistant/internal/nlu"
)

type fakeResponder struct {
	handled  bool
	response nlu.Response
	err      error
	called   bool
	received nlu.Request
}

func (f *fakeResponder) Respond(ctx context.Context, req nlu.Request) (nlu.Response, bool, error) {
	f.called = true
	f.received = req
	return f.response, f.handled, f.err
}

func TestChain_Respond(t *testing.T) {
	errSentinel := errors.New("boom")

	tests := []struct {
		name          string
		responders    []*fakeResponder
		fallback      string
		req           nlu.Request
		cancelCtx     bool
		wantText      string
		wantErr       bool
		expectedCalls []bool
	}{
		{
			name:       "empty chain returns default fallback",
			responders: nil,
			req:        nlu.Request{Text: "hello"},
			wantText:   nlu.DefaultFallback,
		},
		{
			name: "single responder handles",
			responders: []*fakeResponder{
				{handled: true, response: nlu.Response{Text: "handled by first"}},
			},
			req:           nlu.Request{Text: "hello"},
			wantText:      "handled by first",
			expectedCalls: []bool{true},
		},
		{
			name: "single responder does not handle returns default fallback",
			responders: []*fakeResponder{
				{handled: false, response: nlu.Response{Text: "ignored"}},
			},
			req:           nlu.Request{Text: "hello"},
			wantText:      nlu.DefaultFallback,
			expectedCalls: []bool{true},
		},
		{
			name: "first responder handles, stops evaluation (short-circuit)",
			responders: []*fakeResponder{
				{handled: true, response: nlu.Response{Text: "first responder"}},
				{handled: true, response: nlu.Response{Text: "second responder"}},
			},
			req:           nlu.Request{Text: "ping"},
			wantText:      "first responder",
			expectedCalls: []bool{true, false},
		},
		{
			name: "first skips, second handles",
			responders: []*fakeResponder{
				{handled: false},
				{handled: true, response: nlu.Response{Text: "second responder"}},
				{handled: true, response: nlu.Response{Text: "third responder"}},
			},
			req:           nlu.Request{Text: "ping"},
			wantText:      "second responder",
			expectedCalls: []bool{true, true, false},
		},
		{
			name: "all skip returns default fallback",
			responders: []*fakeResponder{
				{handled: false},
				{handled: false},
			},
			req:           nlu.Request{Text: "unknown"},
			wantText:      nlu.DefaultFallback,
			expectedCalls: []bool{true, true},
		},
		{
			name: "custom fallback message when all skip",
			responders: []*fakeResponder{
				{handled: false},
			},
			fallback:      "Custom fallback response",
			req:           nlu.Request{Text: "unknown"},
			wantText:      "Custom fallback response",
			expectedCalls: []bool{true},
		},
		{
			name: "responder error stops chain and propagates error",
			responders: []*fakeResponder{
				{handled: false, err: errSentinel},
				{handled: true, response: nlu.Response{Text: "should not run"}},
			},
			req:           nlu.Request{Text: "err test"},
			wantErr:       true,
			expectedCalls: []bool{true, false},
		},
		{
			name: "canceled context returns error without calling responders",
			responders: []*fakeResponder{
				{handled: true, response: nlu.Response{Text: "should not run"}},
			},
			req:           nlu.Request{Text: "canceled"},
			cancelCtx:     true,
			wantErr:       true,
			expectedCalls: []bool{false},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelCtx {
				cancel()
			}

			var ifaceResponders []nlu.Responder
			for _, r := range tc.responders {
				ifaceResponders = append(ifaceResponders, r)
			}

			var chain *nlu.Chain
			if tc.fallback != "" {
				chain = nlu.NewChainWithFallback(tc.fallback, ifaceResponders...)
			} else {
				chain = nlu.NewChain(ifaceResponders...)
			}

			resp, err := chain.Respond(ctx, tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.cancelCtx && !errors.Is(err, context.Canceled) {
					t.Errorf("expected context.Canceled, got %v", err)
				}
				if !tc.cancelCtx && !errors.Is(err, errSentinel) {
					t.Errorf("expected wrapped errSentinel, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if resp.Text != tc.wantText {
					t.Errorf("expected text %q, got %q", tc.wantText, resp.Text)
				}
			}

			for i, r := range tc.responders {
				if i < len(tc.expectedCalls) {
					if r.called != tc.expectedCalls[i] {
						t.Errorf("responder %d called = %v, want %v", i, r.called, tc.expectedCalls[i])
					}
					if r.called && r.received.Text != tc.req.Text {
						t.Errorf("responder %d received text %q, want %q", i, r.received.Text, tc.req.Text)
					}
				}
			}
		})
	}
}

func TestChain_NilSafe(t *testing.T) {
	var chain *nlu.Chain
	resp, err := chain.Respond(context.Background(), nlu.Request{Text: "test"})
	if err != nil {
		t.Fatalf("unexpected error on nil chain: %v", err)
	}
	if resp.Text != nlu.DefaultFallback {
		t.Errorf("expected fallback on nil chain, got %q", resp.Text)
	}
}
