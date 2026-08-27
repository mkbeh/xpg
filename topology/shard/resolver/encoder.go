package resolver

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// KeyEncoder converts a typed key into stable canonical bytes.
//
// Implementations must be deterministic and safe for concurrent use. Changing
// an encoder changes persistent hash placement and may require data migration.
type KeyEncoder[K any] interface {
	Encode(K) ([]byte, error)
}

// KeyEncoderFunc adapts a function to KeyEncoder.
type KeyEncoderFunc[K any] func(K) ([]byte, error)

// Encode calls the wrapped encoder function.
func (encoder KeyEncoderFunc[K]) Encode(key K) ([]byte, error) {
	if encoder == nil {
		return nil, errors.New("xpg/shard/resolver: key encoder function is nil")
	}

	return encoder(key)
}

// StringKeyEncoder encodes the exact bytes stored in a string without
// normalization.
func StringKeyEncoder() KeyEncoder[string] {
	return KeyEncoderFunc[string](
		func(key string) ([]byte, error) {
			return []byte(key), nil
		},
	)
}

// BytesKeyEncoder encodes exact bytes and returns a defensive copy.
func BytesKeyEncoder() KeyEncoder[[]byte] {
	return KeyEncoderFunc[[]byte](
		func(key []byte) ([]byte, error) {
			return bytes.Clone(key), nil
		},
	)
}

// Bytes16KeyEncoder encodes a 16-byte key exactly.
func Bytes16KeyEncoder() KeyEncoder[[16]byte] {
	return KeyEncoderFunc[[16]byte](
		func(key [16]byte) ([]byte, error) {
			return bytes.Clone(key[:]), nil
		},
	)
}

// Bytes32KeyEncoder encodes a 32-byte key exactly.
func Bytes32KeyEncoder() KeyEncoder[[32]byte] {
	return KeyEncoderFunc[[32]byte](
		func(key [32]byte) ([]byte, error) {
			return bytes.Clone(key[:]), nil
		},
	)
}

// Int64KeyEncoder encodes a signed integer as big-endian two's-complement.
func Int64KeyEncoder() KeyEncoder[int64] {
	return KeyEncoderFunc[int64](
		func(key int64) ([]byte, error) {
			encoded := make([]byte, 8)

			binary.BigEndian.PutUint64(encoded, uint64(key))

			return encoded, nil
		},
	)
}

// Uint64KeyEncoder encodes an unsigned integer as big-endian bytes.
func Uint64KeyEncoder() KeyEncoder[uint64] {
	return KeyEncoderFunc[uint64](
		func(key uint64) ([]byte, error) {
			encoded := make([]byte, 8)

			binary.BigEndian.PutUint64(encoded, key)

			return encoded, nil
		},
	)
}
