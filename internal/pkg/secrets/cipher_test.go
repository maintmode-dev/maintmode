package secrets

import (
	"bytes"
	"encoding/hex"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAESCipher_EncryptDecryptRoundTrip(t *testing.T) {
	t.Parallel()
	cipher := NewAESCipher()
	dek := testDEK(t)

	cases := []struct {
		name      string
		plaintext []byte
	}{
		{"typical secret", []byte("xoxb-slack-bot-token")},
		{"empty", nil},
		{"binary", []byte{0x00, 0x01, 0xff, 0x7f, 0x80}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			envelope, err := cipher.Encrypt(dek, tc.plaintext, testAAD)
			require.NoError(t, err)

			// Envelope carries Tink's prefix + IV, never the bare plaintext.
			require.Greater(t, len(envelope), tinkPrefixLen)
			if len(tc.plaintext) > 0 {
				require.NotContains(t, string(envelope), string(tc.plaintext))
			}

			got, err := cipher.Decrypt(dek, envelope, testAAD)
			require.NoError(t, err)
			// An empty secret may open to nil or []byte{}; only content matters.
			if len(tc.plaintext) == 0 {
				require.Empty(t, got)
			} else {
				require.Equal(t, tc.plaintext, got)
			}
		})
	}
}

func TestAESCipher_NonceIsRandomPerEncrypt(t *testing.T) {
	t.Parallel()
	cipher := NewAESCipher()
	dek := testDEK(t)
	plaintext := []byte("same input twice")

	first, err := cipher.Encrypt(dek, plaintext, testAAD)
	require.NoError(t, err)
	second, err := cipher.Encrypt(dek, plaintext, testAAD)
	require.NoError(t, err)

	// A fresh random IV per call means identical plaintext yields distinct
	// envelopes; equal envelopes would signal nonce reuse (a GCM break). The IV
	// sits right after Tink's 5-byte prefix.
	require.False(t, bytes.Equal(first, second), "envelopes must differ (random nonce per encrypt)")
	require.False(t, bytes.Equal(first[tinkPrefixLen:tinkPrefixLen+12], second[tinkPrefixLen:tinkPrefixLen+12]),
		"IVs must differ")
}

func TestAESCipher_TamperedCiphertextFails(t *testing.T) {
	t.Parallel()
	cipher := NewAESCipher()
	dek := testDEK(t)

	envelope, err := cipher.Encrypt(dek, []byte("api_token"), testAAD)
	require.NoError(t, err)

	// Flip a bit in the ciphertext/tag region (past the prefix and IV).
	tampered := bytes.Clone(envelope)
	tampered[len(tampered)-1] ^= 0x01

	_, err = cipher.Decrypt(dek, tampered, testAAD)
	require.Error(t, err, "GCM must reject tampered ciphertext")
}

func TestAESCipher_TamperedPrefixFails(t *testing.T) {
	t.Parallel()
	cipher := NewAESCipher()
	dek := testDEK(t)

	envelope, err := cipher.Encrypt(dek, []byte("api_token"), testAAD)
	require.NoError(t, err)

	// Corrupt Tink's key-id prefix: the keyset has no key under the altered id,
	// so decryption must fail rather than fall back.
	tampered := bytes.Clone(envelope)
	tampered[1] ^= 0xff

	_, err = cipher.Decrypt(dek, tampered, testAAD)
	require.Error(t, err, "a corrupted key-id prefix must be rejected")
}

func TestAESCipher_WrongDEKFails(t *testing.T) {
	t.Parallel()
	cipher := NewAESCipher()

	envelope, err := cipher.Encrypt(testDEK(t), []byte("password"), testAAD)
	require.NoError(t, err)

	_, err = cipher.Decrypt(testDEK(t), envelope, testAAD)
	require.Error(t, err, "decrypt with a different DEK-keyset must fail")
}

