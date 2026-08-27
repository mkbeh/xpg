// Package shard provides application-level routing across PostgreSQL clusters.
//
// A Topology owns an immutable set of logical shards backed by cluster.Cluster
// values. Resolvers map application keys to shards using rendezvous hashing,
// ordered ranges, time ranges, or custom routing logic.
//
// The package also provides shard grouping, colocation checks, and bounded
// parallel operations across shards.
package shard
