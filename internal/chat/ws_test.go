package chat_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/ratuhin1122/gochat-assistant/internal/chat"
)

func TestHandler_EchoAndOrigins(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		wantConnectErr bool
	}{
		{
			name:           "default localhost allowed",
			allowedOrigins: nil,
			requestOrigin:  "http://localhost:3000",
			wantConnectErr: false,
		},
		{
			name:           "unauthorized origin rejected",
			allowedOrigins: []string{"trusted.com"},
			requestOrigin:  "http://evil.com",
			wantConnectErr: true,
		},
		{
			name:           "configured origin allowed",
			allowedOrigins: []string{"trusted.com"},
			requestOrigin:  "http://trusted.com",
			wantConnectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := chat.NewHandler(logger, tc.allowedOrigins)
			srv := httptest.NewServer(handler)
			defer srv.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			opts := &websocket.DialOptions{
				HTTPHeader: http.Header{
					"Origin": []string{tc.requestOrigin},
				},
			}

			conn, _, err := websocket.Dial(ctx, srv.URL, opts)
			if tc.wantConnectErr {
				if err == nil {
					_ = conn.Close(websocket.StatusNormalClosure, "")
					t.Fatalf("expected dial to fail, but succeeded")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected dial error: %v", err)
			}
			defer conn.CloseNow()

			// Send valid JSON message
			reqMsg := chat.IncomingMessage{
				Type: "text",
				Text: "hello gochat",
			}
			reqBytes, _ := json.Marshal(reqMsg)
			if err := conn.Write(ctx, websocket.MessageText, reqBytes); err != nil {
				t.Fatalf("write failed: %v", err)
			}

			typ, data, err := conn.Read(ctx)
			if err != nil {
				t.Fatalf("read failed: %v", err)
			}

			if typ != websocket.MessageText {
				t.Errorf("expected MessageText, got %v", typ)
			}

			var resp chat.OutgoingMessage
			if err := json.Unmarshal(data, &resp); err != nil {
				t.Fatalf("failed to parse response JSON: %v, raw: %s", err, string(data))
			}

			if resp.Type != "text" {
				t.Errorf("expected response type 'text', got %q", resp.Type)
			}
			if resp.Text != "hello gochat" {
				t.Errorf("expected echo 'hello gochat', got %q", resp.Text)
			}
			if resp.ID == "" {
				t.Errorf("expected non-empty message ID")
			}
			if resp.Timestamp == "" {
				t.Errorf("expected non-empty timestamp")
			}

			// Clean close
			if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
				t.Errorf("expected clean close, got: %v", err)
			}
		})
	}
}

func TestHandler_InvalidInputDoesNotDisconnect(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := chat.NewHandler(logger, nil)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, srv.URL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.CloseNow()

	// 1. Send invalid non-JSON data -> expects error response, NOT disconnect
	if err := conn.Write(ctx, websocket.MessageText, []byte("not a json string")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("connection was closed on invalid JSON: %v", err)
	}

	var errResp chat.OutgoingMessage
	if err := json.Unmarshal(data, &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if errResp.Type != "error" {
		t.Errorf("expected Type 'error', got %q", errResp.Type)
	}

	// 2. Send validation failure (control character) -> expects error response, NOT disconnect
	ctrlMsg := chat.IncomingMessage{
		Type: "text",
		Text: "bad\x00control",
	}
	ctrlBytes, _ := json.Marshal(ctrlMsg)
	if err := conn.Write(ctx, websocket.MessageText, ctrlBytes); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	_, data, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("connection was closed on validation failure: %v", err)
	}
	if err := json.Unmarshal(data, &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if errResp.Type != "error" {
		t.Errorf("expected Type 'error', got %q", errResp.Type)
	}

	// 3. Send valid message on the SAME connection to prove it stayed open
	validMsg := chat.IncomingMessage{
		Type: "text",
		Text: "valid after error",
	}
	validBytes, _ := json.Marshal(validMsg)
	if err := conn.Write(ctx, websocket.MessageText, validBytes); err != nil {
		t.Fatalf("write failed on alive connection: %v", err)
	}

	_, data, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed on alive connection: %v", err)
	}

	var validResp chat.OutgoingMessage
	if err := json.Unmarshal(data, &validResp); err != nil {
		t.Fatalf("failed to unmarshal valid response: %v", err)
	}
	if validResp.Type != "text" || validResp.Text != "valid after error" {
		t.Errorf("unexpected response: %+v", validResp)
	}

	_ = conn.Close(websocket.StatusNormalClosure, "done")
}

