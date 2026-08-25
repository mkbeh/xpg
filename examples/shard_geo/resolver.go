package main

import (
	"fmt"

	"github.com/mkbeh/xpg/shard"
	"github.com/mkbeh/xpg/shard/resolver"
)

func newTenantResolver(topology *shard.Topology) (shard.Resolver[tenantKey], error) {
	shardByRegion := make(map[string]shard.ID, topology.Len())

	// Build the routing index once instead of scanning the topology on every
	// tenant lookup.
	for _, candidate := range topology.Shards() {
		region, ok := candidate.Label("region")
		if !ok || region == "" {
			return nil, fmt.Errorf("shard %q has no region label", candidate.ID())
		}

		if existing, exists := shardByRegion[region]; exists {
			return nil, fmt.Errorf(
				"region %q is assigned to both shard %q and shard %q",
				region,
				existing,
				candidate.ID(),
			)
		}

		shardByRegion[region] = candidate.ID()
	}

	resolve := resolver.ResolveFunc[tenantKey](
		func(key tenantKey, _ *shard.Topology) (shard.ID, error) {
			id, ok := shardByRegion[key.Region]
			if !ok {
				return "", fmt.Errorf(
					"tenant %q has unsupported region %q: %w",
					key.TenantID,
					key.Region,
					shard.ErrNoShard,
				)
			}

			return id, nil
		},
	)

	return resolver.NewCustom(topology, resolve)
}
