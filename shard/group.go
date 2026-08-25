package shard

import (
	"errors"
	"fmt"
)

// SameShard resolves keys and verifies that they all belong to the same shard.
//
// It returns ErrNoShard when no keys are provided and MismatchError when a key
// resolves to a different shard.
func SameShard[K any](resolver Resolver[K], keys ...K) (Shard, error) {
	if resolver == nil {
		return Shard{}, errors.New("xpg/shard: resolver is nil")
	}

	if len(keys) == 0 {
		return Shard{}, ErrNoShard
	}

	expected, err := resolver.Resolve(keys[0])
	if err != nil {
		return Shard{}, fmt.Errorf("xpg/shard: resolve key 0: %w", err)
	}

	expectedID := expected.ID()

	for index := 1; index < len(keys); index++ {
		actual, err := resolver.Resolve(keys[index])
		if err != nil {
			return Shard{}, fmt.Errorf("xpg/shard: resolve key %d: %w", index, err)
		}

		actualID := actual.ID()
		if actualID == expectedID {
			continue
		}

		return Shard{}, &MismatchError{
			Expected: expectedID,
			Actual:   actualID,
			Index:    index,
		}
	}

	return expected, nil
}

// Group contains keys that resolve to the same shard.
// Keys preserve their original relative order.
type Group[K any] struct {
	Shard Shard
	Keys  []K
}

// GroupByShard resolves each key once and groups keys by shard.
//
// Groups are returned in order of each shard's first appearance in keys.
// Keys within each group preserve their original relative order.
func GroupByShard[K any](resolver Resolver[K], keys []K) ([]Group[K], error) {
	if resolver == nil {
		return nil, errors.New("xpg/shard: resolver is nil")
	}

	groups := make([]Group[K], 0)
	indexByID := make(map[ID]int)

	for keyIndex, key := range keys {
		resolved, err := resolver.Resolve(key)
		if err != nil {
			return nil, fmt.Errorf("xpg/shard: resolve key %d: %w", keyIndex, err)
		}

		id := resolved.ID()

		groupIndex, exists := indexByID[id]
		if !exists {
			groupIndex = len(groups)
			indexByID[id] = groupIndex

			groups = append(groups, Group[K]{
				Shard: resolved,
			})
		}

		groups[groupIndex].Keys = append(groups[groupIndex].Keys, key)
	}

	return groups, nil
}
