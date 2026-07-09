package secrets

import (
	"bytes"
	"fmt"

	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"
)

// GenerateDEK returns a fresh DEK as a serialized cleartext Tink keyset built
// from the AES-256-GCM template. The caller wraps it with a Keyring before
// persisting; the cleartext keyset lives only in memory (see dekAEAD for why the
// insecurecleartextkeyset use is safe here).
func GenerateDEK() ([]byte, error) {
	handle, err := keyset.NewHandle(aead.AES256GCMKeyTemplate())
	if err != nil {
		return nil, fmt.Errorf("secrets: generate dek: %w", err)
	}

	dek, err := serializeDEKKeyset(handle)
	if err != nil {
		return nil, fmt.Errorf("secrets: generate dek: %w", err)
	}
	return dek, nil
}

// serializeDEKKeyset renders a DEK-keyset handle to the transient cleartext
// byte form that GenerateDEK returns and the SecretCipher consumes.
func serializeDEKKeyset(handle *keyset.Handle) ([]byte, error) {
	var buf bytes.Buffer
	if err := insecurecleartextkeyset.Write(handle, keyset.NewBinaryWriter(&buf)); err != nil {
		return nil, fmt.Errorf("serialize dek keyset: %w", err)
	}
	return buf.Bytes(), nil
}

// EquivalentDEKs verifies two serialized DEK-keysets carry the same key
// material by functional probe, in BOTH directions: what either keyset seals,
// the other must open. One direction alone would only prove the sealer's
// primary key is present in the opener — a keyset that gained or lost
// non-primary keys would still pass. Rotation uses this to verify a re-wrapped
// DEK before overwriting the stored envelope. A probe is the right check —
// proto re-serialization is not contractually canonical, so byte equality of
// keyset bytes could reject a correct re-wrap (or be quietly relied on until a
// tink-go upgrade breaks it).
func EquivalentDEKs(a, b []byte) error {
	// Inner errors already carry the secrets: prefix; only add direction here.
	if err := probeDEKOpens(a, b); err != nil {
		return fmt.Errorf("dek equivalence (a->b): %w", err)
	}
	if err := probeDEKOpens(b, a); err != nil {
		return fmt.Errorf("dek equivalence (b->a): %w", err)
	}
	return nil
}

// probeDEKOpens seals a probe under sealer and opens it under opener, under
// the dedicated verify AAD domain so probe envelopes can never open as a real
// secret or DEK wrap.
func probeDEKOpens(sealer, opener []byte) error {
	probe := []byte("dek-equivalence-probe")
	aad := encodeAAD(aadDomainDEKVerify)

	env, err := AESCipher{}.Encrypt(sealer, probe, aad)
	if err != nil {
		return err
	}
	got, err := AESCipher{}.Decrypt(opener, env, aad)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, probe) {
		return fmt.Errorf("secrets: dek probe mismatch")
	}
	return nil
}

// parseDEKKeyset and serializeDEKKeyset are the ONLY two places the
// insecurecleartextkeyset API is touched — every other function goes through
// them. The cleartext keyset bytes they handle are transient: produced by
// GenerateDEK or unwrapped by the Keyring, used in memory, and dropped — never
// persisted or logged. That in-memory-only use is exactly what the insecure*
// package gates; keeping both call-sites in one file keeps the claim auditable
// at a glance.

// parseDEKKeyset deserializes a cleartext DEK-keyset. A dek that is not a valid
// serialized keyset (including a raw 32-byte format-v1 key) is rejected here.
func parseDEKKeyset(dek []byte) (*keyset.Handle, error) {
	handle, err := insecurecleartextkeyset.Read(keyset.NewBinaryReader(bytes.NewReader(dek)))
	if err != nil {
		return nil, fmt.Errorf("read dek keyset: %w", err)
	}
	return handle, nil
}
