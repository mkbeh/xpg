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
// NewRange copies the supplied ranges into an internal representation, sorts
// them by Start, and validates that they do not overlap. The caller's slice is
// not modified.
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

		// This rejects empty, reversed, and NaN-bounded ranges.
		valid := valueRange.Start < valueRange.End
		if !valid {
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

	// sourceIndex provides deterministic ordering for equal starts and keeps
	// overlap errors tied to the caller's original slice.
	slices.SortFunc(
		entries,
		func(left, right rangeEntry[K]) int {
			if order := cmp.Compare(left.start, right.start); order != 0 {
				return order
			}

			return cmp.Compare(left.sourceIndex, right.sourceIndex)
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
//
// Resolve performs only an in-memory lookup. It does not acquire a connection
// or execute a PostgreSQL query.
func (resolver *RangeResolver[K]) Resolve(key K) (shard.Shard, error) {
	if resolver == nil || len(resolver.ranges) == 0 {
		return shard.Shard{}, shard.ErrNoShard
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
