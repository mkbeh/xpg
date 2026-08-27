package resolver

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/mkbeh/xpg/topology/shard"
)

// Range maps the bounded half-open interval [Start, End) to one shard.
//
// Start must be less than End. Ranges may be supplied in any order.
// Gaps are allowed and resolve to shard.ErrNoShard.
type Range[K cmp.Ordered] struct {
	Start   K
	End     K
	ShardID shard.ID
}

// RangeResolver resolves ordered keys through bounded, non-overlapping ranges.
type RangeResolver[K cmp.Ordered] struct {
	ranges []rangeEntry[K]
}

type rangeEntry[K cmp.Ordered] struct {
	start K
	end   K
	shard shard.Shard

	sourceIndex int
}

// NewRange creates a resolver from bounded, non-overlapping half-open ranges.
//
// NewRange resolves every ShardID to its immutable Shard handle once, copies
// the routing data into an internal representation, sorts it by Start, and
// validates that ranges do not overlap. The caller's slice is not modified.
func NewRange[K cmp.Ordered](topology *shard.Topology, ranges []Range[K]) (*RangeResolver[K], error) {
	if err := requireTopology(topology); err != nil {
		return nil, err
	}

	if len(ranges) == 0 {
		return nil, errors.New("xpg/topology/shard/resolver: range resolver requires at least one range")
	}

	entries := make([]rangeEntry[K], len(ranges))

	for index, valueRange := range ranges {
		if err := requireShardID(valueRange.ShardID); err != nil {
			return nil, fmt.Errorf("xpg/topology/shard/resolver: range %d: %w", index, err)
		}

		if !(valueRange.Start < valueRange.End) { //nolint:staticcheck // Negated comparison intentionally rejects NaN boundaries.
			return nil, fmt.Errorf("xpg/topology/shard/resolver: range %d must satisfy start < end", index)
		}

		resolved, ok := topology.Shard(valueRange.ShardID)
		if !ok {
			return nil, fmt.Errorf(
				"xpg/topology/shard/resolver: range %d: %w",
				index,
				&shard.UnknownShardError{ShardID: valueRange.ShardID},
			)
		}

		entries[index] = rangeEntry[K]{
			start:       valueRange.Start,
			end:         valueRange.End,
			shard:       resolved,
			sourceIndex: index,
		}
	}

	slices.SortStableFunc(
		entries,
		func(left, right rangeEntry[K]) int {
			return cmp.Compare(left.start, right.start)
		},
	)

	for index := 1; index < len(entries); index++ {
		previous := entries[index-1]
		current := entries[index]

		if previous.end <= current.start {
			continue
		}

		return nil, fmt.Errorf(
			"xpg/topology/shard/resolver: ranges %d and %d overlap",
			previous.sourceIndex,
			current.sourceIndex,
		)
	}

	return &RangeResolver[K]{
		ranges: entries,
	}, nil
}

// Resolve returns the shard whose range contains key.
//
// Resolve performs only an in-memory lookup. It does not consult topology,
// acquire a connection, or execute a PostgreSQL query.
func (resolver *RangeResolver[K]) Resolve(key K) (shard.Shard, error) {
	if resolver == nil || len(resolver.ranges) == 0 {
		return shard.Shard{}, errors.New("xpg/topology/shard/resolver: range resolver is not initialized")
	}

	// Non-overlap validation guarantees strictly increasing upper boundaries,
	// making this search predicate monotonic.
	index := sort.Search(
		len(resolver.ranges),
		func(index int) bool {
			return key < resolver.ranges[index].end
		},
	)

	if index == len(resolver.ranges) {
		return shard.Shard{}, shard.ErrNoShard
	}

	entry := resolver.ranges[index]

	// The first range ending after key may still start after it when there is a
	// gap between configured ranges.
	if key < entry.start {
		return shard.Shard{}, shard.ErrNoShard
	}

	return entry.shard, nil
}
