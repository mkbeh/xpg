package cluster

import "errors"

var (
	// ErrNoPrimary is returned when an operation requires a primary but none
	// is configured.
	ErrNoPrimary = errors.New("xpg/topology/cluster: no primary available")

	// ErrNoReplica is returned when an operation requires a replica but none
	// can be selected.
	ErrNoReplica = errors.New("xpg/topology/cluster: no replica available")
)