// A mismatched AAD must fail authentication — this is the whole point of
// binding the envelope to its slot: a ciphertext resealed for one (kind, key)
// cannot be opened as another (e.g. bot_token <-> a second secret key, or moved
// to a different kind).
func TestAESCipher_WrongAADFails(t *testing.T) {
	t.Parallel()
	cipher := NewAESCipher()
	dek := testDEK(t)

	envelope, err := cipher.Encrypt(dek, []byte("xoxb-secret"), SecretAAD("slack", "bot_token"))
	require.NoError(t, err)

	// Same key, same envelope, different AAD (different secret key) -> reject.
	_, err = cipher.Decrypt(dek, envelope, SecretAAD("slack", "other_key"))
	require.Error(t, err, "a different secret key in the AAD must fail")

	// Different kind -> reject.
	_, err = cipher.Decrypt(dek, envelope, SecretAAD("telegram", "bot_token"))
	require.Error(t, err, "a different kind in the AAD must fail")

	// A secret envelope must not open under a DEK-wrap AAD (domain separation).
	_, err = cipher.Decrypt(dek, envelope, DEKWrapAAD("kek-1"))
	require.Error(t, err, "domain separation: secret envelope must not open as a DEK wrap")

	// The correct AAD still opens it.
	got, err := cipher.Decrypt(dek, envelope, SecretAAD("slack", "bot_token"))
	require.NoError(t, err)
	require.Equal(t, []byte("xoxb-secret"), got)
}

// encodeAAD length-prefixes each field so distinct field splits cannot collide:
// SecretAAD("a|b","c") must differ from SecretAAD("a","b|c").
func TestSecretAAD_NoDelimiterCollision(t *testing.T) {
	t.Parallel()
	require.NotEqual(t, SecretAAD("a|b", "c"), SecretAAD("a", "b|c"),
		"length-prefixing must prevent a delimiter-split collision")
	require.NotEqual(t, SecretAAD("slack", "bot_token"), DEKWrapAAD("slack"),
		"the domain prefix must separate secret and dek-wrap AADs")
}

func TestAESCipher_MalformedEnvelopeFails(t *testing.T) {
	t.Parallel()
	cipher := NewAESCipher()
	dek := testDEK(t)

	// Too short to carry Tink's prefix, let alone an IV and tag.
	_, err := cipher.Decrypt(dek, []byte{0x01, 0x02, 0x03}, testAAD)
	require.Error(t, err)
}

// A raw 32-byte key is the retired v1 DEK format. The hard cut means it must be
// rejected as a malformed keyset — never silently accepted as key material.
func TestAESCipher_RejectsRawV1DEK(t *testing.T) {
	t.Parallel()
	cipher := NewAESCipher()
	rawKey := bytes.Repeat([]byte{0x42}, 32)

	_, err := cipher.Encrypt(rawKey, []byte("x"), testAAD)
	require.Error(t, err, "a raw v1 DEK must be rejected; v2 DEKs are serialized Tink keysets")

	_, err = cipher.Decrypt(rawKey, []byte("whatever"), testAAD)
	require.Error(t, err)
}

// TestAESCipher_DecryptGoldenVector pins the on-disk format AND the AAD binding.
// The DEK-keyset and envelope below were produced once by this implementation
// (format v2: Tink AES-256-GCM keyset, envelope = Tink AEAD output, AAD =
// SecretAAD("slack","bot_token")) and are hardcoded so that a future refactor —
// switching the key template, changing how the keyset is serialized, or altering
// the AAD derivation — fails loudly here instead of silently making every
// already-stored secret undecryptable. Do NOT regenerate this fixture to make a
// changed layout pass; that would defeat its purpose. It was legitimately
// regenerated when the format moved to Tink (v2), before any production data
// existed — the second and final pre-production format change (v1 introduced
// AAD).
func TestAESCipher_DecryptGoldenVector(t *testing.T) {
	t.Parallel()
	dek, err := hex.DecodeString(goldenDEKHex)
	require.NoError(t, err)
	envelope, err := hex.DecodeString(goldenEnvelopeHex)
	require.NoError(t, err)

	got, err := NewAESCipher().Decrypt(dek, envelope, SecretAAD("slack", "bot_token"))
	require.NoError(t, err)
	require.Equal(t, []byte("xoxb-golden-secret"), got)
}

// TestAESCipher_ConcurrentEncryptDecryptIsSafe exercises the package doc's
// "stateless and safe for concurrent use" claim under the race detector.
func TestAESCipher_ConcurrentEncryptDecryptIsSafe(t *testing.T) {
	t.Parallel()
	cipher := NewAESCipher()
	dek := testDEK(t)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			payload := []byte{byte(i), byte(i >> 8), 0xab}
			env, err := cipher.Encrypt(dek, payload, testAAD)
			require.NoError(t, err)
			got, err := cipher.Decrypt(dek, env, testAAD)
			require.NoError(t, err)
			require.Equal(t, payload, got)
		}(i)
	}
	wg.Wait()
}
