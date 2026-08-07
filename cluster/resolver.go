package cluster

import (
	"context"
	"errors"
	"fmt"

	"github.com/mkbeh/xpg"
)

// ReadPolicy defines where a Cluster resolves a read operation.
type ReadPolicy uint8

const (
	// ReadPrimary always resolves reads to the primary pool.
	ReadPrimary ReadPolicy = iota

	// ReadReplicaPreferred prefers a replica and falls back to the primary when
	// no replica can be selected.
	ReadReplicaPreferred

	// ReadReplicaRequired requires a replica and returns ErrNoReplica when none
	// can be selected.
	ReadReplicaRequired
)

const (
	readPolicyPrimary          = "primary"
	readPolicyReplicaPreferred = "replica_preferred"
	readPolicyReplicaRequired  = "replica_required"
)

// ParsePolicy parses a ReadPolicy from its string representation.
func ParsePolicy(value string) (ReadPolicy, error) {
	switch value {
	case readPolicyPrimary:
		return ReadPrimary, nil
	case readPolicyReplicaPreferred:
		return ReadReplicaPreferred, nil
	case readPolicyReplicaRequired:
		return ReadReplicaRequired, nil
	default:
		return 0, fmt.Errorf(
			"xpg/cluster: unknown read policy %q",
			value,
		)
	}
}

// String returns the string representation of the read policy.
func (policy ReadPolicy) String() string {
	switch policy {
	case ReadPrimary:
		return readPolicyPrimary
	case ReadReplicaPreferred:
		return readPolicyReplicaPreferred
	case ReadReplicaRequired:
		return readPolicyReplicaRequired
	default:
		return "unknown"
	}
}

// ReadPool returns a pool for a read operation according to policy.
//
// ReadReplicaPreferred falls back to the primary only when replica selection
// returns ErrNoReplica. Other selector errors are returned to the caller.
func (c *Cluster) ReadPool(ctx context.Context, policy ReadPolicy) (*xpg.Pool, error) {
	if c == nil || c.primary == nil {
		return nil, errors.New("xpg/cluster: cluster is nil")
	}

	switch policy {
	case ReadPrimary:
		return c.primary, nil

	case ReadReplicaPreferred:
		replica, err := c.selectReplica(ctx)
		if errors.Is(err, ErrNoReplica) {
			return c.primary, nil
		}

		return replica, err

	case ReadReplicaRequired:
		return c.selectReplica(ctx)

	default:
		return nil, fmt.Errorf("xpg/cluster: unsupported read policy %d", policy)
	}
}

func (c *Cluster) selectReplica(ctx context.Context) (*xpg.Pool, error) {
	if len(c.replicas) == 0 {
		return nil, ErrNoReplica
	}

	index, err := c.selector.Select(ctx, c.metadata)
	if err != nil {
		return nil, fmt.Errorf("xpg/cluster: select replica: %w", err)
	}

	if index < 0 || index >= len(c.replicas) {
		return nil, fmt.Errorf(
			"xpg/cluster: replica selector returned index %d for %d replicas",
			index,
			len(c.replicas),
		)
	}

	return c.replicas[index], nil
}
