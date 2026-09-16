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
	aadDomainSecret = "maintmode/integration-secret/v2"
	// #nosec G101 -- not a credential: a public AAD domain-separation label,
	// authenticated (not secret) alongside each envelope.
	//
	// aadDomainSecretClient binds a LOGIN provider's secret, which needs more in
	// its AAD than kind+key can carry (see SecretAADForClient). Its own domain,
	// not a v3 bump of the one above: the two coexist permanently rather than
	// one superseding the other, and a domain bump would make every existing
	// Slack/SMTP envelope fail to open.
	aadDomainSecretClient = "maintmode/integration-secret-client/v2"
	aadDomainDEKWrap      = "maintmode/dek-wrap/v2"
	// aadDomainDEKVerify binds the in-memory equivalence probe (EquivalentDEKs);
	// its envelopes are never persisted, the domain only keeps them from ever
	// opening as a real secret or DEK wrap.
	aadDomainDEKVerify = "maintmode/dek-verify/v2"
	// aadDomainOTPCode binds a one-time code sealed for delivery in a queue task.
	// Its own domain keeps such an envelope from ever opening as an integration
	// secret or a wrapped DEK, and vice versa.
	aadDomainOTPCode = "maintmode/otp-code/v2"
)

// SecretAAD binds a secret envelope to its logical slot: the integration kind
// plus the secret key. secretKey identifies the field, so a ciphertext moved to
// another row (different kind) or another key in the same row (e.g. bot_token
// <-> a second secret) fails the GCM tag check. Only stable identifiers go in —
// never mutable state (enabled/config/updated_at) — because mergeSecrets carries
// an unchanged ciphertext forward as-is (no re-encrypt), so its AAD must not
// change on an unrelated edit.
//
// This function is FROZEN. Every integration secret written before login
// providers existed is sealed under it, and it does not re-encrypt on edit, so
// changing these bytes would make every stored Slack/SMTP/Telegram secret
// unreadable with no way back short of re-entering each one by hand. Kinds that
// need more in their AAD get their own function, as SecretAADForClient did.
//
// Note what it no longer implies: kind alone is no longer unique. Two rows of
// one kind can hold ciphertexts that open under each other's AAD. For delivery
// kinds that is harmless — they are single-instance by service convention — but
// it is why login providers do not use this function.
func SecretAAD(kind, secretKey string) []byte {
	return encodeAAD(aadDomainSecret, kind, secretKey)
}

// SecretAADForClient binds a login provider's secret to the OAuth client it was
// issued for: the kind, the secret key, the issuer URL and the client id.
//
// Both config fields are needed, and neither alone would do. config is one
// plaintext jsonb column, so an attacker who can write the table copies whatever
// single field is in the AAD along with the ciphertext and the dek_id:
//
//   - issuer_url alone: copied verbatim, leaving only DNS or TLS for the real
//     issuer host to subvert.
//   - client_id alone: weaker still — it takes no part in routing, so the token
//     request goes wherever the row's issuer_url points, and the secret is gone
//     the moment we POST to that endpoint whatever the far end answers.
//
// Together they close it: the ciphertext opens only on a row that keeps the real
// issuer, so discovery resolves to the genuine IdP, AND only against the real
// client_id, so the registration cannot be swapped underneath it.
//
// The cost is a rule the service must enforce: because mergeSecrets carries an
// unchanged ciphertext forward without re-encrypting, changing either field
// without resupplying the secret would strand it. The service refuses that
// combination rather than letting it produce an unopenable row.
//
// issuerURL is empty for kinds that have no issuer (github_oauth); that is a
// stable value for them and distinct from any real issuer.
func SecretAADForClient(kind, secretKey, issuerURL, clientID string) []byte {
	return encodeAAD(aadDomainSecretClient, kind, secretKey, issuerURL, clientID)
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

// OTPCodeAAD binds a sealed one-time code to the credential row it was issued
// for. The code travels in a queue task while only its hash is in the database,
// so the envelope must not be openable in the context of a different credential:
// a payload edited to carry another row's id fails the GCM tag check instead of
// decrypting.
//
// The DEK sealing an OTP is ephemeral and lives in the same task, so an envelope
// lifted into another task would already fail for want of its key. The AAD is
// not load-bearing against that; what it buys is domain separation from the
// secret and DEK-wrap envelopes, and a binding that still holds if the sealing
// ever stops being per-task.
func OTPCodeAAD(credentialID string) []byte {
	return encodeAAD(aadDomainOTPCode, credentialID)
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
