package resolver

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/mkbeh/xpg/topology/shard"
)

func TestNewRangeValidatesArguments(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")

	tests := []struct {
		name      string
		topology  *shard.Topology
		ranges    []Range[int]
		wantError string
	}{
		{
			name: "nil topology",
			ranges: []Range[int]{
				{Start: 0, End: 10, ShardID: "shard-a"},
			},
			wantError: "xpg/topology/shard/resolver: topology is nil or empty",
		},
		{
			name:      "empty ranges",
			topology:  topology,
			wantError: "xpg/topology/shard/resolver: range resolver requires at least one range",
		},
		{
			name:     "empty shard ID",
			topology: topology,
			ranges: []Range[int]{
				{Start: 0, End: 10},
			},
			wantError: "xpg/topology/shard/resolver: range 0: shard ID must not be empty",
		},
		{
			name:     "empty interval",
			topology: topology,
			ranges: []Range[int]{
				{Start: 10, End: 10, ShardID: "shard-a"},
			},
			wantError: "xpg/topology/shard/resolver: range 0 must satisfy start < end",
		},
		{
			name:     "reversed interval",
			topology: topology,
			ranges: []Range[int]{
				{Start: 20, End: 10, ShardID: "shard-a"},
			},
			wantError: "xpg/topology/shard/resolver: range 0 must satisfy start < end",
		},
		{
			name:     "unknown shard",
			topology: topology,
			ranges: []Range[int]{
				{Start: 0, End: 10, ShardID: "missing"},
			},
			wantError: `xpg/topology/shard/resolver: range 0: xpg/topology/shard: unknown shard "missing"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRange(
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

func TestNewRangeRejectsNaNBoundaries(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")

	tests := []struct {
		name       string
		valueRange Range[float64]
	}{
		{
			name: "NaN start",
			valueRange: Range[float64]{
				Start:   math.NaN(),
				End:     10,
				ShardID: "shard-a",
			},
		},
		{
			name: "NaN end",
			valueRange: Range[float64]{
				Start:   0,
				End:     math.NaN(),
				ShardID: "shard-a",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRange(
				topology,
				[]Range[float64]{test.valueRange},
			)
			if err == nil {
				t.Fatal("expected error")
			}

			if got, want := err.Error(),
				"xpg/topology/shard/resolver: range 0 must satisfy start < end"; got != want {
				t.Fatalf("error = %q, want %q", got, want)
			}
		})
	}
}

func TestNewRangeRejectsOverlapUsingSourceIndexes(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")

	_, err := NewRange(topology, []Range[int]{
		{Start: 100, End: 200, ShardID: "shard-b"},
		{Start: 50, End: 150, ShardID: "shard-a"},
	})
	if err == nil {
		t.Fatal("expected overlap error")
	}

	if got, want := err.Error(), "xpg/topology/shard/resolver: ranges 1 and 0 overlap"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestNewRangeDoesNotModifyInputOrder(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	ranges := []Range[int]{
		{Start: 100, End: 200, ShardID: "shard-b"},
		{Start: 0, End: 100, ShardID: "shard-a"},
	}
	want := slices.Clone(ranges)

	if _, err := NewRange(topology, ranges); err != nil {
		t.Fatalf("NewRange() error = %v", err)
	}

	if !slices.Equal(ranges, want) {
		t.Fatalf("ranges = %+v, want unchanged %+v", ranges, want)
	}
}

func TestRangeResolverHalfOpenBoundariesAndGaps(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	resolver, err := NewRange(topology, []Range[int]{
		{Start: 20, End: 30, ShardID: "shard-b"},
		{Start: 0, End: 10, ShardID: "shard-a"},
	})
	if err != nil {
		t.Fatalf("NewRange() error = %v", err)
	}

	tests := []struct {
		key     int
		wantID  shard.ID
		wantErr error
	}{
		{key: 0, wantID: "shard-a"},
		{key: 9, wantID: "shard-a"},
		{key: 10, wantErr: shard.ErrNoShard},
		{key: 19, wantErr: shard.ErrNoShard},
		{key: 20, wantID: "shard-b"},
		{key: 29, wantID: "shard-b"},
		{key: 30, wantErr: shard.ErrNoShard},
	}

	for _, test := range tests {
		resolved, err := resolver.Resolve(test.key)
		if test.wantErr != nil {
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Resolve(%d) error = %v, want %v", test.key, err, test.wantErr)
			}

			continue
		}

		if err != nil {
			t.Fatalf("Resolve(%d) error = %v", test.key, err)
		}

		if got := resolved.ID(); got != test.wantID {
			t.Fatalf("Resolve(%d).ID() = %q, want %q", test.key, got, test.wantID)
		}
	}
}

func TestRangeResolverAllowsAdjacentRanges(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	resolver, err := NewRange(topology, []Range[int]{
		{Start: 0, End: 100, ShardID: "shard-a"},
		{Start: 100, End: 200, ShardID: "shard-b"},
	})
	if err != nil {
		t.Fatalf("NewRange() error = %v", err)
	}

	resolved, err := resolver.Resolve(100)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got, want := resolved.ID(), shard.ID("shard-b"); got != want {
		t.Fatalf("Resolve().ID() = %q, want %q", got, want)
	}
}

func TestRangeResolverSupportsStrings(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	resolver, err := NewRange(topology, []Range[string]{
		{Start: "a", End: "m", ShardID: "shard-a"},
		{Start: "m", End: "z", ShardID: "shard-b"},
	})
	if err != nil {
		t.Fatalf("NewRange() error = %v", err)
	}

	resolved, err := resolver.Resolve("m")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got, want := resolved.ID(), shard.ID("shard-b"); got != want {
		t.Fatalf("Resolve().ID() = %q, want %q", got, want)
	}
}

func TestRangeResolverUninitialized(t *testing.T) {
	t.Parallel()

	var resolver *RangeResolver[int]

	_, err := resolver.Resolve(1)
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/topology/shard/resolver: range resolver is not initialized"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
