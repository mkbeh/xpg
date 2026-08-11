package resolver

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/mkbeh/xpg/shard"
)

// TimeRange maps the bounded half-open interval [Start, End) to one shard.
//
// Start must be before End. Ranges may be supplied in any order.
// Gaps are allowed and resolve to shard.ErrNoShard.
type TimeRange struct {
	Start   time.Time
	End     time.Time
	ShardID shard.ID
}

// TimeRangeResolver resolves time instants through bounded, non-overlapping
// ranges.
type TimeRangeResolver struct {
	ranges []timeRangeEntry
}

type timeRangeEntry struct {
	start time.Time
	end   time.Time
	shard shard.Shard

	sourceIndex int
}

// NewTimeRange creates a resolver from bounded, non-overlapping half-open time
// ranges.
//
// Range boundaries are normalized to UTC. The caller's slice and time values
// are not modified.
func NewTimeRange(topology *shard.Topology, ranges []TimeRange) (*TimeRangeResolver, error) {
	if err := requireTopology(topology); err != nil {
		return nil, err
	}

	if len(ranges) == 0 {
		return nil, errors.New("xpg/shard/resolver: time range resolver requires at least one range")
	}

	entries := make([]timeRangeEntry, len(ranges))

	for index, valueRange := range ranges {
		if err := requireShardID(valueRange.ShardID); err != nil {
			return nil, fmt.Errorf("xpg/shard/resolver: time range %d: %w", index, err)
		}

		start := normalizeTime(valueRange.Start)
		end := normalizeTime(valueRange.End)

		if !start.Before(end) {
			return nil, fmt.Errorf("xpg/shard/resolver: time range %d must satisfy start < end", index)
		}

		resolved, ok := topology.Shard(valueRange.ShardID)
		if !ok {
			return nil, fmt.Errorf(
				"xpg/shard/resolver: time range %d: %w",
				index,
				&shard.UnknownShardError{
					ShardID: valueRange.ShardID,
				},
			)
		}

		entries[index] = timeRangeEntry{
			start:       start,
			end:         end,
			shard:       resolved,
			sourceIndex: index,
		}
	}

	// sourceIndex keeps overlap diagnostics tied to the caller's original
	// slice and provides deterministic ordering for equal starts.
	slices.SortFunc(
		entries,
		func(left, right timeRangeEntry) int {
			if order := left.start.Compare(right.start); order != 0 {
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
		// [00:00, 01:00) and [01:00, 02:00)
		if previous.end.After(current.start) {
			return nil, fmt.Errorf(
				"xpg/shard/resolver: time ranges %d and %d overlap",
				previous.sourceIndex,
				current.sourceIndex,
			)
		}
	}

	return &TimeRangeResolver{
		ranges: entries,
	}, nil
}

// Resolve returns the shard whose configured time range contains key.
//
// Resolve performs only an in-memory lookup. It does not acquire a connection
// or execute a PostgreSQL query.
func (resolver *TimeRangeResolver) Resolve(key time.Time) (shard.Shard, error) {
	if resolver == nil || len(resolver.ranges) == 0 {
		return shard.Shard{}, shard.ErrNoShard
	}

	key = normalizeTime(key)

	// Non-overlap validation guarantees strictly increasing upper boundaries,
	// making this search predicate monotonic.
	index := sort.Search(
		len(resolver.ranges),
		func(index int) bool {
			return key.Before(resolver.ranges[index].end)
		},
	)

	if index == len(resolver.ranges) {
		return shard.Shard{}, shard.ErrNoShard
	}

	entry := resolver.ranges[index]

	// The first range ending after key may still start after it when there is a
	// gap between configured ranges.
	if key.Before(entry.start) {
		return shard.Shard{}, shard.ErrNoShard
	}

	return entry.shard, nil
}

func normalizeTime(value time.Time) time.Time {
	return value.UTC()
}
