// Package resolver provides routing strategies for shard.Topology.
//
// Resolvers map application keys to shards using rendezvous hashing, ordered
// ranges, time ranges, or custom routing logic.
//
// Resolvers borrow their topology and must not outlive it.
package resolver
