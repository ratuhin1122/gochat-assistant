package server_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ratuhin1122/gochat-assistant/internal/server"
)

func TestRouter_Healthz(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := server.New(logger)

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
	r.Mount("/", server.New(logger))

	// Directly test the panic handler with server router
	mux, ok := server.New(logger).(*chi.Mux)
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
