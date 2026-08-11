package shard

// Resolver maps a typed application key to one shard.
//
// Implementations shared by concurrent callers must be concurrency-safe.
type Resolver[K any] interface {
	Resolve(key K) (Shard, error)
}
