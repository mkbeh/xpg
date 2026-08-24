package main

import (
	"context"
	"fmt"
	"net/http"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

type metricsRuntime struct {
	handler       http.Handler
	meterProvider *sdkmetric.MeterProvider
}

func newMetricsRuntime(res *resource.Resource) (*metricsRuntime, error) {
	registry := promclient.NewRegistry()

	exporter, err := otelprom.New(
		otelprom.WithRegisterer(registry),
		otelprom.WithoutScopeInfo(),
	)
	if err != nil {
		return nil, fmt.Errorf("create Prometheus exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exporter),
	)

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

func (m *metricsRuntime) MeterProvider() *sdkmetric.MeterProvider {
	return m.meterProvider
}

func (m *metricsRuntime) Shutdown(ctx context.Context) error {
	return m.meterProvider.Shutdown(ctx)
}
