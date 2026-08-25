package shard

// Resolver maps a typed application key to one shard.
//
// Resolve should return ErrNoShard when the key cannot be mapped to a shard.
// Implementations must be safe for concurrent use.
type Resolver[K any] interface {
	Resolve(key K) (Shard, error)
}
