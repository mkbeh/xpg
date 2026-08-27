package shard

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/mkbeh/xpg/topology/cluster"
)

// Topology is an immutable ordered set of logical shards.
//
// NewTopology takes ownership of all clusters only after it returns
// successfully. Close closes every owned cluster exactly once.
type Topology struct {
	shards     []Shard
	shardsByID map[ID]Shard

	closeOnce sync.Once
}

// NewTopology validates and creates an immutable topology. Shards retain the
// cluster registration order. Every cluster must have a unique, non-empty ID.
func NewTopology(clusters ...*cluster.Cluster) (*Topology, error) {
	if len(clusters) == 0 {
		return nil, errors.New("xpg/shard: topology must contain at least one shard")
	}

	shards := make([]Shard, len(clusters))
	shardsByID := make(map[ID]Shard, len(clusters))

	for index, candidate := range clusters {
		if candidate == nil {
			return nil, fmt.Errorf("xpg/shard: shard %d: cluster is nil", index)
		}

		id := candidate.ID()
		if id == "" {
			return nil, fmt.Errorf("xpg/shard: shard %d: cluster ID must not be empty", index)
		}

		if _, exists := shardsByID[id]; exists {
			return nil, fmt.Errorf("xpg/shard: duplicate shard ID %q", id)
		}

		current := Shard{cluster: candidate}
		shards[index] = current
		shardsByID[id] = current
	}

	return &Topology{
		shards:     shards,
		shardsByID: shardsByID,
	}, nil
}

// Len returns the number of registered shards.
func (t *Topology) Len() int {
	if t == nil {
		return 0
	}

	return len(t.shards)
}

// At returns the shard at index in registration order. At panics when index is
// outside the topology, matching ordinary slice indexing semantics.
func (t *Topology) At(index int) Shard {
	return t.shards[index]
}

// Shards returns a defensive copy of shards in registration order.
func (t *Topology) Shards() []Shard {
	if t == nil {
		return nil
	}

	return slices.Clone(t.shards)
}

// Shard returns one shard by stable cluster ID.
func (t *Topology) Shard(id ID) (Shard, bool) {
	if t == nil {
		return Shard{}, false
	}

	resolved, ok := t.shardsByID[id]

	return resolved, ok
}

// Close closes owned clusters in reverse registration order. Close is safe to
// call multiple times.
func (t *Topology) Close() {
	if t == nil {
		return
	}

	t.closeOnce.Do(func() {
		for _, current := range slices.Backward(t.shards) {
			current.cluster.Close()
		}
	})
}
