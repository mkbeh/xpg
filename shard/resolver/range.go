package resolver

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/mkbeh/xpg/shard"
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

// RangeResolver routes ordered keys through non-overlapping ranges.
type RangeResolver[K cmp.Ordered] struct {
	ranges []rangeEntry[K]
}

// NewRange creates a resolver from non-overlapping half-open ranges.
//
// The supplied ranges may be unordered. NewRange copies and sorts them by Start,
// validates their boundaries and overlap, and leaves the caller's slice unchanged.
func NewRange[K cmp.Ordered](topology *shard.Topology, ranges []Range[K]) (*RangeResolver[K], error) {
	if err := requireTopology(topology); err != nil {
		return nil, err
	}

	if len(ranges) == 0 {
		return nil, errors.New("xpg/shard/resolver: range resolver requires at least one range")
	}

	entries := make([]rangeEntry[K], len(ranges))

	for index, valueRange := range ranges {
		if err := requireShardID(valueRange.ShardID); err != nil {
			return nil, fmt.Errorf("xpg/shard/resolver: range %d: %w", index, err)
		}

		// Using < intentionally rejects empty and reversed ranges as well as
		// ranges with NaN boundaries for floating-point key types.
		if !(valueRange.Start < valueRange.End) { //nolint:staticcheck // Negated comparison intentionally rejects NaN boundaries.
			return nil, fmt.Errorf("xpg/shard/resolver: range %d must satisfy start < end", index)
		}

		resolved, ok := topology.Shard(valueRange.ShardID)
		if !ok {
			return nil, fmt.Errorf(
				"xpg/shard/resolver: range %d: %w",
				index,
				&shard.UnknownShardError{
					ShardID: valueRange.ShardID,
				},
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

	// Once ranges are sorted by Start, checking adjacent entries is sufficient
	// to detect every overlap.
	for index := 1; index < len(entries); index++ {
		previous := entries[index-1]
		current := entries[index]

		// Adjacent half-open ranges are valid:
		//
		// [0, 100) and [100, 200)
		if previous.end <= current.start {
			continue
		}

		return nil, fmt.Errorf(
			"xpg/shard/resolver: ranges %d and %d overlap",
			previous.sourceIndex,
			current.sourceIndex,
		)
	}

	return &RangeResolver[K]{
		ranges: entries,
	}, nil
}

// Resolve returns the shard whose configured range contains key.
func (resolver *RangeResolver[K]) Resolve(key K) (shard.Shard, error) {
	if resolver == nil || len(resolver.ranges) == 0 {
		return shard.Shard{}, errors.New("xpg/shard/resolver: range resolver is not initialized")
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

type rangeEntry[K cmp.Ordered] struct {
	start K
	end   K
	shard shard.Shard

	sourceIndex int
}
