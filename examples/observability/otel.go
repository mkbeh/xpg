package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

const tracingInstrumentationName = "github.com/mkbeh/xpg/examples/observability"

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
		return nil, fmt.Errorf(
			"create OpenTelemetry resource: %w",
			err,
		)
	}

	return res, nil
}

func newTracerProvider(
	resource *resource.Resource,
) (*sdktrace.TracerProvider, error) {
	exporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create stdout trace exporter: %w",
			err,
		)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource),
		// The synchronous processor keeps the example easy to inspect.
		// Production applications should normally use a batch processor with
		// an OTLP exporter.
		sdktrace.WithSyncer(exporter),
	)

	return tracerProvider, nil
}
