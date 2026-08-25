package resolver

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/mkbeh/xpg/shard"
)

const (
	// rendezvousDomain identifies the persistent hash-placement algorithm.
	// Changing it changes shard placement and therefore requires data migration.
	rendezvousDomain = "xpg.shard.rendezvous.v1"

	// rendezvousLengthSize is the size of each uint32 length prefix in bytes.
	rendezvousLengthSize = 4
)

// HashResolver routes keys using rendezvous/HRW hashing with SHA-256.
//
// HashResolver captures the shard set when it is created. The shards remain
// borrowed from the topology, so the resolver must not outlive it.
type HashResolver[K any] struct {
	shards           []shard.Shard
	prefix           []byte
	encoder          KeyEncoder[K]
	maxShardIDLength int
}

// NewHash creates a rendezvous hash resolver bound to topology.
//
// Namespace is an opaque non-empty string and part of the persistent placement
// contract. Changing the namespace, key encoder, shard IDs, or placement format
// changes shard placement and may require data migration.
func NewHash[K any](
	topology *shard.Topology,
	namespace string,
	encoder KeyEncoder[K],
) (*HashResolver[K], error) {
	if err := requireTopology(topology); err != nil {
		return nil, err
	}

	if encoder == nil {
		return nil, errors.New("xpg/shard/resolver: key encoder is nil")
	}

	if namespace == "" {
		return nil, errors.New("xpg/shard/resolver: hash namespace must not be empty")
	}

	if len(namespace) > math.MaxUint32 {
		return nil, errors.New("xpg/shard/resolver: hash namespace is too large")
	}

	shards := topology.Shards()
	maxShardIDLength := 0

	for _, candidate := range shards {
		id := candidate.ID()

		if len(id) > math.MaxUint32 {
			return nil, errors.New("xpg/shard/resolver: shard ID is too large")
		}

		maxShardIDLength = max(maxShardIDLength, len(id))
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

	return &HashResolver[K]{
		shards:           shards,
		prefix:           prefix,
		encoder:          encoder,
		maxShardIDLength: maxShardIDLength,
	}, nil
}

// Resolve maps key to a shard using rendezvous hashing.
func (resolver *HashResolver[K]) Resolve(key K) (shard.Shard, error) {
	if resolver == nil || len(resolver.shards) == 0 || resolver.encoder == nil {
		return shard.Shard{}, errors.New("xpg/shard/resolver: hash resolver is not initialized")
	}

	encoded, err := resolver.encoder.Encode(key)
	if err != nil {
		return shard.Shard{}, fmt.Errorf("xpg/shard/resolver: encode hash key: %w", err)
	}

	if len(encoded) > math.MaxUint32 {
		return shard.Shard{}, errors.New("xpg/shard/resolver: encoded key is too large")
	}

	keyLengthOffset := len(resolver.prefix)
	keyOffset := keyLengthOffset + rendezvousLengthSize
	idLengthOffset := keyOffset + len(encoded)
	idOffset := idLengthOffset + rendezvousLengthSize

	// Persistent placement format:
	//
	// domain || namespace_length || namespace ||
	// key_length || key || shard_id_length || shard_id
	//
	// The candidate-independent prefix and key are written once. Only the shard
	// ID suffix is overwritten while evaluating candidates.
	scoreInput := make(
		[]byte,
		idOffset+resolver.maxShardIDLength,
	)

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
		if !hasBest ||
			comparison > 0 ||
			(comparison == 0 && candidateID < bestID) {
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
