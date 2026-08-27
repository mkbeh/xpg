package resolver

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/mkbeh/xpg/topology/shard"
)

const (
	// rendezvousDomain identifies the persistent hash-placement algorithm.
	// Changing it changes shard placement and therefore requires data migration.
	rendezvousDomain = "xpg.shard.rendezvous.v1"

	// rendezvousLengthSize is the size of each uint32 length prefix in bytes.
	rendezvousLengthSize = 4
)

// RendezvousResolver implements rendezvous/HRW routing with SHA-256 and stable
// named shard IDs.
type RendezvousResolver[K any] struct {
	shards      []shard.Shard
	prefix      []byte
	encoder     KeyEncoder[K]
	maxIDLength int
}

// NewRendezvous creates the version-1 rendezvous resolver bound to topology.
//
// Namespace is part of the persistent placement contract. Changing it changes
// shard placement and may require data migration. The topology's shard slice is
// copied once; Resolve does not consult topology.
func NewRendezvous[K any](
	topology *shard.Topology,
	namespace string,
	encoder KeyEncoder[K],
) (*RendezvousResolver[K], error) {
	if err := requireTopology(topology); err != nil {
		return nil, err
	}

	if encoder == nil {
		return nil, errors.New("xpg/shard/resolver: key encoder is nil")
	}

	if namespace == "" {
		return nil, errors.New("xpg/shard/resolver: rendezvous namespace must not be empty")
	}

	if uint64(len(namespace)) > uint64(math.MaxUint32) {
		return nil, errors.New("xpg/shard/resolver: rendezvous namespace is too large")
	}

	shards := topology.Shards()
	maxIDLength := 0

	for _, candidate := range shards {
		id := candidate.ID()

		if uint64(len(id)) > uint64(math.MaxUint32) {
			return nil, errors.New("xpg/shard/resolver: shard ID is too large")
		}

		maxIDLength = max(maxIDLength, len(id))
	}

	prefixSize := len(rendezvousDomain) + rendezvousLengthSize + len(namespace)
	prefix := make([]byte, prefixSize)

	lengthOffset := len(rendezvousDomain)
	namespaceOffset := lengthOffset + rendezvousLengthSize

	copy(prefix, rendezvousDomain)

	binary.BigEndian.PutUint32(
		prefix[lengthOffset:namespaceOffset],
		uint32(len(namespace)),
	)

	copy(prefix[namespaceOffset:], namespace)

	return &RendezvousResolver[K]{
		shards:      shards,
		prefix:      prefix,
		encoder:     encoder,
		maxIDLength: maxIDLength,
	}, nil
}

// Resolve maps key to a shard using rendezvous hashing.
func (resolver *RendezvousResolver[K]) Resolve(key K) (shard.Shard, error) {
	if resolver == nil || len(resolver.shards) == 0 || resolver.encoder == nil {
		return shard.Shard{}, errors.New("xpg/shard/resolver: rendezvous resolver is not initialized")
	}

	encoded, err := resolver.encoder.Encode(key)
	if err != nil {
		return shard.Shard{}, fmt.Errorf("xpg/shard/resolver: encode rendezvous key: %w", err)
	}

	if uint64(len(encoded)) > uint64(math.MaxUint32) {
		return shard.Shard{}, errors.New("xpg/shard/resolver: encoded key is too large")
	}

	keyLengthOffset := len(resolver.prefix)
	keyOffset := keyLengthOffset + rendezvousLengthSize
	idLengthOffset := keyOffset + len(encoded)
	idOffset := idLengthOffset + rendezvousLengthSize

	scoreInput := make([]byte, idOffset+resolver.maxIDLength)

	copy(scoreInput, resolver.prefix)

	binary.BigEndian.PutUint32(
		scoreInput[keyLengthOffset:keyOffset],
		uint32(len(encoded)),
	)

	copy(scoreInput[keyOffset:idLengthOffset], encoded)

	var (
		selected shard.Shard
		best     [sha256.Size]byte
		bestID   shard.ID
		hasBest  bool
	)

	for _, candidate := range resolver.shards {
		candidateID := candidate.ID()
		inputEnd := idOffset + len(candidateID)

		binary.BigEndian.PutUint32(
			scoreInput[idLengthOffset:idOffset],
			uint32(len(candidateID)),
		)

		copy(scoreInput[idOffset:inputEnd], candidateID)

		score := sha256.Sum256(scoreInput[:inputEnd])
		comparison := bytes.Compare(score[:], best[:])

		// Shard ID is the deterministic tie-breaker, so placement does not
		// depend on topology registration order when scores are equal.
		if !hasBest || comparison > 0 || (comparison == 0 && candidateID < bestID) {
			selected = candidate
			best = score
			bestID = candidateID
			hasBest = true
		}
	}

	if !hasBest {
		return shard.Shard{}, shard.ErrNoShard
	}

	return selected, nil
}
