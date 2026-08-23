package otelxpg

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

const instrumentationName = "github.com/mkbeh/xpg/extra/otelxpg"

// Metrics exports xpg statistics through OpenTelemetry.
//
// Metrics is immutable after construction and may be reused for multiple
// pool registrations.
type Metrics struct {
	meterProvider metric.MeterProvider
}

// metricsRegistration owns one OpenTelemetry callback registration.
type metricsRegistration struct {
	registration metric.Registration
	closeOnce    sync.Once
}

func (m *metricsRegistration) Close() {
	if m == nil || m.registration == nil {
		return
	}

	m.closeOnce.Do(func() {
		if err := m.registration.Unregister(); err != nil {
			otel.Handle(
				fmt.Errorf("otelxpg: unregister metrics: %w", err),
			)
		}
	})
}
