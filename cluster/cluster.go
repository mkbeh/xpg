package cluster

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/mkbeh/xpg"
)

// Config configures a Cluster from independently created pools.
//
// New takes ownership of Primary and Replicas only after it returns
// successfully. Cluster.Close closes the owned pools.
type Config struct {
	Primary  *xpg.Pool
	Replicas []*xpg.Pool
	Selector ReplicaSelector
}

// Cluster routes operations between one primary pool and optional replica
// pools.
//
// A deployment with one PostgreSQL endpoint is represented by a Cluster with
// one Primary and no Replicas.
//
// Cluster does not inspect SQL, retry failed queries, promote replicas, or
// discover PostgreSQL nodes. Those responsibilities remain with the caller or
// the surrounding high-availability infrastructure.
type Cluster struct {
	primary  *xpg.Pool
	replicas []*xpg.Pool

	metadata replicaMetadata
	selector ReplicaSelector

	closeOnce sync.Once
}

// New creates a Cluster from independently configured pools.
//
// Primary is required. Replicas may be omitted. When Selector is nil,
// replicas are selected using round-robin.
func New(config Config) (*Cluster, error) {
	if config.Primary != nil && invalidPool(config.Primary) {
		return nil, errors.New("xpg/cluster: primary pool is invalid")
	}

	if config.Primary == nil && len(config.Replicas) == 0 {
		return nil, errors.New("xpg/cluster: at least one pool is required")
	}

	replicas := slices.Clone(config.Replicas)
	metadata := make(replicaMetadata, len(replicas))

	for index, replica := range replicas {
		if invalidPool(replica) {
			return nil, fmt.Errorf("xpg/cluster: replica %d is nil", index)
		}

		metadata[index] = ReplicaInfo{
			name:   replica.Name(),
			labels: cloneLabels(replica.Labels()),
		}
	}

	selector := config.Selector
	if selector == nil {
		selector = RoundRobinSelector()
	}

	return &Cluster{
		primary:  config.Primary,
		replicas: replicas,
		metadata: metadata,
		selector: selector,
	}, nil
}

func invalidPool(pool *xpg.Pool) bool {
	return pool == nil || pool.Raw() == nil
}

// Primary returns the primary pool owned by the cluster.
//
// The returned pool is borrowed and must not be closed separately.
func (c *Cluster) Primary() *xpg.Pool {
	if c == nil {
		return nil
	}

	return c.primary
}

// ReplicaCount returns the number of replicas registered in the cluster.
func (c *Cluster) ReplicaCount() int {
	if c == nil {
		return 0
	}

	return len(c.replicas)
}

// ReplicaAt returns the replica at index in registration order.
//
// The returned pool is borrowed and must not be closed separately. ReplicaAt
// panics when c is nil or index is outside the replica set, matching ordinary
// slice indexing semantics.
func (c *Cluster) ReplicaAt(index int) *xpg.Pool {
	return c.replicas[index]
}

// Close closes all replica pools in reverse registration order and then closes
// the primary pool.
//
// Close is safe to call multiple times.
func (c *Cluster) Close() {
	if c == nil {
		return
	}

	c.closeOnce.Do(func() {
		for index := len(c.replicas) - 1; index >= 0; index-- {
			c.replicas[index].Close()
		}

		c.primary.Close()
	})
}
