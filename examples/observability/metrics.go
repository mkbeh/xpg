package main

import (
	"fmt"
	"net/http"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

func newMeterProvider(resource *resource.Resource) (*sdkmetric.MeterProvider, http.Handler, error) {
	registry := promclient.NewRegistry()

	exporter, err := otelprom.New(
		otelprom.WithRegisterer(registry),
		otelprom.WithoutScopeInfo(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create Prometheus exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource),
		sdkmetric.WithReader(exporter),
	)

	handler := promhttp.HandlerFor(
		registry,
		promhttp.HandlerOpts{},
	)

	return meterProvider, handler, nil
}
