package xuuid

import (
	"bytes"

	"github.com/google/uuid"
)

func Compare(a, b uuid.UUID) int {
	return bytes.Compare(a[:], b[:])
}

func CompareBool(a, b uuid.UUID) bool {
	return Compare(a, b) < 0
}
