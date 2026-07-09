package secrets

import (
	"encoding/binary"
)

// AAD (additional authenticated data) domains. The version suffix is part of
// the authenticated bytes, so the on-disk format is self-describing: a future
// format bumps the domain and old envelopes fail to open under it — a hard
// error, never a silent fallback. v2 = Tink formats (DEK is a serialized Tink
// keyset, the secret envelope is Tink AEAD output); v1 was the hand-rolled
// [nonce][ct+tag] layout, retired before any production data existed.
const (
	// #nosec G101 -- not a credential: this is a public AAD domain-separation
	// label authenticated (not secret) alongside each envelope.
	aadDomainSecret  = "maintmode/integration-secret/v2"
	aadDomainDEKWrap = "maintmode/dek-wrap/v2"
	// aadDomainDEKVerify binds the in-memory equivalence probe (EquivalentDEKs);
	// its envelopes are never persisted, the domain only keeps them from ever
	// opening as a real secret or DEK wrap.
	aadDomainDEKVerify = "maintmode/dek-verify/v2"
)

// SecretAAD binds a secret envelope to its logical slot: the integration kind
// plus the secret key. kind is UNIQUE + immutable and secretKey identifies the
// field, so a ciphertext moved to another row (different kind) or another key in
// the same row (e.g. bot_token <-> a second secret) fails the GCM tag check. Only
// stable identifiers go in — never mutable state (enabled/config/updated_at) —
// because mergeSecrets carries an unchanged ciphertext forward as-is (no
// re-encrypt), so its AAD must not change on an unrelated edit.
func SecretAAD(kind, secretKey string) []byte {
	return encodeAAD(aadDomainSecret, kind, secretKey)
}

// DEKWrapAAD binds a wrapped-DEK envelope to the KEK that wrapped it (kekID), and
// (via the domain string) separates it from a secret envelope so one can't be
// substituted for the other. Rotation is unaffected: a re-wrap produces a fresh
// envelope and the new kekID is known at WrapDEK time, so the AAD is recomputed;
// verify-unwrap after re-wrap uses the same new-kekID AAD.
//
// The row id is deliberately NOT in this AAD. Binding it would only catch swapping
// a wrapped-DEK between two rows under the SAME KEK, which — once secrets are
// bound by kind+key — yields a decrypt failure (porch/DoS), never disclosure, and
// an at-rest writer can already DoS by zeroing a byte. Adding it would force
// app-side id generation and split the datakey.Create contract for no real gain.
func DEKWrapAAD(kekID string) []byte {
	return encodeAAD(aadDomainDEKWrap, kekID)
}

// encodeAAD length-prefixes each field so distinct field splits can never collide
// (a|b + c must differ from a + b|c). Each part is written as a uvarint length
// followed by its bytes; the domain is the first part. uvarint avoids any
// fixed-width length cap while staying unambiguous.
func encodeAAD(domain string, parts ...string) []byte {
	all := append([]string{domain}, parts...)

	size := 0
	for _, p := range all {
		size += binary.MaxVarintLen64 + len(p)
	}

	buf := make([]byte, 0, size)
	for _, p := range all {
		buf = binary.AppendUvarint(buf, uint64(len(p)))
		buf = append(buf, p...)
	}
	return buf
}
