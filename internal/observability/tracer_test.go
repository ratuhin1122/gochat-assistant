package observability_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ratuhin1122/gochat-assistant/internal/observability"
	"go.opentelemetry.io/otel/trace"
)

func TestInitTracing_NoopWhenUnset(t *testing.T) {
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	ctx := context.Background()
	shutdown, err := observability.InitTracing(ctx, "test-service")
	if err != nil {
		t.Fatalf("expected no error when OTEL_EXPORTER_OTLP_ENDPOINT is unset, got: %v", err)
	}

	tracer := observability.Tracer()
	if tracer == nil {
		t.Fatal("expected non-nil tracer")
	}

	_, span := tracer.Start(ctx, "test.span")
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	span.End()

	shutdownCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	if err := shutdown(shutdownCtx); err != nil {
		t.Errorf("expected clean shutdown from no-op tracer, got: %v", err)
	}
}

func TestInitTracing_WithEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")

	ctx := context.Background()
	shutdown, err := observability.InitTracing(ctx, "test-service")
	if err != nil {
		t.Fatalf("failed to init tracing with endpoint: %v", err)
	}

	tracer := observability.Tracer()
	ctx, span := tracer.Start(ctx, "test.endpoint.span", trace.WithSpanKind(trace.SpanKindInternal))
	span.End()

	shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Shutdown should flush and succeed (or timeout if no collector is running, without panic)
	_ = shutdown(shutdownCtx)
}
