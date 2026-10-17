package chat_test

import (
	"context"
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

			// Test Echo
			msg := "hello gochat"
			if err := conn.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
				t.Fatalf("write failed: %v", err)
			}

			typ, data, err := conn.Read(ctx)
			if err != nil {
				t.Fatalf("read failed: %v", err)
			}

			if typ != websocket.MessageText {
				t.Errorf("expected MessageText, got %v", typ)
			}
			if string(data) != msg {
				t.Errorf("expected echo %q, got %q", msg, string(data))
			}

			// Clean close
			if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
				t.Errorf("expected clean close, got: %v", err)
			}
		})
	}
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
