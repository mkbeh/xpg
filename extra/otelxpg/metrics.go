package otelxpg

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

const instrumentationName = "github.com/mkbeh/xpg/extra/otelxpg"

// Metrics exports xpg pool statistics through OpenTelemetry.
//
// Metrics is safe for concurrent use and reuse across multiple pools.
type Metrics struct {
	meterProvider metric.MeterProvider
}

// metricsRegistration represents one OpenTelemetry callback registration.
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
			otel.Handle(fmt.Errorf("otelxpg: unregister metrics: %w", err))
		}
	})
}
