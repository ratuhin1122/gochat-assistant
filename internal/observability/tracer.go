package observability

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TracerName is the instrumentation scope name for gochat-assistant.
const TracerName = "github.com/ratuhin1122/gochat-assistant"

// Tracer returns the named tracer for the application.
func Tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer(TracerName)
}

// InitTracing configures OpenTelemetry tracing.
// If OTEL_EXPORTER_OTLP_ENDPOINT is set, it initializes an OTLP exporter (HTTP or gRPC)
// and registers the global TracerProvider.
// If OTEL_EXPORTER_OTLP_ENDPOINT is empty, a no-op TracerProvider is configured.
// It returns a shutdown function that flushes pending spans and releases resources.
func InitTracing(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	if serviceName == "" {
		serviceName = "gochat-assistant"
	}

	var exporter sdktrace.SpanExporter
	var err error

	// If the endpoint specifies port 4317 or begins with grpc://, use gRPC exporter; otherwise use HTTP exporter.
	if strings.Contains(endpoint, "4317") || strings.HasPrefix(endpoint, "grpc://") {
		clean := strings.TrimPrefix(endpoint, "grpc://")
		insecure := true
		if strings.HasPrefix(clean, "https://") {
			clean = strings.TrimPrefix(clean, "https://")
			insecure = false
		} else if strings.HasPrefix(clean, "http://") {
			clean = strings.TrimPrefix(clean, "http://")
			insecure = true
		}
		var opts []otlptracegrpc.Option
		opts = append(opts, otlptracegrpc.WithEndpoint(clean))
		if insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("create otlp grpc trace exporter: %w", err)
		}
	} else {
		clean := endpoint
		insecure := true
		if strings.HasPrefix(clean, "https://") {
			clean = strings.TrimPrefix(clean, "https://")
			insecure = false
		} else if strings.HasPrefix(clean, "http://") {
			clean = strings.TrimPrefix(clean, "http://")
			insecure = true
		}
		var opts []otlptracehttp.Option
		opts = append(opts, otlptracehttp.WithEndpoint(clean))
		if insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exporter, err = otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("create otlp http trace exporter: %w", err)
		}
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := func(sCtx context.Context) error {
		if err := tp.Shutdown(sCtx); err != nil {
			return fmt.Errorf("shutdown trace provider: %w", err)
		}
		return nil
	}

	return shutdown, nil
}
