package shard

import (
	"errors"
	"fmt"
)

// SameShard resolves the keys and verifies that they all belong to the same
// shard. It returns that shard when all keys are colocated.
func SameShard[K any](resolver Resolver[K], keys ...K) (Shard, error) {
	if resolver == nil {
		return Shard{}, errors.New("xpg/topology/shard: resolver is nil")
	}

	if len(keys) == 0 {
		return Shard{}, ErrNoShard
	}

	expected, err := resolver.Resolve(keys[0])
	if err != nil {
		return Shard{}, fmt.Errorf("xpg/topology/shard: resolve key 0: %w", err)
	}

	expectedID := expected.ID()

	for index, key := range keys[1:] {
		actual, err := resolver.Resolve(key)
		if err != nil {
			return Shard{}, fmt.Errorf("xpg/topology/shard: resolve key %d: %w", index+1, err)
		}

		if actualID := actual.ID(); actualID != expectedID {
			return Shard{}, &MismatchError{
				Expected: expectedID,
				Actual:   actualID,
				Index:    index + 1,
			}
		}
	}

	return expected, nil
}

// Group contains input keys that resolve to one shard. Keys preserve their
// original relative order.
type Group[K any] struct {
	Shard Shard
	Keys  []K
}

// GroupByShard resolves every key once and returns groups in order of each
// shard's first appearance in the input.
func GroupByShard[K any](resolver Resolver[K], keys []K) ([]Group[K], error) {
	if resolver == nil {
		return nil, errors.New("xpg/topology/shard: resolver is nil")
	}

	groups := make([]Group[K], 0)

	var indexByID map[ID]int

	for keyIndex, key := range keys {
		resolved, err := resolver.Resolve(key)
		if err != nil {
			return nil, fmt.Errorf("xpg/topology/shard: resolve key %d: %w", keyIndex, err)
		}

		if indexByID == nil {
			indexByID = make(map[ID]int)
		}

		groups = addKeyToGroup(groups, indexByID, resolved, key)
	}

	return groups, nil
}

// Partition contains keys grouped by resolved shard together with keys that
// could not be mapped to any shard. Group and key order follow GroupByShard;
// unresolved keys preserve their original relative order.
type Partition[K any] struct {
	Groups     []Group[K]
	Unresolved []K
}

// PartitionByShard resolves every key once. Keys for which Resolve returns
// ErrNoShard are collected in Unresolved. Any other resolver error aborts the
// operation and returns a zero Partition.
func PartitionByShard[K any](resolver Resolver[K], keys []K) (Partition[K], error) {
	if resolver == nil {
		return Partition[K]{}, errors.New("xpg/topology/shard: resolver is nil")
	}

	if len(keys) == 0 {
		return Partition[K]{}, nil
	}

	var (
		partition Partition[K]
		indexByID map[ID]int
	)

	for keyIndex, key := range keys {
		resolved, err := resolver.Resolve(key)
		if err != nil {
			if errors.Is(err, ErrNoShard) {
				partition.Unresolved = append(partition.Unresolved, key)
				continue
			}

			return Partition[K]{}, fmt.Errorf("xpg/topology/shard: resolve key %d: %w", keyIndex, err)
		}

		if indexByID == nil {
			indexByID = make(map[ID]int)
		}

		partition.Groups = addKeyToGroup(partition.Groups, indexByID, resolved, key)
	}

	return partition, nil
}

func addKeyToGroup[K any](
	groups []Group[K],
	indexByID map[ID]int,
	target Shard,
	key K,
) []Group[K] {
	id := target.ID()

	idx, ok := indexByID[id]
	if !ok {
		idx = len(groups)
		indexByID[id] = idx
		groups = append(groups, Group[K]{Shard: target})
	}

	groups[idx].Keys = append(groups[idx].Keys, key)

	return groups
}