func TestHandler_ReadLimit(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := chat.NewHandler(logger, nil)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, srv.URL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.CloseNow()

	// Send message exceeding 4096 bytes
	largeMsg := strings.Repeat("x", 4097)
	if err := conn.Write(ctx, websocket.MessageText, []byte(largeMsg)); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Reading should fail due to StatusMessageTooBig (1009)
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Fatalf("expected error reading oversized message, got nil")
	}

	cs := websocket.CloseStatus(err)
	if cs != websocket.StatusMessageTooBig {
		t.Errorf("expected StatusMessageTooBig (%d), got %d (%v)", websocket.StatusMessageTooBig, cs, err)
	}
}

func TestHandler_MessageRateLimit(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := chat.NewHandler(logger, nil)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, srv.URL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.CloseNow()

	// Burst 10 messages should all succeed
	for i := 0; i < 10; i++ {
		msg := chat.IncomingMessage{Type: "text", Text: "burst message"}
		b, _ := json.Marshal(msg)
		if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}

		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read %d failed: %v", i, err)
		}
		var resp chat.OutgoingMessage
		_ = json.Unmarshal(data, &resp)
		if resp.Type != "text" {
			t.Fatalf("expected text message on burst %d, got %q", i, resp.Type)
		}
	}

	// 11th message in quick succession should be rate limited and return error type message
	msg := chat.IncomingMessage{Type: "text", Text: "one too many"}
	b, _ := json.Marshal(msg)
	if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatalf("write 11th failed: %v", err)
	}

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read 11th failed (connection was closed?): %v", err)
	}

	var rateLimitResp chat.OutgoingMessage
	if err := json.Unmarshal(data, &rateLimitResp); err != nil {
		t.Fatalf("failed to unmarshal rate limit response: %v", err)
	}
	if rateLimitResp.Type != "error" {
		t.Errorf("expected type 'error' when rate limit exceeded, got %q", rateLimitResp.Type)
	}
	if !strings.Contains(rateLimitResp.Text, "rate limit") {
		t.Errorf("expected rate limit text, got %q", rateLimitResp.Text)
	}

	_ = conn.Close(websocket.StatusNormalClosure, "done")
}

func TestHandler_UpgradeRateLimit(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := chat.NewHandler(logger, nil)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testIP := "203.0.113.88"
	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"X-Forwarded-For": []string{testIP},
		},
	}

	// Burst 10 upgrades from the same IP should succeed
	var conns []*websocket.Conn
	for i := 0; i < 10; i++ {
		conn, _, err := websocket.Dial(ctx, srv.URL, opts)
		if err != nil {
			t.Fatalf("upgrade %d failed: %v", i, err)
		}
		conns = append(conns, conn)
	}
	defer func() {
		for _, c := range conns {
			_ = c.CloseNow()
		}
	}()

	// 11th upgrade from the same IP should fail with HTTP 429 Too Many Requests
	_, resp, err := websocket.Dial(ctx, srv.URL, opts)
	if err == nil {
		t.Fatalf("expected 11th upgrade to fail with 429, but succeeded")
	}
	if resp == nil {
		t.Fatalf("expected non-nil HTTP response for failed upgrade")
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected status 429 Too Many Requests, got %d", resp.StatusCode)
	}
}

