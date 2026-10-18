// Package observability provides Prometheus metrics instrumentation for the gochat-assistant application.
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ActiveWSConnections tracks the current number of active WebSocket connections.
	ActiveWSConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gochat",
		Name:      "active_websocket_connections",
		Help:      "Number of currently active WebSocket connections.",
	})

	// ActiveWebSocketConnections is an alias for ActiveWSConnections.
	ActiveWebSocketConnections = ActiveWSConnections

	// MessagesByResponder tracks the number of messages processed by each responder.
	MessagesByResponder = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "gochat",
		Name:      "messages_by_responder_total",
		Help:      "Total number of messages processed by responder.",
	}, []string{"responder"})

	// HTTPRequestDuration tracks the duration of HTTP requests in seconds.
	HTTPRequestDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gochat",
		Name:      "http_request_duration_seconds",
		Help:      "Duration of HTTP requests in seconds.",
		Buckets:   prometheus.DefBuckets,
	})

	// LLMErrors tracks the total number of errors encountered when calling LLM providers.
	LLMErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gochat",
		Name:      "llm_errors_total",
		Help:      "Total number of LLM provider generation errors.",
	})
)

func init() {
	// Initialize common responder labels to 0 so they appear in /metrics immediately.
	MessagesByResponder.WithLabelValues("faq")
	MessagesByResponder.WithLabelValues("weather")
	MessagesByResponder.WithLabelValues("llm")
	MessagesByResponder.WithLabelValues("default")
}

// WSConnect increments the active WebSocket connections gauge.
func WSConnect() {
	ActiveWSConnections.Inc()
}

// WSDisconnect decrements the active WebSocket connections gauge.
func WSDisconnect() {
	ActiveWSConnections.Dec()
}

// RecordMessageByResponder increments the message counter for the specified responder.
func RecordMessageByResponder(responder string) {
	MessagesByResponder.WithLabelValues(responder).Inc()
}

// RecordHTTPRequestDuration observes the duration of an HTTP request in seconds.
func RecordHTTPRequestDuration(durationSeconds float64) {
	HTTPRequestDuration.Observe(durationSeconds)
}

// RecordLLMError increments the LLM error counter.
func RecordLLMError(provider ...string) {
	LLMErrors.Inc()
}
