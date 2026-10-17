// Package server builds the application HTTP router and its middleware stack.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// New constructs and returns the application http.Handler.
//
// Middleware order (outermost → innermost):
//  1. RequestID   – stamps X-Request-Id on every request
//  2. Recover     – catches panics and logs them via slog
//  3. SecHeaders  – adds conservative security response headers
//  4. ReqLogger   – logs method/path/status/duration via slog
//  5. Timeout(30s)– applied per-group; /ws routes MUST be registered
//     outside this group to avoid the deadline.
func New(logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if id := middleware.GetReqID(r.Context()); id != "" {
				w.Header().Set("X-Request-Id", id)
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Use(recoverMiddleware(logger))
	r.Use(securityHeaders)
	r.Use(requestLogger(logger))

	// ── Timed group (regular HTTP routes) ─────────────────────────────────
	// WebSocket / long-poll routes must be added to the router OUTSIDE this
	// group so they are not subject to the 30 s deadline.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Second))

		r.Get("/healthz", handleHealthz)
	})

	return r
}

// handleHealthz is the liveness probe endpoint.
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// securityHeaders sets conservative security-related response headers on
// every response before passing control to the next handler.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// recoverMiddleware catches panics, logs a structured error with slog, and
// writes a 500 Internal Server Error response.
func recoverMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.ErrorContext(r.Context(), "panic recovered",
						"request_id", middleware.GetReqID(r.Context()),
						"method", r.Method,
						"path", r.URL.Path,
						"panic", rec,
					)
					http.Error(w,
						http.StatusText(http.StatusInternalServerError),
						http.StatusInternalServerError,
					)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// requestLogger logs a structured line for every completed request.
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			logger.InfoContext(r.Context(), "request",
				"request_id", middleware.GetReqID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
