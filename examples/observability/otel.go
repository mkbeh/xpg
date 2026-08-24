package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

const tracingInstrumentationName = "github.com/mkbeh/xpg/examples/observability"

type tracingRuntime struct {
	tracerProvider *sdktrace.TracerProvider
}

func newOTelResource(ctx context.Context) (*resource.Resource, error) {
	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName("xpg-observability-example"),
			semconv.ServiceVersion("dev"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	return res, nil
}

func newTracingRuntime(res *resource.Resource) (*tracingRuntime, error) {
	exporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return nil, fmt.Errorf("create stdout trace exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		// A synchronous processor keeps this runnable example easy to inspect.
		// Production applications should normally prefer WithBatcher with an
		// OTLP exporter.
		sdktrace.WithSyncer(exporter),
	)

	return &tracingRuntime{
		tracerProvider: tracerProvider,
	}, nil
}

func (t *tracingRuntime) TracerProvider() *sdktrace.TracerProvider {
	return t.tracerProvider
}

func (t *tracingRuntime) Tracer() trace.Tracer {
	return t.tracerProvider.Tracer(tracingInstrumentationName)
}

func (t *tracingRuntime) Shutdown(ctx context.Context) error {
	return t.tracerProvider.Shutdown(ctx)
}
