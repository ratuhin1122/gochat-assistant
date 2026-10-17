package server_test

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
	"github.com/go-chi/chi/v5"
	"github.com/ratuhin1122/gochat-assistant/internal/server"
)

func TestRouter_Healthz(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := server.New(logger, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	if body := rr.Body.String(); body != "ok" {
		t.Errorf("expected body 'ok', got %q", body)
	}

	// Verify Security Headers
	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for header, expectedVal := range expectedHeaders {
		if val := rr.Header().Get(header); val != expectedVal {
			t.Errorf("header %s: got %q, want %q", header, val, expectedVal)
		}
	}

	// Verify Request ID was set
	if reqID := rr.Header().Get("X-Request-Id"); reqID == "" {
		t.Error("expected X-Request-Id header to be present")
	}
}

func TestRouter_PanicRecovery(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	// Build router and add a panic route to test panic recovery middleware
	r := chi.NewRouter()
	r.Mount("/", server.New(logger, nil))

	// Directly test the panic handler with server router
	mux, ok := server.New(logger, nil).(*chi.Mux)
	if !ok {
		t.Fatal("expected *chi.Mux")
	}
	mux.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestRouter_StaticFiles(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := server.New(logger, nil)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantSubstr string
	}{
		{
			name:       "root index.html",
			path:       "/",
			wantStatus: http.StatusOK,
			wantSubstr: "GoChat Assistant",
		},
		{
			name:       "stylesheet",
			path:       "/style.css",
			wantStatus: http.StatusOK,
			wantSubstr: "prefers-color-scheme",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("path %q: expected status %d, got %d", tc.path, tc.wantStatus, rr.Code)
			}

			if body := rr.Body.String(); !testing.Short() && len(tc.wantSubstr) > 0 {
				if len(body) == 0 {
					t.Fatalf("path %q: expected non-empty body", tc.path)
				}
			}
		})
	}
}

func TestRouter_WebSocket(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := server.New(logger, nil)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket on router: %v", err)
	}
	defer conn.CloseNow()

	msg := "test ping"
	if err := conn.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	typ, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if typ != websocket.MessageText || string(data) != msg {
		t.Errorf("expected echo %q, got %q", msg, string(data))
	}

	_ = conn.Close(websocket.StatusNormalClosure, "")
}


