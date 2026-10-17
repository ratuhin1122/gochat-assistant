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
