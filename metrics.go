package xpg

// PoolMetrics registers metrics for a Pool.
//
// Implementations are expected to be immutable and safe to reuse for multiple
// pools. Register is called after the underlying pgxpool.Pool has been created.
type PoolMetrics interface {
	Register(pool *Pool) (PoolMetricsRegistration, error)
}

// PoolMetricsRegistration owns a metrics registration associated with one
// Pool. Close is called once before the underlying pgxpool.Pool is closed.
type PoolMetricsRegistration interface {
	Close()
}
