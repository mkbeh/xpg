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

// NewMetrics creates an OpenTelemetry metrics integration.
//
// When no MeterProvider is configured, the global OpenTelemetry MeterProvider
// is used. The returned value is safe for concurrent use and reuse across
// multiple pools.
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
// A nil provider leaves the global OpenTelemetry MeterProvider in use. The
// caller retains ownership of a non-nil provider and is responsible for
// shutting it down after all instrumented pools have been closed.
func WithMeterProvider(provider metric.MeterProvider) MetricsOption {
	return metricsOptionFunc(func(settings *metricsSettings) {
		if provider != nil {
			settings.meterProvider = provider
		}
	})
}

type metricsOptionFunc func(*metricsSettings)

func (option metricsOptionFunc) apply(settings *metricsSettings) {
	option(settings)
}

type metricsSettings struct {
	meterProvider metric.MeterProvider
}
