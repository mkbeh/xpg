// Package cluster provides explicit routing between a PostgreSQL primary pool,
// when configured, and zero or more replica pools.
//
// The package does not inspect SQL, retry failed queries, promote replicas,
// or discover PostgreSQL nodes.
package cluster
