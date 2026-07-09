package dekrotator

import (
	"github.com/ruko1202/maintmode/internal/storages/datakey"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Keyring is the subset of *secrets.Keyring rotation needs. Depending on the
// interface (not the concrete type) keeps the store testable and documents that
// rotation only unwraps existing DEKs and wraps them under the active KEK.
type Keyring interface {
	ActiveKEKID() string
	Knows(kekID string) bool
	WrapDEK(dek []byte) (wrapped []byte, kekID string, err error)
	UnwrapDEK(wrapped []byte, kekID string) ([]byte, error)
}

// Rotator re-wraps every DEK under the keyring's active KEK. It exists to migrate
// stored DEKs onto a new KEK after the active KEK changes; the DEK plaintext is
// unchanged, so secrets sealed by those DEKs stay readable and need no re-encrypt.
type Rotator struct {
	txManager *dbtx.TxManager
	store     *datakey.Store
	keyring   Keyring
}

func NewRotator(txManager *dbtx.TxManager, store *datakey.Store, kr Keyring) *Rotator {
	return &Rotator{
		txManager: txManager,
		store:     store,
		keyring:   kr,
	}
}
