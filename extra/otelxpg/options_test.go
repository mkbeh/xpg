package otelxpg

import (
	"testing"

	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewMetrics(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics(nil)

	if metrics == nil {
		t.Fatal("expected metrics")
	}

	if metrics.meterProvider != nil {
		t.Fatal("expected nil meter provider")
	}
}

func TestWithMeterProvider(t *testing.T) {
	t.Parallel()

	provider := noop.NewMeterProvider()

	metrics := NewMetrics(
		WithMeterProvider(provider),
	)

	if metrics.meterProvider != provider {
		t.Fatal("unexpected meter provider")
	}
}

func TestWithMeterProviderLastWins(t *testing.T) {
	t.Parallel()

	first := noop.NewMeterProvider()
	second := noop.NewMeterProvider()

	metrics := NewMetrics(
		WithMeterProvider(first),
		WithMeterProvider(second),
	)

	if metrics.meterProvider != second {
		t.Fatal("expected last meter provider to win")
	}
}

func TestWithMeterProviderNilIgnored(t *testing.T) {
	t.Parallel()

	provider := noop.NewMeterProvider()

	metrics := NewMetrics(
		WithMeterProvider(provider),
		WithMeterProvider(nil),
	)

	if metrics.meterProvider != provider {
		t.Fatal("expected nil meter provider option to be ignored")
	}
}
