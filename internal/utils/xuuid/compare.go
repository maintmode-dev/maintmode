package xuuid

import (
	"bytes"

	"github.com/google/uuid"
)

func Compare(a, b uuid.UUID) int {
	return bytes.Compare(a[:], b[:])
}
