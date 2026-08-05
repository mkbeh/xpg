package xpgotel

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/mkbeh/xpg"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const instrumentationName = "github.com/mkbeh/xpg/otel"

const (
	connectionCountMetricName                = "db.client.connection.count"
	connectionMaxMetricName                  = "db.client.connection.max"
	connectionConstructingMetricName         = "xpg.pool.connection.constructing"
	connectionAcquireCountMetricName         = "xpg.pool.connection.acquire.count"
	connectionAcquireTimeMetricName          = "xpg.pool.connection.acquire.time"
	connectionAcquireCanceledCountMetricName = "xpg.pool.connection.acquire.canceled.count"
	connectionAcquireEmptyCountMetricName    = "xpg.pool.connection.acquire.empty.count"
	connectionAcquireEmptyWaitTimeMetricName = "xpg.pool.connection.acquire.empty.wait_time"
	connectionCreateCountMetricName          = "xpg.pool.connection.create.count"
	connectionDestroyCountMetricName         = "xpg.pool.connection.destroy.count"
)

const (
	dbSystemNameAttribute    = "db.system.name"
	poolNameAttribute        = "db.client.connection.pool.name"
	connectionStateAttribute = "db.client.connection.state"
	destroyReasonAttribute   = "xpg.pool.connection.destroy.reason"
)

const (
	dbSystemPostgreSQL = "postgresql"

	connectionStateIdle = "idle"
	connectionStateUsed = "used"

	destroyReasonIdleTimeout = "idle_timeout"
	destroyReasonLifetime    = "lifetime"
)

type poolMetricInstruments struct {
	connectionCount         metric.Int64ObservableUpDownCounter
	connectionMax           metric.Int64ObservableUpDownCounter
	constructingConnections metric.Int64ObservableGauge
	acquireCount            metric.Int64ObservableCounter
	acquireTime             metric.Float64ObservableCounter
	canceledAcquireCount    metric.Int64ObservableCounter
	emptyAcquireCount       metric.Int64ObservableCounter
	emptyAcquireWaitTime    metric.Float64ObservableCounter
	createdConnections      metric.Int64ObservableCounter
	destroyedConnections    metric.Int64ObservableCounter
}

type poolMetricAttributes struct {
	base              metric.ObserveOption
	idle              metric.ObserveOption
	used              metric.ObserveOption
	destroyedIdle     metric.ObserveOption
	destroyedLifetime metric.ObserveOption
}

// Metrics exports xpg pool statistics through OpenTelemetry.
type Metrics struct {
	meterProvider metric.MeterProvider
}

type poolMetrics struct {
	registration metric.Registration
	closeOnce    sync.Once
}

// Register registers metrics for one xpg Pool.
func (m *Metrics) Register(
	pool *xpg.Pool,
) (xpg.PoolMetricsRegistration, error) {
	if m == nil {
		return nil, errors.New("xpg/otel: metrics is nil")
	}

	provider := m.meterProvider
	if provider == nil {
		provider = otel.GetMeterProvider()
	}

	return registerPoolMetrics(pool, provider)
}

func registerPoolMetrics(
	pool *xpg.Pool,
	provider metric.MeterProvider,
) (xpg.PoolMetricsRegistration, error) {
	meter := provider.Meter(instrumentationName)

	instruments, err := newPoolMetricInstruments(meter)
	if err != nil {
		return nil, err
	}

	attributes := newPoolMetricAttributes(pool.Name(), pool.Labels())

	registration, err := meter.RegisterCallback(
		func(_ context.Context, observer metric.Observer) error {
			instruments.observe(observer, pool.Stats(), attributes)
			return nil
		},
		instruments.observables()...,
	)
	if err != nil {
		return nil, fmt.Errorf("xpg/otel: register pool metrics callback: %w", err)
	}

	return &poolMetrics{
		registration: registration,
	}, nil
}

func (m *poolMetrics) Close() {
	if m == nil || m.registration == nil {
		return
	}

	m.closeOnce.Do(func() {
		if err := m.registration.Unregister(); err != nil {
			otel.Handle(
				fmt.Errorf("xpg/otel: unregister pool metrics: %w", err),
			)
		}
	})
}

func (i poolMetricInstruments) observe(
	observer metric.Observer,
	stats xpg.PoolStats,
	attributes poolMetricAttributes,
) {
	observer.ObserveInt64(
		i.connectionCount,
		int64(stats.IdleConns),
		attributes.idle,
	)
	observer.ObserveInt64(
		i.connectionCount,
		int64(stats.AcquiredConns),
		attributes.used,
	)
	observer.ObserveInt64(
		i.connectionMax,
		int64(stats.MaxConns),
		attributes.base,
	)
	observer.ObserveInt64(
		i.constructingConnections,
		int64(stats.ConstructingConns),
		attributes.base,
	)
	observer.ObserveInt64(
		i.acquireCount,
		stats.AcquireCount,
		attributes.base,
	)
	observer.ObserveFloat64(
		i.acquireTime,
		stats.AcquireDuration.Seconds(),
		attributes.base,
	)
	observer.ObserveInt64(
		i.canceledAcquireCount,
		stats.CanceledAcquireCount,
		attributes.base,
	)
	observer.ObserveInt64(
		i.emptyAcquireCount,
		stats.EmptyAcquireCount,
		attributes.base,
	)
	observer.ObserveFloat64(
		i.emptyAcquireWaitTime,
		stats.EmptyAcquireWaitTime.Seconds(),
		attributes.base,
	)
	observer.ObserveInt64(
		i.createdConnections,
		stats.NewConnsCount,
		attributes.base,
	)
	observer.ObserveInt64(
		i.destroyedConnections,
		stats.MaxIdleDestroyCount,
		attributes.destroyedIdle,
	)
	observer.ObserveInt64(
		i.destroyedConnections,
		stats.MaxLifetimeDestroyCount,
		attributes.destroyedLifetime,
	)
}

