package entity

import (
	"time"

	"github.com/google/uuid"
)

// DataKey is a data-encryption key (DEK) stored wrapped by a KEK. EncryptedDEK
// holds the envelope bytes ([nonce][ciphertext]); the plaintext DEK is never
// persisted. KEKID names the KEK that wrapped it, so a rotation can re-wrap and
// UnwrapDEK can pick the right KEK.
type DataKey struct {
	ID           uuid.UUID
	KEKID        string
	EncryptedDEK []byte
	Label        *string
	CreatedAt    time.Time
}
