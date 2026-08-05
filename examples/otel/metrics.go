package main

import (
	"context"
	"fmt"
	"net/http"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

type metricsRuntime struct {
	handler       http.Handler
	meterProvider *sdkmetric.MeterProvider
}

func newMetricsRuntime(ctx context.Context) (*metricsRuntime, error) {
	registry := promclient.NewRegistry()

	exporter, err := otelprom.New(
		otelprom.WithRegisterer(registry),
		otelprom.WithoutScopeInfo(),
	)
	if err != nil {
		return nil, fmt.Errorf("create Prometheus exporter: %w", err)
	}

	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(
				"xpg-observability-example",
			),
			semconv.ServiceVersion("dev"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exporter),
	)

	otel.SetMeterProvider(meterProvider)

	return &metricsRuntime{
		handler: promhttp.HandlerFor(
			registry,
			promhttp.HandlerOpts{},
		),
		meterProvider: meterProvider,
	}, nil
}

func (m *metricsRuntime) Handler() http.Handler {
	return m.handler
}

func (m *metricsRuntime) Shutdown(ctx context.Context) error {
	return m.meterProvider.Shutdown(ctx)
}
