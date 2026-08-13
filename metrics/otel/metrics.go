package xpgotel

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

const instrumentationName = "github.com/mkbeh/xpg/otel"

// Metrics exports xpg statistics through OpenTelemetry.
//
// Metrics is immutable after construction and may be reused for multiple
// pool and cache registrations.
type Metrics struct {
	meterProvider metric.MeterProvider
}

// metricsRegistration owns one OpenTelemetry callback registration.
//
// The same implementation is used by pool and cache metrics because both
// registrations have identical lifecycle semantics.
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
				fmt.Errorf("xpg/otel: unregister metrics: %w", err),
			)
		}
	})
}
