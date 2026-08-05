package xpg

import "time"

// PoolStats is a detached point-in-time snapshot of connection pool statistics.
type PoolStats struct {
	// Current state.

	// AcquiredConns is the number of connections currently checked out from the
	// pool.
	AcquiredConns int32

	// ConstructingConns is the number of connections currently being created.
	ConstructingConns int32

	// IdleConns is the number of currently idle connections.
	IdleConns int32

	// MaxConns is the maximum number of connections allowed by the pool.
	MaxConns int32

	// TotalConns is the number of acquired, idle, and constructing connections.
	TotalConns int32

	// Acquire lifecycle.

	// AcquireCount is the cumulative number of successful connection acquires.
	AcquireCount int64

	// AcquireDuration is the cumulative duration of successful connection
	// acquires.
	AcquireDuration time.Duration

	// CanceledAcquireCount is the cumulative number of connection acquires
	// canceled by context cancellation.
	CanceledAcquireCount int64

	// EmptyAcquireCount is the cumulative number of successful acquires that
	// waited because the pool was empty.
	EmptyAcquireCount int64

	// EmptyAcquireWaitTime is the cumulative time spent waiting on successful
	// acquires while the pool was empty.
	EmptyAcquireWaitTime time.Duration

	// Connection lifecycle.

	// NewConnsCount is the cumulative number of connections opened by the pool.
	NewConnsCount int64

	// MaxIdleDestroyCount is the cumulative number of connections closed after
	// exceeding MaxConnIdleTime.
	MaxIdleDestroyCount int64

	// MaxLifetimeDestroyCount is the cumulative number of connections closed
	// after exceeding MaxConnLifetime.
	MaxLifetimeDestroyCount int64
}

// Stats returns a detached snapshot of the current pool statistics.
func (p *Pool) Stats() PoolStats {
	stats := p.pool.Stat()

	return PoolStats{
		AcquiredConns:           stats.AcquiredConns(),
		ConstructingConns:       stats.ConstructingConns(),
		IdleConns:               stats.IdleConns(),
		MaxConns:                stats.MaxConns(),
		TotalConns:              stats.TotalConns(),
		AcquireCount:            stats.AcquireCount(),
		AcquireDuration:         stats.AcquireDuration(),
		CanceledAcquireCount:    stats.CanceledAcquireCount(),
		EmptyAcquireCount:       stats.EmptyAcquireCount(),
		EmptyAcquireWaitTime:    stats.EmptyAcquireWaitTime(),
		NewConnsCount:           stats.NewConnsCount(),
		MaxIdleDestroyCount:     stats.MaxIdleDestroyCount(),
		MaxLifetimeDestroyCount: stats.MaxLifetimeDestroyCount(),
	}
}
