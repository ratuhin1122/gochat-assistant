// Package chat manages real-time WebSocket communication.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const (
	maxMessageSize = 4096
	pingInterval   = 30 * time.Second
	pingTimeout    = 10 * time.Second
)

// Handler handles WebSocket connections for the chatbot.
type Handler struct {
	logger         *slog.Logger
	allowedOrigins []string
	validator      *MessageValidator
}

// NewHandler constructs a chat Handler with verified origins.
// If allowedOrigins is empty, it defaults to localhost origins.
func NewHandler(logger *slog.Logger, allowedOrigins []string) *Handler {
	return &Handler{
		logger:         logger,
		allowedOrigins: resolveOrigins(allowedOrigins),
		validator:      NewMessageValidator(),
	}
}

// ServeHTTP upgrades the HTTP connection to WebSocket, enforces the 4096-byte
// read limit, echoes text messages back, and closes cleanly upon cancellation or disconnect.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: h.allowedOrigins,
	})
	if err != nil {
		h.logger.WarnContext(ctx, "websocket accept failed", "err", err)
		return
	}
	defer conn.CloseNow()

	// Enforce maximum incoming message size
	conn.SetReadLimit(maxMessageSize)

	// Active heartbeat: ping periodically to detect dead connections
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, pingTimeout)
				err := conn.Ping(pingCtx)
				pingCancel()
				if err != nil {
					conn.CloseNow()
					cancel()
					return
				}
			}
		}
	}()

	// Message read loop
	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				cs := websocket.CloseStatus(err)
				if cs != websocket.StatusNormalClosure && cs != websocket.StatusGoingAway {
					h.logger.DebugContext(ctx, "websocket read ended", "err", err)
				}
			}
			break
		}

		if msgType != websocket.MessageText {
			continue
		}

		// Parse incoming JSON
		var incoming IncomingMessage
		if err := json.Unmarshal(data, &incoming); err != nil {
			errResp, _ := json.Marshal(NewErrorMessage("invalid JSON message format"))
			_ = conn.Write(ctx, websocket.MessageText, errResp)
			continue
		}

		// Validate incoming message
		if err := h.validator.Validate(&incoming); err != nil {
			errResp, _ := json.Marshal(NewErrorMessage(err.Error()))
			_ = conn.Write(ctx, websocket.MessageText, errResp)
			continue
		}

		// Valid message: echo back as outgoing JSON message
		outgoing := NewOutgoingMessage(incoming.Type, incoming.Text)
		outBytes, err := json.Marshal(outgoing)
		if err != nil {
			h.logger.ErrorContext(ctx, "failed to marshal response", "err", err)
			continue
		}

		if err := conn.Write(ctx, websocket.MessageText, outBytes); err != nil {
			if !errors.Is(err, context.Canceled) {
				cs := websocket.CloseStatus(err)
				if cs != websocket.StatusNormalClosure && cs != websocket.StatusGoingAway {
					h.logger.DebugContext(ctx, "websocket write failed", "err", err)
				}
			}
			break
		}
	}

	_ = conn.Close(websocket.StatusNormalClosure, "")
}

// resolveOrigins normalizes and applies localhost defaults if no origins are supplied.
func resolveOrigins(origins []string) []string {
	if len(origins) == 0 {
		return []string{"localhost:*", "localhost", "127.0.0.1:*", "127.0.0.1"}
	}
	var res []string
	for _, o := range origins {
		res = append(res, o)
		if o == "localhost" {
			res = append(res, "localhost:*")
		} else if o == "127.0.0.1" {
			res = append(res, "127.0.0.1:*")
		}
	}
	return res
}
