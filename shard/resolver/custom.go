package resolver

import (
	"errors"

	"github.com/mkbeh/xpg/shard"
)

// ResolveFunc maps an application key to a shard ID within topology.
//
// Implementations shared by concurrent callers must be deterministic and
// concurrency-safe. They should not perform hidden I/O or modify topology.
type ResolveFunc[K any] func(key K, topology *shard.Topology) (shard.ID, error)

// CustomResolver adapts ResolveFunc to shard.Resolver.
type CustomResolver[K any] struct {
	topology *shard.Topology
	resolve  ResolveFunc[K]
}

// NewCustom binds custom routing logic to one immutable topology.
func NewCustom[K any](topology *shard.Topology, resolve ResolveFunc[K]) (*CustomResolver[K], error) {
	if err := requireTopology(topology); err != nil {
		return nil, err
	}

	if resolve == nil {
		return nil, errors.New("xpg/shard/resolver: custom resolve function is nil")
	}

	return &CustomResolver[K]{
		topology: topology,
		resolve:  resolve,
	}, nil
}

// Resolve maps key to a shard and rejects IDs absent from the bound topology.
func (resolver *CustomResolver[K]) Resolve(key K) (shard.Shard, error) {
	if resolver == nil ||
		resolver.topology == nil ||
		resolver.resolve == nil {
		return shard.Shard{}, errors.New(
			"xpg/shard/resolver: custom resolver is not initialized",
		)
	}

	id, err := resolver.resolve(key, resolver.topology)
	if err != nil {
		return shard.Shard{}, err
	}

	resolved, ok := resolver.topology.Shard(id)
	if !ok {
		return shard.Shard{}, &shard.UnknownShardError{
			ShardID: id,
		}
	}

	return resolved, nil
}
