// Package resolver provides routing strategies for shard.Topology.
//
// Resolvers are bound to an immutable topology and return shard.Shard values.
// The package provides rendezvous hashing, ordered numeric or string ranges,
// time ranges, and an adapter for custom routing functions.
//
// Resolvers borrow their topology and must not outlive it.
package resolver
