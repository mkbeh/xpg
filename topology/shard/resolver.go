package shard

// Resolver maps a typed application key to the shard owning that key.
//
// Resolve should return ErrNoShard when the key cannot be mapped to a shard.
// When Resolve returns nil error, it must return a valid Shard. Implementations
// shared by concurrent callers must be concurrency-safe.
type Resolver[K any] interface {
	Resolve(key K) (Shard, error)
}
