package secrets

import (
	"bytes"
	"fmt"

	"github.com/tink-crypto/tink-go/v2/core/registry"
	"github.com/tink-crypto/tink-go/v2/keyset"
)

// Keyring wraps/unwraps data-encryption keys (DEKs) under key-encryption keys
// (KEKs) served by a registry.KMSClient. A KEK is addressed by URI: the active
// URI wraps new DEKs; any URI the client supports (e.g. a retired local KEK
// kept in config) still unwraps during a rotation.
//
// The client is injected, not looked up in Tink's global KMS registry — the
// process-global registry is shared mutable state that tests and repeated
// bootstraps would fight over. Swapping local KEKs for a cloud KMS means
// injecting that provider's client; the Keyring does not change.
type Keyring struct {
	activeKEKURI string
	kms          registry.KMSClient
}

// NewKeyring returns a keyring wrapping under activeKEKURI, or fails fast: a
// missing client, an unsupported active URI, or an active KEK the client cannot
// build an AEAD for (probed here) all abort startup rather than surfacing on
// the first write.
func NewKeyring(activeKEKURI string, kms registry.KMSClient) (*Keyring, error) {
	if !kms.Supported(activeKEKURI) {
		return nil, fmt.Errorf("secrets: active kek %q not supported by the kms client", activeKEKURI)
	}
	// Probe the active KEK now: a client may claim support but fail to build the
	// primitive (bad key material, unreachable KMS). Startup is the right place
	// to find out.
	if _, err := kms.GetAEAD(activeKEKURI); err != nil {
		return nil, fmt.Errorf("secrets: active kek %q: %w", activeKEKURI, err)
	}
	return &Keyring{activeKEKURI: activeKEKURI, kms: kms}, nil
}

// NewLocalKeyring builds a keyring over locally-configured KEKs (see
// NewLocalKMSClient) — the non-KMS deployment shape and the test default.
func NewLocalKeyring(activeKEKURI string, keys map[string]string) (*Keyring, error) {
	kms, err := NewLocalKMSClient(keys)
	if err != nil {
		return nil, err
	}
	return NewKeyring(activeKEKURI, kms)
}

// ActiveKEKID returns the URI of the KEK that WrapDEK uses. Callers stamp it
// onto data_keys.kek_id so UnwrapDEK can later find the right KEK. (The column
// and method predate the URI form; the "id" IS the URI in format v2.)
func (k *Keyring) ActiveKEKID() string { return k.activeKEKURI }

// Knows reports whether the keyring can serve the KEK with the given URI.
// Rotation uses it to skip DEKs sealed under a KEK that is not available
// (orphaned/foreign rows it cannot and should not re-wrap) rather than failing
// on an unwrap it can't perform.
func (k *Keyring) Knows(kekURI string) bool {
	return k.kms.Supported(kekURI)
}

// WrapDEK seals a serialized DEK-keyset under the active KEK and returns the
// wrapped bytes plus the KEK URI used, for storage in
// data_keys(encrypted_dek, kek_id). The wrap is Tink's encrypted-keyset write,
// authenticated against the active-URI-bound AAD, so a wrapped DEK cannot be
// unwrapped under a different KEK URI than the one it was sealed with.
func (k *Keyring) WrapDEK(dek []byte) (wrapped []byte, kekID string, err error) {
	if len(dek) == 0 {
		return nil, "", fmt.Errorf("secrets: dek keyset is empty")
	}

	handle, err := parseDEKKeyset(dek)
	if err != nil {
		return nil, "", fmt.Errorf("secrets: wrap dek: %w", err)
	}
	kekAEAD, err := k.kms.GetAEAD(k.activeKEKURI)
	if err != nil {
		return nil, "", fmt.Errorf("secrets: wrap dek: %w", err)
	}

	var buf bytes.Buffer
	if err := handle.WriteWithAssociatedData(keyset.NewBinaryWriter(&buf), kekAEAD, DEKWrapAAD(k.activeKEKURI)); err != nil {
		return nil, "", fmt.Errorf("secrets: wrap dek: %w", err)
	}
	return buf.Bytes(), k.activeKEKURI, nil
}

// UnwrapDEK opens a wrapped DEK-keyset using the KEK addressed by kekURI,
// authenticating the same URI-bound AAD, and returns the serialized cleartext
// keyset (in-memory only — the same transient shape GenerateDEK produces). A
// URI the client does not support (e.g. a retired KEK dropped from config) is a
// hard error, not a silent miss — the secret sealed under that DEK is
// unreadable and the caller must know.
func (k *Keyring) UnwrapDEK(wrapped []byte, kekURI string) ([]byte, error) {
	if !k.kms.Supported(kekURI) {
		return nil, fmt.Errorf("secrets: unknown kek %q; cannot unwrap dek", kekURI)
	}
	kekAEAD, err := k.kms.GetAEAD(kekURI)
	if err != nil {
		return nil, fmt.Errorf("secrets: unwrap dek: %w", err)
	}

	handle, err := keyset.ReadWithAssociatedData(keyset.NewBinaryReader(bytes.NewReader(wrapped)), kekAEAD, DEKWrapAAD(kekURI))
	if err != nil {
		return nil, fmt.Errorf("secrets: unwrap dek: %w", err)
	}

	dek, err := serializeDEKKeyset(handle)
	if err != nil {
		return nil, fmt.Errorf("secrets: unwrap dek: %w", err)
	}
	return dek, nil
}
