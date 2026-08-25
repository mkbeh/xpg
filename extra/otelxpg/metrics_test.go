package otelxpg

import (
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mkbeh/xpg"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestMetricsRegistration(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics(
		WithMeterProvider(noop.NewMeterProvider()),
	)

	pool := newTestPool(t, metrics)

	pool.Close()
	pool.Close()
}

func TestMetricsRegisterNilReceiver(t *testing.T) {
	t.Parallel()

	var metrics *Metrics

	registration, err := metrics.Register(nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if registration != nil {
		t.Fatal("expected nil registration")
	}
}

func TestMetricsRegisterNilPool(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics(
		WithMeterProvider(noop.NewMeterProvider()),
	)

	registration, err := metrics.Register(nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if registration != nil {
		t.Fatal("expected nil registration")
	}
}

func TestWithMeterProviderNilUsesGlobalProvider(t *testing.T) {
	metrics := NewMetrics(
		WithMeterProvider(nil),
	)

	pool := newTestPool(t, metrics)
	pool.Close()
}

func TestPoolMetricInstruments(t *testing.T) {
	t.Parallel()

	meter := &recordingMeter{}

	instruments, err := newPoolMetricInstruments(meter)
	if err != nil {
		t.Fatalf("create instruments: %v", err)
	}

	want := []instrumentSpec{
		{
			name:        connectionCountMetricName,
			kind:        "int64_observable_up_down_counter",
			description: "The number of connections currently in the state described by db.client.connection.state.",
			unit:        "{connection}",
		},
		{
			name:        connectionMaxMetricName,
			kind:        "int64_observable_up_down_counter",
			description: "The maximum number of open connections allowed by the pool.",
			unit:        "{connection}",
		},
		{
			name:        connectionConstructingMetricName,
			kind:        "int64_observable_gauge",
			description: "The number of connections currently being created by the pool.",
			unit:        "{connection}",
		},
		{
			name:        connectionAcquireCountMetricName,
			kind:        "int64_observable_counter",
			description: "The cumulative number of successful connection acquires.",
			unit:        "{request}",
		},
		{
			name:        connectionAcquireTimeMetricName,
			kind:        "float64_observable_counter",
			description: "The cumulative time spent acquiring connections.",
			unit:        "s",
		},
		{
			name:        connectionAcquireCanceledCountMetricName,
			kind:        "int64_observable_counter",
			description: "The cumulative number of connection acquires canceled by context.",
			unit:        "{request}",
		},
		{
			name:        connectionAcquireEmptyCountMetricName,
			kind:        "int64_observable_counter",
			description: "The cumulative number of successful acquires that waited because the pool was empty.",
			unit:        "{request}",
		},
		{
			name:        connectionAcquireEmptyWaitTimeMetricName,
			kind:        "float64_observable_counter",
			description: "The cumulative time spent waiting for a connection while the pool was empty.",
			unit:        "s",
		},
		{
			name:        connectionCreateCountMetricName,
			kind:        "int64_observable_counter",
			description: "The cumulative number of connections created by the pool.",
			unit:        "{connection}",
		},
		{
			name:        connectionDestroyCountMetricName,
			kind:        "int64_observable_counter",
			description: "The cumulative number of connections destroyed by pool lifecycle limits.",
			unit:        "{connection}",
		},
	}

	if !slices.Equal(meter.instruments, want) {
		t.Fatalf(
			"instruments = %#v, want %#v",
			meter.instruments,
			want,
		)
	}

	if got, want := len(instruments.observables()), len(want); got != want {
		t.Fatalf("observable count = %d, want %d", got, want)
	}
}

func TestPoolMetricAttributes(t *testing.T) {
	t.Parallel()

	attributes := newPoolMetricAttributes(
		"test-pool",
		map[string]string{
			"region":                 "eu",
			dbSystemNameAttribute:    "custom-system",
			poolNameAttribute:        "custom-pool",
			connectionStateAttribute: "custom-state",
			destroyReasonAttribute:   "custom-reason",
		},
	)

	base := observeAttributes(attributes.base)

	requireStringAttribute(t, base, dbSystemNameAttribute, dbSystemPostgreSQL)
	requireStringAttribute(t, base, poolNameAttribute, "test-pool")
	requireStringAttribute(t, base, "region", "eu")
	requireStringAttribute(t, base, connectionStateAttribute, "custom-state")
	requireStringAttribute(t, base, destroyReasonAttribute, "custom-reason")

	idle := observeAttributes(attributes.idle)
	requireStringAttribute(t, idle, connectionStateAttribute, connectionStateIdle)

	used := observeAttributes(attributes.used)
	requireStringAttribute(t, used, connectionStateAttribute, connectionStateUsed)

	destroyedIdle := observeAttributes(attributes.destroyedIdle)
	requireStringAttribute(t, destroyedIdle, destroyReasonAttribute, destroyReasonIdleTimeout)

	destroyedLifetime := observeAttributes(attributes.destroyedLifetime)
	requireStringAttribute(t, destroyedLifetime, destroyReasonAttribute, destroyReasonLifetime)
}

func TestPoolMetricInstrumentsObserve(t *testing.T) {
	t.Parallel()

	stats := xpg.PoolStats{
		AcquiredConns:           1,
		ConstructingConns:       2,
		IdleConns:               3,
		MaxConns:                4,
		AcquireCount:            5,
		AcquireDuration:         6 * time.Second,
		CanceledAcquireCount:    7,
		EmptyAcquireCount:       8,
		EmptyAcquireWaitTime:    9 * time.Second,
		NewConnsCount:           10,
		MaxIdleDestroyCount:     11,
		MaxLifetimeDestroyCount: 12,
	}

	attributes := newPoolMetricAttributes(
		"test-pool",
		map[string]string{
			"region": "eu",
		},
	)

	observer := &recordingObserver{}

	var instruments poolMetricInstruments

	instruments.observe(
		observer,
		stats,
		attributes,
	)

	wantInt64 := []int64{
		3,
		1,
		4,
		2,
		5,
		7,
		8,
		10,
		11,
		12,
	}

	if !slices.Equal(observer.int64Values, wantInt64) {
		t.Fatalf(
			"int64 observations = %v, want %v",
			observer.int64Values,
			wantInt64,
		)
	}

	wantFloat64 := []float64{
		6,
		9,
	}

	if !slices.Equal(observer.float64Values, wantFloat64) {
		t.Fatalf(
			"float64 observations = %v, want %v",
			observer.float64Values,
			wantFloat64,
		)
	}

	requireStringAttribute(
		t,
		observer.int64Attributes[0],
		connectionStateAttribute,
		connectionStateIdle,
	)
	requireStringAttribute(
		t,
		observer.int64Attributes[1],
		connectionStateAttribute,
		connectionStateUsed,
	)
	requireStringAttribute(
		t,
		observer.int64Attributes[8],
		destroyReasonAttribute,
		destroyReasonIdleTimeout,
	)
	requireStringAttribute(
		t,
		observer.int64Attributes[9],
		destroyReasonAttribute,
		destroyReasonLifetime,
	)

	for _, attributes := range observer.int64Attributes {
		requireStringAttribute(t, attributes, poolNameAttribute, "test-pool")
		requireStringAttribute(t, attributes, "region", "eu")
	}

	for _, attributes := range observer.float64Attributes {
		requireStringAttribute(t, attributes, poolNameAttribute, "test-pool")
		requireStringAttribute(t, attributes, "region", "eu")
	}
}

func TestMetricsRegistrationCloseOnce(t *testing.T) {
	t.Parallel()

	registration := &registrationStub{}

	metricsRegistration := &metricsRegistration{
		registration: registration,
	}

	metricsRegistration.Close()
	metricsRegistration.Close()

	if registration.calls != 1 {
		t.Fatalf(
			"unregister calls = %d, want 1",
			registration.calls,
		)
	}
}

func newTestPool(
	t *testing.T,
	metrics xpg.Metrics,
) *xpg.Pool {
	t.Helper()

	poolConfig, err := pgxpool.ParseConfig("")
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}

	pool, err := xpg.New(
		t.Context(),
		poolConfig,
		xpg.WithName("test-pool"),
		xpg.WithMetrics(metrics),
	)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	return pool
}

func observeAttributes(option metric.ObserveOption) attribute.Set {
	return metric.NewObserveConfig(
		[]metric.ObserveOption{option},
	).Attributes()
}

func requireStringAttribute(
	t *testing.T,
	attributes attribute.Set,
	key string,
	want string,
) {
	t.Helper()

	value, ok := attributes.Value(attribute.Key(key))
	if !ok {
		t.Fatalf("attribute %q is missing", key)
	}

	if got := value.AsString(); got != want {
		t.Fatalf(
			"attribute %q = %q, want %q",
			key,
			got,
			want,
		)
	}
}

type instrumentSpec struct {
	name        string
	kind        string
	description string
	unit        string
}

type recordingMeter struct {
	noop.Meter

	instruments []instrumentSpec
}

func (m *recordingMeter) Int64ObservableUpDownCounter(
	name string,
	options ...metric.Int64ObservableUpDownCounterOption,
) (metric.Int64ObservableUpDownCounter, error) {
	config := metric.NewInt64ObservableUpDownCounterConfig(options...)

	m.instruments = append(
		m.instruments,
		instrumentSpec{
			name:        name,
			kind:        "int64_observable_up_down_counter",
			description: config.Description(),
			unit:        config.Unit(),
		},
	)

	return m.Meter.Int64ObservableUpDownCounter(name, options...)
}

func (m *recordingMeter) Int64ObservableGauge(
	name string,
	options ...metric.Int64ObservableGaugeOption,
) (metric.Int64ObservableGauge, error) {
	config := metric.NewInt64ObservableGaugeConfig(options...)

	m.instruments = append(
		m.instruments,
		instrumentSpec{
			name:        name,
			kind:        "int64_observable_gauge",
			description: config.Description(),
			unit:        config.Unit(),
		},
	)

	return m.Meter.Int64ObservableGauge(name, options...)
}

func (m *recordingMeter) Int64ObservableCounter(
	name string,
	options ...metric.Int64ObservableCounterOption,
) (metric.Int64ObservableCounter, error) {
	config := metric.NewInt64ObservableCounterConfig(options...)

	m.instruments = append(
		m.instruments,
		instrumentSpec{
			name:        name,
			kind:        "int64_observable_counter",
			description: config.Description(),
			unit:        config.Unit(),
		},
	)

	return m.Meter.Int64ObservableCounter(name, options...)
}

func (m *recordingMeter) Float64ObservableCounter(
	name string,
	options ...metric.Float64ObservableCounterOption,
) (metric.Float64ObservableCounter, error) {
	config := metric.NewFloat64ObservableCounterConfig(options...)

	m.instruments = append(
		m.instruments,
		instrumentSpec{
			name:        name,
			kind:        "float64_observable_counter",
			description: config.Description(),
			unit:        config.Unit(),
		},
	)

	return m.Meter.Float64ObservableCounter(name, options...)
}

type recordingObserver struct {
	noop.Observer

	int64Values       []int64
	int64Attributes   []attribute.Set
	float64Values     []float64
	float64Attributes []attribute.Set
}

func (o *recordingObserver) ObserveInt64(
	_ metric.Int64Observable,
	value int64,
	options ...metric.ObserveOption,
) {
	o.int64Values = append(
		o.int64Values,
		value,
	)

	o.int64Attributes = append(
		o.int64Attributes,
		metric.NewObserveConfig(options).Attributes(),
	)
}

func (o *recordingObserver) ObserveFloat64(
	_ metric.Float64Observable,
	value float64,
	options ...metric.ObserveOption,
) {
	o.float64Values = append(
		o.float64Values,
		value,
	)

	o.float64Attributes = append(
		o.float64Attributes,
		metric.NewObserveConfig(options).Attributes(),
	)
}

type registrationStub struct {
	noop.Registration

	calls int
}

func (r *registrationStub) Unregister() error {
	r.calls++

	return nil
}
