// Package shard provides explicit application-level routing across PostgreSQL
// clusters.
//
// A Topology owns an ordered set of logical shards. Every Shard is backed by a
// cluster.Cluster, and typed resolvers map application keys directly to shards.
// Built-in resolvers support rendezvous hashing, ordered ranges, and time
// ranges; applications may also provide custom routing logic. Resolvers borrow
// their topology and do not own its clusters.
//
// The package also provides shard grouping, colocation checks, and bounded
// fan-out.
//
// The package does not inspect SQL, hide shard keys in contexts, move data,
// replicate reference tables, or provide distributed transactions.
package shard
