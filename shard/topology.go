package shard

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/mkbeh/xpg/cluster"
)

// Config registers one Cluster as a logical shard.
//
// The shard ID and labels are provided by Cluster.
type Config struct {
	Cluster *cluster.Cluster
}

// Topology is an immutable ordered set of logical shards and their PostgreSQL
// clusters.
//
// NewTopology takes ownership of all configured clusters only after it returns
// successfully. Close closes every owned cluster exactly once.
type Topology struct {
	shards    []Shard
	indexByID map[ID]int

	closeOnce sync.Once
}

// NewTopology validates and creates an immutable topology.
//
// Shards retain their registration order. Every cluster must have a non-empty
// and unique ID.
func NewTopology(configs []Config) (*Topology, error) {
	if len(configs) == 0 {
		return nil, errors.New(
			"xpg/shard: topology must contain at least one shard",
		)
	}

	shards := make([]Shard, len(configs))
	indexByID := make(map[ID]int, len(configs))

	for index, config := range configs {
		if config.Cluster == nil {
			return nil, fmt.Errorf(
				"xpg/shard: shard %d: cluster is nil",
				index,
			)
		}

		id := config.Cluster.ID()
		if id == "" {
			return nil, fmt.Errorf(
				"xpg/shard: shard %d: cluster ID must not be empty",
				index,
			)
		}

		if previousIndex, exists := indexByID[id]; exists {
			return nil, fmt.Errorf(
				"xpg/shard: duplicate shard ID %q at indexes %d and %d",
				id,
				previousIndex,
				index,
			)
		}

		shards[index] = Shard{
			cluster: config.Cluster,
		}
		indexByID[id] = index
	}

	return &Topology{
		shards:    shards,
		indexByID: indexByID,
	}, nil
}

// Len returns the number of registered shards.
func (t *Topology) Len() int {
	if t == nil {
		return 0
	}

	return len(t.shards)
}

// At returns the shard at index in registration order.
//
// At panics when t is nil or index is out of range.
func (t *Topology) At(index int) Shard {
	return t.shards[index]
}

// Shard returns the shard with id.
func (t *Topology) Shard(id ID) (Shard, bool) {
	if t == nil {
		return Shard{}, false
	}

	index, ok := t.indexByID[id]
	if !ok {
		return Shard{}, false
	}

	return t.shards[index], true
}

// Shards returns a defensive copy of shards in registration order.
func (t *Topology) Shards() []Shard {
	if t == nil {
		return nil
	}

	return slices.Clone(t.shards)
}

// Close closes owned clusters in reverse registration order. Close is safe to
// call multiple times.
func (t *Topology) Close() {
	if t == nil {
		return
	}

	t.closeOnce.Do(func() {
		for _, shard := range slices.Backward(t.shards) {
			shard.cluster.Close()
		}
	})
}
