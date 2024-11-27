package postgres

import (
	"hash/fnv"

	"github.com/google/uuid"
)

func GenerateUUID() string {
	return uuid.New().String()
}

func StringAsHash64(s string) uint64 {
	hash := fnv.New64()
	_, _ = hash.Write([]byte(s))
	return hash.Sum64()
}
