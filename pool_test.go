package xpg

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewRejectsNilConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config *pgxpool.Config
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name:   "nil connection config",
			config: &pgxpool.Config{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := New(
				t.Context(),
				test.config,
			)

			assertErrorMessage(
				t,
				err,
				"xpg: pool config is nil",
			)
		})
	}
}

func TestPoolMetadata(t *testing.T) {
	t.Parallel()

	config := newPoolTestConfig(t)

	labels := map[string]string{
		"region": "eu",
		"role":   "reader",
	}

	labelsOption := WithLabels(labels)
	labels["region"] = "changed"

	pool, err := New(
		t.Context(),
		config,
		WithName(" orders "),
		labelsOption,
		WithLabel("role", "primary"),
	)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if got, want := pool.Name(), "orders"; got != want {
		t.Fatalf(
			"Name() = %q, want %q",
			got,
			want,
		)
	}

	gotLabels := pool.Labels()

	if got, want := gotLabels["region"], "eu"; got != want {
		t.Fatalf(
			"region label = %q, want %q",
			got,
			want,
		)
	}

	if got, want := gotLabels["role"], "primary"; got != want {
		t.Fatalf(
			"role label = %q, want %q",
			got,
			want,
		)
	}

	gotLabels["region"] = "mutated"

	if got, want := pool.Labels()["region"], "eu"; got != want {
		t.Fatalf(
			"region label after mutation = %q, want %q",
			got,
			want,
		)
	}

	if pool.Raw() == nil {
		t.Fatal("Raw returned nil")
	}
}

func TestPoolDerivedName(t *testing.T) {
	t.Parallel()

	pool, err := New(
		t.Context(),
		newPoolTestConfig(t),
	)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if got, want := pool.Name(), "localhost:5432/testdb"; got != want {
		t.Fatalf(
			"Name() = %q, want %q",
			got,
			want,
		)
	}
}

func TestPoolStats(t *testing.T) {
	t.Parallel()

	config := newPoolTestConfig(t)
	config.MaxConns = 17

	pool, err := New(
		t.Context(),
		config,
	)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	stats := pool.Stats()

	if got, want := stats.MaxConns, int32(17); got != want {
		t.Fatalf(
			"MaxConns = %d, want %d",
			got,
			want,
		)
	}

	if stats.TotalConns != 0 {
		t.Fatalf(
			"TotalConns = %d, want 0",
			stats.TotalConns,
		)
	}
}

func TestPoolMetricsLifecycle(t *testing.T) {
	t.Parallel()

	registration := &poolTestMetricsRegistration{}
	metrics := &poolTestMetrics{
		registration: registration,
	}

	pool, err := New(
		t.Context(),
		newPoolTestConfig(t),
		WithMetrics(metrics),
	)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	if metrics.registerCalls != 1 {
		t.Fatalf(
			"register calls = %d, want 1",
			metrics.registerCalls,
		)
	}

	if metrics.pool != pool {
		t.Fatal("metrics registered with unexpected pool")
	}

	pool.Close()
	pool.Close()

	if registration.closeCalls != 1 {
		t.Fatalf(
			"registration close calls = %d, want 1",
			registration.closeCalls,
		)
	}
}

func TestNewPreservesMetricsRegistrationError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("register failed")
	metrics := &poolTestMetrics{
		err: expectedErr,
	}

	pool, err := New(
		t.Context(),
		newPoolTestConfig(t),
		WithMetrics(metrics),
	)

	if pool != nil {
		t.Fatal("expected nil pool")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf("original error was not preserved: %v", err)
	}

	if metrics.registerCalls != 1 {
		t.Fatalf(
			"register calls = %d, want 1",
			metrics.registerCalls,
		)
	}
}

func newPoolTestConfig(t *testing.T) *pgxpool.Config {
	t.Helper()

	config, err := pgxpool.ParseConfig(
		"postgres://user:password@localhost:5432/testdb?sslmode=disable",
	)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}

	return config
}

type poolTestMetrics struct {
	registration MetricsRegistration
	err          error

	pool          *Pool
	registerCalls int
}

func (m *poolTestMetrics) Register(
	pool *Pool,
) (MetricsRegistration, error) {
	m.registerCalls++
	m.pool = pool

	if m.err != nil {
		return nil, m.err
	}

	return m.registration, nil
}

type poolTestMetricsRegistration struct {
	closeCalls int
}

func (r *poolTestMetricsRegistration) Close() {
	r.closeCalls++
}
