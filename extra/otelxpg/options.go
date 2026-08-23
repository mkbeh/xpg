package otelxpg

import (
	"go.opentelemetry.io/otel/metric"
)

// MetricsOption configures OpenTelemetry metrics.
//
// The interface is sealed so options can only be created by this package.
type MetricsOption interface {
	apply(*metricsSettings)
}

type metricsOptionFunc func(*metricsSettings)

func (option metricsOptionFunc) apply(settings *metricsSettings) {
	option(settings)
}

type metricsSettings struct {
	meterProvider metric.MeterProvider
}

// NewMetrics creates an OpenTelemetry metrics implementation.
//
// By default, metrics use the global OpenTelemetry MeterProvider. The returned
// value is immutable and may be reused for multiple pools.
func NewMetrics(options ...MetricsOption) *Metrics {
	settings := metricsSettings{}

	for _, option := range options {
		if option == nil {
			continue
		}

		option.apply(&settings)
	}

	return &Metrics{
		meterProvider: settings.meterProvider,
	}
}

// WithMeterProvider configures the MeterProvider used for metrics.
//
// The caller owns the provider and must shut it down after all instrumented
// pools have been closed.
func WithMeterProvider(provider metric.MeterProvider) MetricsOption {
	return metricsOptionFunc(func(settings *metricsSettings) {
		if provider != nil {
			settings.meterProvider = provider
		}
	})
}
