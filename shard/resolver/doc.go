// Package resolver provides routing strategies for shard.Topology.
//
// Resolvers are bound to an immutable topology and map application keys to
// shard.Shard values. The package provides rendezvous hashing, ordered ranges,
// time ranges, and an adapter for custom routing functions.
//
// Resolvers borrow their topology and must not outlive it.
package resolver
