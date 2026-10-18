package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ratuhin1122/gochat-assistant/internal/observability"
)

func TestMetrics_RegistrationsAndOperations(t *testing.T) {
	// Test ActiveWSConnections
	observability.WSConnect()
	observability.WSDisconnect()
	observability.ActiveWSConnections.Inc()
	observability.ActiveWebSocketConnections.Dec()

	// Test MessagesByResponder
	observability.RecordMessageByResponder("faq")
	observability.RecordMessageByResponder("weather")
	observability.RecordMessageByResponder("llm")
	observability.RecordMessageByResponder("default")
	observability.MessagesByResponder.WithLabelValues("custom").Inc()

	// Test HTTPRequestDuration
	observability.RecordHTTPRequestDuration(0.025)
	observability.HTTPRequestDuration.Observe(0.050)

	// Test LLMErrors
	observability.RecordLLMError()
	observability.RecordLLMError("gemini")
	observability.LLMErrors.Inc()

	// Gather metrics from default registry and verify exposition
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from promhttp.Handler, got %d", rec.Code)
	}

	body := rec.Body.String()

	expectedMetrics := []string{
		"gochat_active_websocket_connections",
		"gochat_messages_by_responder_total",
		"gochat_http_request_duration_seconds",
		"gochat_llm_errors_total",
	}

	for _, m := range expectedMetrics {
		if !strings.Contains(body, m) {
			t.Errorf("expected metrics output to contain %q, but was missing", m)
		}
	}
}
