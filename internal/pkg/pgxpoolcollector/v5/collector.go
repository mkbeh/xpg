// Package v5 provides prometheus plug-in metrics for a pgx client.
//
// This package tracks the following metrics under the following names:
//
//	#ns_postgres_acquire_count{}
//	#ns_postgres_acquire_duration{}
//	#ns_postgres_acquired_conns{}
//	#ns_postgres_canceled_acquire_count{}
//	#ns_postgres_constructing_conns{}
//	#ns_postgres_empty_acquire_count{}
//	#ns_postgres_idle_conns{}
//	#ns_postgres_max_conns{}
//	#ns_postgres_total_conns{}
//
// Labels list:
//	client_id=#{client_id}
//	client_kind=#{master/replica}
//	db=#{db}
//	shard_id=#{shard_id}

package v5

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// StatsGetter is an interface that gets sql.DBStats.
// It's implemented by e.g. *sql.DB or *sqlx.DB.
type StatsGetter interface {
	Stat() *pgxpool.Stat
}

// StatsCollector implements the prometheus.Collector interface.
type StatsCollector struct {
	sg StatsGetter

	// descriptions of exported metrics
	acquireCount         *prometheus.Desc
	acquireDuration      *prometheus.Desc
	acquiredConns        *prometheus.Desc
	canceledAcquireCount *prometheus.Desc
	constructingConns    *prometheus.Desc
	emptyAcquireCount    *prometheus.Desc
	idleConns            *prometheus.Desc
	maxConns             *prometheus.Desc
	totalConns           *prometheus.Desc
}

// NewStatsCollector creates a new StatsCollector.
func NewStatsCollector(namespace, subsystem string, constLabels prometheus.Labels, sg StatsGetter) *StatsCollector {
	return &StatsCollector{
		sg: sg,
		acquireCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "acquire_count"),
			"Cumulative count of successful acquires from the pool.",
			nil,
			constLabels,
		),
		acquireDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "acquire_duration"),
			"Total duration of all successful acquires from the pool.",
			nil,
			constLabels,
		),
		acquiredConns: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "acquired_conns"),
			"Number of currently acquired connections in the pool.",
			nil,
			constLabels,
		),
		canceledAcquireCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "canceled_acquire_count"),
			"Cumulative count of acquires from the pool that were canceled by a context.",
			nil,
			constLabels,
		),
		constructingConns: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "constructing_conns"),
			"Number of conns with construction in progress in the pool.",
			nil,
			constLabels,
		),
		emptyAcquireCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "empty_acquire_count"),
			"Cumulative count of successful acquires from the pool that waited for a resource to be released or constructed because the pool was empty.",
			nil,
			constLabels,
		),
		idleConns: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "idle_conns"),
			"Number of currently idle conns in the pool.",
			nil,
			constLabels,
		),
		maxConns: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "max_conns"),
			"Maximum size of the pool.",
			nil,
			constLabels,
		),
		totalConns: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "total_conns"),
			"Total number of resources currently in the pool. The value is the sum of ConstructingConns, AcquiredConns, and IdleConns.",
			nil,
			constLabels,
		),
	}
}

// Describe implements the prometheus.Collector interface.
func (c StatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.acquireCount
	ch <- c.acquireDuration
	ch <- c.acquiredConns
	ch <- c.canceledAcquireCount
	ch <- c.constructingConns
	ch <- c.emptyAcquireCount
	ch <- c.idleConns
	ch <- c.maxConns
	ch <- c.totalConns
}

// Collect implements the prometheus.Collector interface.
func (c StatsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.sg.Stat()

	ch <- prometheus.MustNewConstMetric(
		c.acquireCount,
		prometheus.GaugeValue,
		float64(stats.AcquireCount()),
	)

	ch <- prometheus.MustNewConstMetric(
		c.acquireDuration,
		prometheus.GaugeValue,
		float64(stats.AcquireDuration()),
	)

	ch <- prometheus.MustNewConstMetric(
		c.acquiredConns,
		prometheus.GaugeValue,
		float64(stats.AcquiredConns()),
	)

	ch <- prometheus.MustNewConstMetric(
		c.canceledAcquireCount,
		prometheus.GaugeValue,
		float64(stats.CanceledAcquireCount()),
	)

	ch <- prometheus.MustNewConstMetric(
		c.constructingConns,
		prometheus.GaugeValue,
		float64(stats.ConstructingConns()),
	)

	ch <- prometheus.MustNewConstMetric(
		c.emptyAcquireCount,
		prometheus.GaugeValue,
		float64(stats.EmptyAcquireCount()),
	)

	ch <- prometheus.MustNewConstMetric(
		c.idleConns,
		prometheus.GaugeValue,
		float64(stats.IdleConns()),
	)

	ch <- prometheus.MustNewConstMetric(
		c.maxConns,
		prometheus.GaugeValue,
		float64(stats.MaxConns()),
	)

	ch <- prometheus.MustNewConstMetric(
		c.totalConns,
		prometheus.GaugeValue,
		float64(stats.TotalConns()),
	)
}
