package chat_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/ratuhin1122/gochat-assistant/internal/chat"
	"github.com/ratuhin1122/gochat-assistant/internal/server"
)

func TestIntegration_Chat(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	router := server.New(logger, []string{"*"})
	srv := httptest.NewServer(router)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	dialWS := func(t *testing.T, timeout time.Duration) (*websocket.Conn, context.Context, context.CancelFunc) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			cancel()
			t.Fatalf("failed to dial websocket on %s: %v", wsURL, err)
		}
		return conn, ctx, cancel
	}

	sendJSON := func(t *testing.T, ctx context.Context, conn *websocket.Conn, v any) {
		t.Helper()
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("failed to marshal JSON: %v", err)
		}
		if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
			t.Fatalf("failed to write websocket message: %v", err)
		}
	}

	readJSON := func(t *testing.T, ctx context.Context, conn *websocket.Conn) chat.OutgoingMessage {
		t.Helper()
		typ, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("failed to read websocket message: %v", err)
		}
		if typ != websocket.MessageText {
			t.Fatalf("expected message type text, got %v", typ)
		}
		var msg chat.OutgoingMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("failed to unmarshal JSON response (%s): %v", string(data), err)
		}
		return msg
	}

	t.Run("hello gets FAQ reply", func(t *testing.T) {
		conn, ctx, cancel := dialWS(t, 5*time.Second)
		defer cancel()
		defer conn.CloseNow()

		sendJSON(t, ctx, conn, chat.IncomingMessage{
			Type: "text",
			Text: "hello",
		})

		resp := readJSON(t, ctx, conn)
		if resp.Type != "text" {
			t.Errorf("expected response type 'text', got %q", resp.Type)
		}
		if !strings.Contains(strings.ToLower(resp.Text), "assist") && !strings.Contains(strings.ToLower(resp.Text), "hello") {
			t.Errorf("expected FAQ greeting reply, got %q", resp.Text)
		}
	})

	t.Run("600-character message gets error message", func(t *testing.T) {
		conn, ctx, cancel := dialWS(t, 5*time.Second)
		defer cancel()
		defer conn.CloseNow()

		sendJSON(t, ctx, conn, chat.IncomingMessage{
			Type: "text",
			Text: strings.Repeat("a", 600),
		})

		resp := readJSON(t, ctx, conn)
		if resp.Type != "error" {
			t.Errorf("expected response type 'error', got %q", resp.Type)
		}
		if len(resp.Text) == 0 {
			t.Error("expected non-empty error message text")
		}
	})

	t.Run("sending too fast triggers rate limit", func(t *testing.T) {
		conn, ctx, cancel := dialWS(t, 10*time.Second)
		defer cancel()
		defer conn.CloseNow()

		// Burst 15 messages rapidly to exhaust connection rate limit (burst=10)
		const burstCount = 15
		for i := 0; i < burstCount; i++ {
			sendJSON(t, ctx, conn, chat.IncomingMessage{
				Type: "text",
				Text: "ping",
			})
		}

		// Read responses until rate limit exceeded error is detected
		var rateLimitTriggered bool
		for i := 0; i < burstCount; i++ {
			resp := readJSON(t, ctx, conn)
			if resp.Type == "error" && strings.Contains(strings.ToLower(resp.Text), "rate limit") {
				rateLimitTriggered = true
				break
			}
		}

		if !rateLimitTriggered {
			t.Fatal("expected rate limit error after sending messages too fast, but none was received")
		}
	})
}
