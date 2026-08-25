package xpg

// Metrics registers metrics for a Pool.
//
// Implementations must be safe to reuse across multiple pools. Register is
// called after the underlying pgxpool.Pool has been created.
type Metrics interface {
	Register(pool *Pool) (MetricsRegistration, error)
}

// MetricsRegistration represents a metrics registration for one Pool.
//
// Close is called once before the underlying pgxpool.Pool is closed.
type MetricsRegistration interface {
	Close()
}
