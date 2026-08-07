package cluster

import "errors"

// ErrNoReplica indicates that a read policy required a replica but no replica
// was available for selection.
var ErrNoReplica = errors.New("xpg/cluster: no replica available")
