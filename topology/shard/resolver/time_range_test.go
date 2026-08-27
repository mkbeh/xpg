package resolver

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mkbeh/xpg/topology/shard"
)

func TestNewTimeRangeValidatesArguments(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	tests := []struct {
		name      string
		topology  *shard.Topology
		ranges    []TimeRange
		wantError string
	}{
		{
			name: "nil topology",
			ranges: []TimeRange{
				{Start: start, End: end, ShardID: "shard-a"},
			},
			wantError: "xpg/topology/shard/resolver: topology is nil or empty",
		},
		{
			name:      "empty ranges",
			topology:  topology,
			wantError: "xpg/topology/shard/resolver: time range resolver requires at least one range",
		},
		{
			name:     "empty shard ID",
			topology: topology,
			ranges: []TimeRange{
				{Start: start, End: end},
			},
			wantError: "xpg/topology/shard/resolver: time range 0: shard ID must not be empty",
		},
		{
			name:     "empty interval",
			topology: topology,
			ranges: []TimeRange{
				{Start: start, End: start, ShardID: "shard-a"},
			},
			wantError: "xpg/topology/shard/resolver: time range 0 must satisfy start < end",
		},
		{
			name:     "reversed interval",
			topology: topology,
			ranges: []TimeRange{
				{Start: end, End: start, ShardID: "shard-a"},
			},
			wantError: "xpg/topology/shard/resolver: time range 0 must satisfy start < end",
		},
		{
			name:     "unknown shard",
			topology: topology,
			ranges: []TimeRange{
				{Start: start, End: end, ShardID: "missing"},
			},
			wantError: `xpg/topology/shard/resolver: time range 0: xpg/topology/shard: unknown shard "missing"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewTimeRange(
				test.topology,
				test.ranges,
			)
			if err == nil {
				t.Fatal("expected error")
			}

			if got := err.Error(); got != test.wantError {
				t.Fatalf("error = %q, want %q", got, test.wantError)
			}
		})
	}
}

func TestNewTimeRangeRejectsOverlapUsingSourceIndexes(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	_, err := NewTimeRange(topology, []TimeRange{
		{Start: base.Add(2 * time.Hour), End: base.Add(4 * time.Hour), ShardID: "shard-b"},
		{Start: base.Add(time.Hour), End: base.Add(3 * time.Hour), ShardID: "shard-a"},
	})
	if err == nil {
		t.Fatal("expected overlap error")
	}

	if got, want := err.Error(), "xpg/topology/shard/resolver: time ranges 1 and 0 overlap"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestNewTimeRangeDoesNotModifyInput(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	location := time.FixedZone("UTC+3", 3*60*60)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, location)
	ranges := []TimeRange{
		{Start: base.Add(time.Hour), End: base.Add(2 * time.Hour), ShardID: "shard-b"},
		{Start: base, End: base.Add(time.Hour), ShardID: "shard-a"},
	}
	want := slices.Clone(ranges)

	if _, err := NewTimeRange(topology, ranges); err != nil {
		t.Fatalf("NewTimeRange() error = %v", err)
	}

	if !slices.Equal(ranges, want) {
		t.Fatalf("ranges = %+v, want unchanged %+v", ranges, want)
	}
}

func TestTimeRangeResolverHalfOpenBoundariesAndGaps(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	resolver, err := NewTimeRange(topology, []TimeRange{
		{Start: base.Add(2 * time.Hour), End: base.Add(3 * time.Hour), ShardID: "shard-b"},
		{Start: base, End: base.Add(time.Hour), ShardID: "shard-a"},
	})
	if err != nil {
		t.Fatalf("NewTimeRange() error = %v", err)
	}

	tests := []struct {
		key     time.Time
		wantID  shard.ID
		wantErr error
	}{
		{key: base, wantID: "shard-a"},
		{key: base.Add(time.Hour - time.Nanosecond), wantID: "shard-a"},
		{key: base.Add(time.Hour), wantErr: shard.ErrNoShard},
		{key: base.Add(2 * time.Hour), wantID: "shard-b"},
		{key: base.Add(3 * time.Hour), wantErr: shard.ErrNoShard},
	}

	for _, test := range tests {
		resolved, err := resolver.Resolve(test.key)
		if test.wantErr != nil {
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Resolve(%v) error = %v, want %v", test.key, err, test.wantErr)
			}

			continue
		}

		if err != nil {
			t.Fatalf("Resolve(%v) error = %v", test.key, err)
		}

		if got := resolved.ID(); got != test.wantID {
			t.Fatalf("Resolve(%v).ID() = %q, want %q", test.key, got, test.wantID)
		}
	}
}

func TestTimeRangeResolverNormalizesToUTC(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	resolver, err := NewTimeRange(topology, []TimeRange{
		{Start: start, End: end, ShardID: "shard-a"},
	})
	if err != nil {
		t.Fatalf("NewTimeRange() error = %v", err)
	}

	location := time.FixedZone("UTC+3", 3*60*60)
	key := start.Add(30 * time.Minute).In(location)

	resolved, err := resolver.Resolve(key)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got, want := resolved.ID(), shard.ID("shard-a"); got != want {
		t.Fatalf("Resolve().ID() = %q, want %q", got, want)
	}
}

func TestTimeRangeResolverAllowsAdjacentRanges(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	resolver, err := NewTimeRange(topology, []TimeRange{
		{Start: base, End: base.Add(time.Hour), ShardID: "shard-a"},
		{Start: base.Add(time.Hour), End: base.Add(2 * time.Hour), ShardID: "shard-b"},
	})
	if err != nil {
		t.Fatalf("NewTimeRange() error = %v", err)
	}

	resolved, err := resolver.Resolve(base.Add(time.Hour))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got, want := resolved.ID(), shard.ID("shard-b"); got != want {
		t.Fatalf("Resolve().ID() = %q, want %q", got, want)
	}
}

func TestTimeRangeResolverUninitialized(t *testing.T) {
	t.Parallel()

	var resolver *TimeRangeResolver

	_, err := resolver.Resolve(time.Now())
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/topology/shard/resolver: time range resolver is not initialized"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
