package monitoring

import (
	"context"
	"log"

	"go.opentelemetry.io/exporter/otlp/otlptrace"
	"go.opentelemetry.io/exporter/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/sdk/resource"
	sdktrace "go.opentelemetry.io/sdk/trace"
	semconv "go.opentelemetry.io/semconv/v1.17.0"
)

func InitializeTracing(ctx context.Context, serviceName, collectorURL string) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(collectorURL))
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	return tp, nil
}

func ShutdownTracing(ctx context.Context, tp *sdktrace.TracerProvider) {
	if err := tp.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down tracer provider: %v", err)
	}
}
