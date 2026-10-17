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
	"github.com/ratuhin1122/gochat-assistant/internal/config"
	"github.com/ratuhin1122/gochat-assistant/internal/nlu"
	"golang.org/x/time/rate"
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
	ipLimiter      *IPRateLimiter
	chain          *nlu.Chain
}

// NewHandler constructs a chat Handler with verified origins, rate limiting, and an NLU chain.
// If allowedOrigins is empty, it defaults to localhost origins.
// If chain is omitted or nil, a default chain is created.
func NewHandler(logger *slog.Logger, allowedOrigins []string, chain ...*nlu.Chain) *Handler {
	var c *nlu.Chain
	if len(chain) > 0 && chain[0] != nil {
		c = chain[0]
	} else {
		var err error
		c, err = nlu.NewDefaultChain()
		if err != nil {
			logger.Error("failed to initialize default nlu chain", "err", err)
			c = nlu.NewChain()
		}
	}

	return &Handler{
		logger:         logger,
		allowedOrigins: resolveOrigins(allowedOrigins),
		validator:      NewMessageValidator(),
		ipLimiter:      NewIPRateLimiter(rate.Every(time.Minute/config.RateLimitIPUpgradesPerMin), config.RateLimitIPBurst),
		chain:          c,
	}
}

// ServeHTTP upgrades the HTTP connection to WebSocket, enforces the 4096-byte
// read limit, echoes text messages back, and closes cleanly upon cancellation or disconnect.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// Rate limit WebSocket upgrades per IP (10 per minute)
	ip := ClientIP(r)
	if !h.ipLimiter.Allow(ip) {
		h.logger.WarnContext(r.Context(), "websocket upgrade rate limit exceeded", "ip", ip)
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
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

	// Rate limit incoming messages per connection (5 per second, burst 10)
	msgLimiter := rate.NewLimiter(rate.Limit(config.RateLimitConnMessagesPerSec), config.RateLimitConnBurst)

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

		// Enforce per-connection message rate limit (5 per sec, burst 10)
		if !msgLimiter.Allow() {
			h.logger.WarnContext(ctx, "per-connection message rate limit exceeded")
			errResp, _ := json.Marshal(NewErrorMessage("rate limit exceeded: too many messages, please slow down"))
			_ = conn.Write(ctx, websocket.MessageText, errResp)
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

		// Process message through NLU chain
		resp, err := h.chain.Respond(ctx, nlu.Request{Text: incoming.Text})
		if err != nil {
			h.logger.ErrorContext(ctx, "nlu chain failed", "err", err)
			errResp, _ := json.Marshal(NewErrorMessage("failed to process message"))
			_ = conn.Write(ctx, websocket.MessageText, errResp)
			continue
		}

		outgoing := NewOutgoingMessage(incoming.Type, resp.Text)
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
