package shard

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/mkbeh/xpg/cluster"
)

// Config registers one Cluster as a logical shard in a Topology.
//
// The shard ID and labels are provided by Cluster. Config is intentionally
// retained as the extension point for future topology-specific options.
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

// NewTopology validates and creates an immutable topology. Shards retain their
// registration order. Every cluster must have a non-empty and unique ID.
func NewTopology(configs []Config) (*Topology, error) {
	if len(configs) == 0 {
		return nil, errors.New("xpg/shard: topology must contain at least one shard")
	}

	shards := make([]Shard, 0, len(configs))
	indexByID := make(map[ID]int, len(configs))

	for index, config := range configs {
		resolved, err := newShard(config)
		if err != nil {
			return nil, fmt.Errorf("xpg/shard: shard %d: %w", index, err)
		}

		id := resolved.ID()

		if _, exists := indexByID[id]; exists {
			return nil, fmt.Errorf("xpg/shard: duplicate shard ID %q", id)
		}

		indexByID[id] = len(shards)
		shards = append(shards, resolved)
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

// Shard returns one shard by stable ID.
func (t *Topology) Shard(id ID) (Shard, bool) {
	return t.lookup(id)
}

func (t *Topology) lookup(id ID) (Shard, bool) {
	if t == nil {
		return Shard{}, false
	}

	index, ok := t.indexByID[id]
	if !ok {
		return Shard{}, false
	}

	return t.shards[index], true
}

// Close closes owned clusters in reverse registration order. Close is safe to
// call multiple times.
func (t *Topology) Close() {
	if t == nil {
		return
	}

	t.closeOnce.Do(func() {
		for index := len(t.shards) - 1; index >= 0; index-- {
			t.shards[index].cluster.Close()
		}
	})
}

func newShard(config Config) (Shard, error) {
	if config.Cluster == nil {
		return Shard{}, errors.New("cluster is nil")
	}

	if config.Cluster.ID() == "" {
		return Shard{}, errors.New("cluster ID must not be empty")
	}

	return Shard{
		cluster: config.Cluster,
	}, nil
}
