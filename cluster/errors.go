package cluster

import "errors"

var (
	// ErrNoPrimary is returned when an operation requires a primary pool but
	// the cluster has no primary configured.
	ErrNoPrimary = errors.New("xpg/cluster: no primary available")

	// ErrNoReplica is returned when an operation requires a replica but no
	// replica can be selected.
	ErrNoReplica = errors.New("xpg/cluster: no replica available")
)
