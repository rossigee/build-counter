package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const tracingTimeout = 10 * time.Second

var tracer trace.Tracer

// initTracing initializes OpenTelemetry tracing if environment variables are set
func initTracing() (func(), error) {
	// Check if tracing is enabled
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	}

	if endpoint == "" {
		log.Println("OpenTelemetry tracing disabled (no OTEL_EXPORTER_OTLP_ENDPOINT set)")
		tracer = otel.Tracer("build-counter")
		return func() {}, nil
	}

	log.Printf("Initializing OpenTelemetry tracing with endpoint: %s", endpoint)

	// Create OTLP HTTP exporter
	ctx := context.Background()

	// Configure exporter options
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
	}

	// Add headers if provided
	if headers := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"); headers != "" {
		// Parse headers from environment variable
		// Format: key1=value1,key2=value2
		headerMap := make(map[string]string)
		// Simple parsing - in production you might want more robust parsing
		log.Printf("Using OTEL headers: %s", headers)
		opts = append(opts, otlptracehttp.WithHeaders(headerMap))
	}

	// Check if insecure connection is requested
	if os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true" {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "build-counter"
	}

	serviceVersion := os.Getenv("OTEL_SERVICE_VERSION")
	if serviceVersion == "" {
		serviceVersion = version
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Create tracer
	tracer = tp.Tracer("build-counter")

	log.Println("OpenTelemetry tracing initialized successfully")

	// Return cleanup function
	return func() {
		log.Println("Shutting down OpenTelemetry tracing...")
		ctx, cancel := context.WithTimeout(context.Background(), tracingTimeout)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down trace provider: %v", err)
		}
	}, nil
}

// startSpan starts a new span with the given name and returns the span
func startSpan(ctx context.Context, name string) trace.Span {
	if tracer == nil {
		// Return a no-op span if tracer is not initialized (e.g., during tests)
		return trace.SpanFromContext(ctx)
	}
	_, span := tracer.Start(ctx, name)
	return span
}

// recordError records an error in the span
func recordError(span trace.Span, err error) {
	if err != nil && span != nil {
		span.RecordError(err)
	}
}
