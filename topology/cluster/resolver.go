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

// ParseReadPolicy parses a ReadPolicy from its string representation.
func ParseReadPolicy(value string) (ReadPolicy, error) {
	switch value {
	case readPolicyPrimary:
		return ReadPrimary, nil
	case readPolicyReplicaPreferred:
		return ReadReplicaPreferred, nil
	case readPolicyReplicaRequired:
		return ReadReplicaRequired, nil
	default:
		return 0, fmt.Errorf(
			"xpg/topology/cluster: unknown read policy %q",
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

// ReadPool returns a pool according to policy.
//
// ReadReplicaPreferred falls back to the primary only when no replica can be
// selected. Other selector errors are returned to the caller.
func (c *Cluster) ReadPool(ctx context.Context, policy ReadPolicy) (*xpg.Pool, error) {
	if c == nil {
		return nil, errors.New("xpg/topology/cluster: cluster is nil")
	}

	switch policy {
	case ReadPrimary:
		return c.resolvePrimary()

	case ReadReplicaPreferred:
		replica, err := c.resolveReplica(ctx)
		if err == nil {
			return replica, nil
		}

		if !errors.Is(err, ErrNoReplica) {
			return nil, err
		}

		return c.resolvePrimary()

	case ReadReplicaRequired:
		return c.resolveReplica(ctx)

	default:
		return nil, fmt.Errorf("xpg/topology/cluster: unsupported read policy %d", policy)
	}
}

func (c *Cluster) resolvePrimary() (*xpg.Pool, error) {
	if c.primary == nil {
		return nil, ErrNoPrimary
	}

	return c.primary, nil
}

func (c *Cluster) resolveReplica(ctx context.Context) (*xpg.Pool, error) {
	if len(c.replicas) == 0 {
		return nil, ErrNoReplica
	}

	index, err := c.selector.Select(ctx, c.metadata)
	if err != nil {
		return nil, fmt.Errorf("xpg/topology/cluster: select replica: %w", err)
	}

	if index < 0 || index >= len(c.replicas) {
		return nil, fmt.Errorf(
			"xpg/topology/cluster: replica selector returned invalid index %d for %d replicas",
			index,
			len(c.replicas),
		)
	}

	return c.replicas[index], nil
}