func (i poolMetricInstruments) observables() []metric.Observable {
	return []metric.Observable{
		i.connectionCount,
		i.connectionMax,
		i.constructingConnections,
		i.acquireCount,
		i.acquireTime,
		i.canceledAcquireCount,
		i.emptyAcquireCount,
		i.emptyAcquireWaitTime,
		i.createdConnections,
		i.destroyedConnections,
	}
}

func newPoolMetricInstruments(meter metric.Meter) (poolMetricInstruments, error) {
	var instruments poolMetricInstruments

	var err error

	instruments.connectionCount, err = meter.Int64ObservableUpDownCounter(
		connectionCountMetricName,
		metric.WithDescription(
			"The number of connections currently used or idle in the pool.",
		),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return poolMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			connectionCountMetricName,
			err,
		)
	}

	instruments.connectionMax, err = meter.Int64ObservableUpDownCounter(
		connectionMaxMetricName,
		metric.WithDescription(
			"The maximum number of open connections allowed by the pool.",
		),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return poolMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			connectionMaxMetricName,
			err,
		)
	}

	instruments.constructingConnections, err = meter.Int64ObservableGauge(
		connectionConstructingMetricName,
		metric.WithDescription(
			"The number of connections currently being created by the pool.",
		),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return poolMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			connectionConstructingMetricName,
			err,
		)
	}

	instruments.acquireCount, err = meter.Int64ObservableCounter(
		connectionAcquireCountMetricName,
		metric.WithDescription(
			"The cumulative number of successful connection acquires.",
		),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return poolMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			connectionAcquireCountMetricName,
			err,
		)
	}

	instruments.acquireTime, err = meter.Float64ObservableCounter(
		connectionAcquireTimeMetricName,
		metric.WithDescription(
			"The cumulative time spent acquiring connections.",
		),
		metric.WithUnit("s"),
	)
	if err != nil {
		return poolMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			connectionAcquireTimeMetricName,
			err,
		)
	}

	instruments.canceledAcquireCount, err = meter.Int64ObservableCounter(
		connectionAcquireCanceledCountMetricName,
		metric.WithDescription(
			"The cumulative number of connection acquires canceled by context.",
		),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return poolMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			connectionAcquireCanceledCountMetricName,
			err,
		)
	}

	instruments.emptyAcquireCount, err = meter.Int64ObservableCounter(
		connectionAcquireEmptyCountMetricName,
		metric.WithDescription(
			"The cumulative number of successful acquires that waited because the pool was empty.",
		),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return poolMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			connectionAcquireEmptyCountMetricName,
			err,
		)
	}

	instruments.emptyAcquireWaitTime, err = meter.Float64ObservableCounter(
		connectionAcquireEmptyWaitTimeMetricName,
		metric.WithDescription(
			"The cumulative time spent waiting for a connection while the pool was empty.",
		),
		metric.WithUnit("s"),
	)
	if err != nil {
		return poolMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			connectionAcquireEmptyWaitTimeMetricName,
			err,
		)
	}

	instruments.createdConnections, err = meter.Int64ObservableCounter(
		connectionCreateCountMetricName,
		metric.WithDescription(
			"The cumulative number of connections opened by the pool.",
		),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return poolMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			connectionCreateCountMetricName,
			err,
		)
	}

	instruments.destroyedConnections, err = meter.Int64ObservableCounter(
		connectionDestroyCountMetricName,
		metric.WithDescription(
			"The cumulative number of connections destroyed by pool lifecycle limits.",
		),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return poolMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			connectionDestroyCountMetricName,
			err,
		)
	}

	return instruments, nil
}

func newPoolMetricAttributes(name string, labels map[string]string) poolMetricAttributes {
	var base []attribute.KeyValue

	for key, value := range labels {
		base = append(base, attribute.String(key, value))
	}

	// System attributes are appended last, so xpg-controlled values win when
	// user labels contain duplicate keys.
	base = append(
		base,
		attribute.String(dbSystemNameAttribute, dbSystemPostgreSQL),
		attribute.String(poolNameAttribute, name),
	)

	option := func(extra ...attribute.KeyValue) metric.ObserveOption {
		return metric.WithAttributeSet(
			attribute.NewSet(
				slices.Concat(base, extra)...,
			),
		)
	}

	return poolMetricAttributes{
		base: option(),
		idle: option(
			attribute.String(
				connectionStateAttribute,
				connectionStateIdle,
			),
		),
		used: option(
			attribute.String(
				connectionStateAttribute,
				connectionStateUsed,
			),
		),
		destroyedIdle: option(
			attribute.String(
				destroyReasonAttribute,
				destroyReasonIdleTimeout,
			),
		),
		destroyedLifetime: option(
			attribute.String(
				destroyReasonAttribute,
				destroyReasonLifetime,
			),
		),
	}
}
