// Package secrets provides the crypto-low foundation for protecting integration
// secrets at rest. It implements envelope encryption on Google Tink
// (tink-go/v2): a data-encryption key (DEK) seals each secret value, and a
// key-encryption key (KEK) seals the DEK. Only the wrapped DEK and the sealed
// secrets are persisted — plaintext key material never touches the database.
//
// Format v2 (Tink): a DEK is a serialized cleartext Tink keyset built from the
// AES-256-GCM key template; it exists only in memory (fresh from GenerateDEK or
// unwrapped by the Keyring) or wrapped under a KEK in data_keys. A secret
// envelope is Tink's AEAD output — [5-byte key-id prefix][iv][ct+tag] — so nonce
// management is Tink's responsibility, not ours.
//
// Every seal binds an AAD (additional authenticated data) that ties the envelope
// to its logical slot — the kind+secret-key for a secret, the kek id for a
// wrapped DEK (see aad.go). The AAD is authenticated but not stored, so the exact
// same AAD must be supplied to open. This defeats an at-rest attacker who can
// write the DB from swapping one ciphertext into another slot: authentication
// fails when the AAD differs. The AAD domain carries the format version (/v2),
// so the on-disk format is self-describing without a separate version byte.
package secrets

import (
	"fmt"

	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/tink"
)

// SecretCipher seals and opens secret values under a DEK. The DEK is a
// serialized cleartext Tink keyset (see GenerateDEK); it is stored wrapped by a
// KEK (see Keyring), and callers pass the already-unwrapped keyset bytes.
type SecretCipher interface {
	// Encrypt seals plaintext under the DEK-keyset, binding aad, and returns a
	// Tink AEAD envelope. The same aad must be passed to Decrypt.
	Encrypt(dek, plaintext, aad []byte) ([]byte, error)
	// Decrypt opens an envelope produced by Encrypt; it fails if aad differs.
	Decrypt(dek, envelope, aad []byte) ([]byte, error)
}

// AESCipher is the AES-256-GCM implementation of SecretCipher, delegating the
// primitive (nonce handling included) to Tink. It is stateless and safe for
// concurrent use.
type AESCipher struct{}

// NewAESCipher returns the default SecretCipher.
func NewAESCipher() AESCipher { return AESCipher{} }

func (AESCipher) Encrypt(dek, plaintext, aad []byte) ([]byte, error) {
	primitive, err := dekAEAD(dek)
	if err != nil {
		return nil, fmt.Errorf("secrets: encrypt: %w", err)
	}
	envelope, err := primitive.Encrypt(plaintext, aad)
	if err != nil {
		return nil, fmt.Errorf("secrets: encrypt: %w", err)
	}
	return envelope, nil
}

func (AESCipher) Decrypt(dek, envelope, aad []byte) ([]byte, error) {
	primitive, err := dekAEAD(dek)
	if err != nil {
		return nil, fmt.Errorf("secrets: decrypt: %w", err)
	}
	plaintext, err := primitive.Decrypt(envelope, aad)
	if err != nil {
		return nil, fmt.Errorf("secrets: decrypt: %w", err)
	}
	return plaintext, nil
}

// dekAEAD deserializes a cleartext DEK-keyset (via parseDEKKeyset — see
// dekkeyset.go for the transient-use contract) and builds its AEAD primitive.
// A dek that is not a valid serialized keyset (including a raw 32-byte
// format-v1 key) is rejected here.
func dekAEAD(dek []byte) (tink.AEAD, error) {
	handle, err := parseDEKKeyset(dek)
	if err != nil {
		return nil, err
	}
	primitive, err := aead.New(handle)
	if err != nil {
		return nil, fmt.Errorf("dek aead: %w", err)
	}
	return primitive, nil
}
