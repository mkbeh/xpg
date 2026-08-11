package cluster

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/mkbeh/xpg"
)

// ID identifies one logical PostgreSQL cluster.
type ID string

// Config configures a Cluster from independently created pools.
//
// ID and Labels are optional cluster metadata. New takes ownership of Primary
// and Replicas only after it returns successfully. Cluster.Close closes the
// owned pools.
type Config struct {
	ID     ID
	Labels map[string]string

	Primary  *xpg.Pool
	Replicas []*xpg.Pool
	Selector ReplicaSelector
}

// Cluster routes operations across an optional primary pool and zero or more
// replica pools.
//
// A deployment with one PostgreSQL endpoint is represented by a Cluster with
// one Primary and no Replicas. A read-only deployment may omit Primary and
// configure only Replicas.
//
// Cluster does not inspect SQL, retry failed queries, promote replicas, or
// discover PostgreSQL nodes. Those responsibilities remain with the caller or
// the surrounding high-availability infrastructure.
type Cluster struct {
	id     ID
	labels map[string]string

	primary  *xpg.Pool
	replicas []*xpg.Pool

	metadata replicaMetadata
	selector ReplicaSelector

	closeOnce sync.Once
}

// New creates a Cluster from independently configured pools.
//
// At least one pool is required. When Selector is nil, replicas are selected
// using round-robin.
func New(config Config) (*Cluster, error) {
	if config.Primary != nil && invalidPool(config.Primary) {
		return nil, errors.New("xpg/cluster: primary pool is invalid")
	}

	if config.Primary == nil && len(config.Replicas) == 0 {
		return nil, errors.New("xpg/cluster: at least one pool is required")
	}

	if err := validateLabels(config.Labels); err != nil {
		return nil, fmt.Errorf("xpg/cluster: %w", err)
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
		id:       config.ID,
		labels:   cloneLabels(config.Labels),
		primary:  config.Primary,
		replicas: replicas,
		metadata: metadata,
		selector: selector,
	}, nil
}

func invalidPool(pool *xpg.Pool) bool {
	return pool == nil || pool.Raw() == nil
}

func validateLabels(labels map[string]string) error {
	for key, value := range labels {
		if key == "" {
			return errors.New("label key must not be empty")
		}

		if value == "" {
			return fmt.Errorf("label %q value must not be empty", key)
		}
	}

	return nil
}

// ID returns the stable logical cluster ID.
func (c *Cluster) ID() ID {
	if c == nil {
		return ""
	}

	return c.id
}

// Label returns one cluster label without allocating a copy of all labels.
func (c *Cluster) Label(key string) (string, bool) {
	if c == nil {
		return "", false
	}

	value, ok := c.labels[key]

	return value, ok
}

// Labels returns a defensive copy of cluster labels.
func (c *Cluster) Labels() map[string]string {
	if c == nil {
		return nil
	}

	return cloneLabels(c.labels)
}

// Primary returns the primary pool owned by the cluster.
//
// Primary returns nil when no primary is configured. The returned pool is
// borrowed and must not be closed separately.
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
// the primary pool when one is configured.
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

		if c.primary != nil {
			c.primary.Close()
		}
	})
}
