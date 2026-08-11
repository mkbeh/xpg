package resolver

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/mkbeh/xpg/shard"
)

const (
	// rendezvousDomain identifies the persistent hash-placement algorithm.
	// Changing it changes shard placement and therefore requires data migration.
	rendezvousDomain = "xpg.shard.rendezvous.v1"

	// rendezvousLengthSize is the size of each uint32 length prefix in bytes.
	rendezvousLengthSize = 4
)

// HashResolver implements rendezvous/HRW routing with SHA-256 and stable named
// shard IDs.
type HashResolver[K any] struct {
	shards      []shard.Shard
	prefix      []byte
	encoder     KeyEncoder[K]
	maxIDLength int
}

// NewHash creates the version-1 rendezvous resolver bound to topology.
//
// Namespace is part of the persistent placement contract. Changing it changes
// shard placement and may require data migration.
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

	trimmedNamespace := strings.TrimSpace(namespace)

	if trimmedNamespace == "" {
		return nil, errors.New("xpg/shard/resolver: hash namespace must not be blank")
	}

	if trimmedNamespace != namespace {
		return nil, errors.New("xpg/shard/resolver: hash namespace must not contain surrounding whitespace")
	}

	if len(namespace) > math.MaxUint32 {
		return nil, errors.New("xpg/shard/resolver: hash namespace is too large")
	}

	shards := topology.Shards()
	maxIDLength := 0

	for _, candidate := range shards {
		id := candidate.ID()

		if len(id) > math.MaxUint32 {
			return nil, errors.New("xpg/shard/resolver: shard ID is too large")
		}

		maxIDLength = max(maxIDLength, len(id))
	}

	// Prefix is invariant for the lifetime of the resolver:
	//
	// domain || namespace_length || namespace
	prefix := make([]byte, len(rendezvousDomain)+rendezvousLengthSize+len(namespace))

	offset := copy(prefix, rendezvousDomain)

	binary.BigEndian.PutUint32(
		prefix[offset:offset+rendezvousLengthSize],
		uint32(len(namespace)),
	)
	offset += rendezvousLengthSize

	copy(prefix[offset:], namespace)

	return &HashResolver[K]{
		shards:      shards,
		prefix:      prefix,
		encoder:     encoder,
		maxIDLength: maxIDLength,
	}, nil
}

// Resolve selects the shard with the lexicographically greatest SHA-256 score.
//
// Resolve performs only in-memory routing. It does not acquire a connection or
// execute a PostgreSQL query.
func (resolver *HashResolver[K]) Resolve(key K) (shard.Shard, error) {
	if resolver == nil ||
		len(resolver.shards) == 0 ||
		resolver.encoder == nil {
		return shard.Shard{}, errors.New("xpg/shard/resolver: hash resolver is not initialized")
	}

	encoded, err := resolver.encoder.Encode(key)
	if err != nil {
		return shard.Shard{}, fmt.Errorf("xpg/shard/resolver: encode hash key: %w", err)
	}

	if len(encoded) > math.MaxUint32 {
		return shard.Shard{}, errors.New("xpg/shard/resolver: encoded key is too large")
	}

	// Build the candidate-independent prefix once. The shard ID suffix is
	// overwritten for each candidate below.
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
		if !hasBest ||
			comparison > 0 ||
			comparison == 0 && candidateID < bestID {
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
