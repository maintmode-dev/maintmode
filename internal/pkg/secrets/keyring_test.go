package secrets

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tink-crypto/tink-go/v2/testing/fakekms"
)

func TestGenerateDEK(t *testing.T) {
	t.Parallel()
	first, err := GenerateDEK()
	require.NoError(t, err)
	require.NotEmpty(t, first, "DEK must be a non-empty serialized Tink keyset")

	second, err := GenerateDEK()
	require.NoError(t, err)
	require.NotEqual(t, first, second, "each DEK-keyset must be independently random")

	// A generated DEK must be immediately usable to seal/open a secret.
	aad := SecretAAD("slack", "bot_token")
	env, err := NewAESCipher().Encrypt(first, []byte("secret"), aad)
	require.NoError(t, err)
	got, err := NewAESCipher().Decrypt(first, env, aad)
	require.NoError(t, err)
	require.Equal(t, []byte("secret"), got)
}

func TestNewKeyring_WrapUnwrapRoundTrip(t *testing.T) {
	t.Parallel()
	kr, err := NewLocalKeyring(kekURI(1), map[string]string{kekURI(1): hexKey(1)})
	require.NoError(t, err)
	require.Equal(t, kekURI(1), kr.ActiveKEKID())

	dek := testDEK(t)
	wrapped, kekID, err := kr.WrapDEK(dek)
	require.NoError(t, err)
	require.Equal(t, kekURI(1), kekID)
	require.NotEqual(t, dek, wrapped)

	got, err := kr.UnwrapDEK(wrapped, kekID)
	require.NoError(t, err)
	// The unwrapped keyset must open what the original DEK sealed — functional
	// identity, not byte identity: proto re-serialization is not contractually
	// canonical, so the bytes may legitimately differ.
	requireSameDEK(t, dek, got)
}

// TestKeyring_UnwrapGoldenWrappedDEK pins the wrapped-DEK on-disk format (see
// golden_test.go): a change to the keyset wrap serialization or the
// DEKWrapAAD derivation makes every stored data_keys row unreadable, and must
// fail here loudly instead. Same policy as the secret-envelope vector: do NOT
// regenerate to make a changed format pass.
func TestKeyring_UnwrapGoldenWrappedDEK(t *testing.T) {
	t.Parallel()
	kr, err := NewLocalKeyring(goldenKEKURI, map[string]string{goldenKEKURI: goldenKEKHex})
	require.NoError(t, err)

	wrapped, err := hex.DecodeString(goldenWrappedDEKHex)
	require.NoError(t, err)
	goldenDEK, err := hex.DecodeString(goldenDEKHex)
	require.NoError(t, err)

	got, err := kr.UnwrapDEK(wrapped, goldenKEKURI)
	require.NoError(t, err)
	requireSameDEK(t, goldenDEK, got)
}

func TestEquivalentDEKs_DetectsDifferentKeyMaterial(t *testing.T) {
	t.Parallel()
	a, b := testDEK(t), testDEK(t)
	require.NoError(t, EquivalentDEKs(a, a))
	require.Error(t, EquivalentDEKs(a, b), "distinct DEK-keysets must not verify as equivalent")
}

func TestNewKeyring_RetiredKEKStillUnwraps(t *testing.T) {
	t.Parallel()
	// A DEK wrapped under kek-1 must still unwrap after kek-2 becomes active,
	// as long as kek-1 stays resident — this is what makes rotation seamless.
	old, err := NewLocalKeyring(kekURI(1), map[string]string{kekURI(1): hexKey(1)})
	require.NoError(t, err)

	dek := testDEK(t)
	wrapped, kekID, err := old.WrapDEK(dek)
	require.NoError(t, err)

	rotated, err := NewLocalKeyring(kekURI(2), map[string]string{
		kekURI(1): hexKey(1), // retired but retained
		kekURI(2): hexKey(2), // new active
	})
	require.NoError(t, err)
	require.Equal(t, kekURI(2), rotated.ActiveKEKID())

	got, err := rotated.UnwrapDEK(wrapped, kekID)
	require.NoError(t, err)
	requireSameDEK(t, dek, got)
}

func TestNewKeyring_UnknownKEKFailsUnwrap(t *testing.T) {
	t.Parallel()
	kr, err := NewLocalKeyring(kekURI(1), map[string]string{kekURI(1): hexKey(1)})
	require.NoError(t, err)

	wrapped, _, err := kr.WrapDEK(testDEK(t))
	require.NoError(t, err)

	_, err = kr.UnwrapDEK(wrapped, "local-kms://kek-gone")
	require.Error(t, err, "an unknown kek uri must be a hard error, not a silent miss")
	require.Contains(t, err.Error(), "unknown kek")
	require.False(t, kr.Knows("local-kms://kek-gone"))
	require.True(t, kr.Knows(kekURI(1)))
}

func TestKeyring_UnwrapDEKRejectsCorruptWrapped(t *testing.T) {
	t.Parallel()
	kr, err := NewLocalKeyring(kekURI(1), map[string]string{kekURI(1): hexKey(1)})
	require.NoError(t, err)

	wrapped, kekID, err := kr.WrapDEK(testDEK(t))
	require.NoError(t, err)

	t.Run("tampered bytes", func(t *testing.T) {
		t.Parallel()
		tampered := bytes.Clone(wrapped)
		tampered[len(tampered)-1] ^= 0x01
		_, err := kr.UnwrapDEK(tampered, kekID)
		require.Error(t, err, "a corrupt wrapped DEK must fail authentication, not return garbage")
		require.Contains(t, err.Error(), "unwrap dek")
	})

	t.Run("truncated", func(t *testing.T) {
		t.Parallel()
		_, err := kr.UnwrapDEK(wrapped[:5], kekID)
		require.Error(t, err, "a truncated wrapped DEK must error, not panic")
	})
}

// The wrapped-DEK AAD binds the KEK URI: the same wrapped bytes must not unwrap
// under a different URI even if that URI serves the SAME key material.
func TestKeyring_WrappedDEKBoundToKEKURI(t *testing.T) {
	t.Parallel()
	sameKey := hexKey(7)
	kr, err := NewLocalKeyring(kekURI(1), map[string]string{
		kekURI(1): sameKey,
		kekURI(2): sameKey, // identical key bytes under a second URI
	})
	require.NoError(t, err)

	wrapped, _, err := kr.WrapDEK(testDEK(t))
	require.NoError(t, err)

	_, err = kr.UnwrapDEK(wrapped, kekURI(2))
	require.Error(t, err, "AAD must bind the wrap to its KEK URI, not just the key bytes")
}

func TestNewKeyring_FailFast(t *testing.T) {
	t.Parallel()
	valid := map[string]string{kekURI(1): hexKey(1)}

	t.Run("active uri not in local keys", func(t *testing.T) {
		t.Parallel()
		_, err := NewLocalKeyring(kekURI(2), valid)
		require.ErrorContains(t, err, "not supported")
	})

	t.Run("active kek fails aead probe", func(t *testing.T) {
		t.Parallel()
		// A prefix-only client claims Supported for any fake-kms:// URI, but
		// GetAEAD fails on this one (the URI body is not a base64 keyset) — so
		// this exercises the constructor's AEAD probe, not the Supported check.
		cl, err := fakekms.NewClient("fake-kms://")
		require.NoError(t, err)

		const badURI = "fake-kms://not-a-base64-keyset!"
		require.True(t, cl.Supported(badURI), "precondition: the probe branch, not Supported, must reject")
		_, err = NewKeyring(badURI, cl)
		require.ErrorContains(t, err, "secrets: active kek")
	})
}

func TestNewLocalKMSClient_FailFast(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		keys    map[string]string
		wantErr string
	}{
		{"no keks", map[string]string{}, "no local keks configured"},
		{"wrong scheme", map[string]string{"kek-1": hexKey(1)}, "must use the local-kms:// scheme"},
		{"non-hex key", map[string]string{kekURI(1): "zzzz"}, "not valid hex"},
		{"short key", map[string]string{kekURI(1): hex.EncodeToString(make([]byte, 16))}, "must decode to 32 bytes"},
		{"all-zero key", map[string]string{kekURI(1): hex.EncodeToString(make([]byte, keyLen))}, "all-zero"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, err := NewLocalKMSClient(tc.keys)
			require.Error(t, err, "must fail fast, not build a client")
			require.Nil(t, client)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestNewLocalKMSClient_FailFastNeverLeaksKeyBytes(t *testing.T) {
	t.Parallel()
	// A too-short key must not appear (even partially) in the error text: fail-fast
	// messages surface in logs, and key material must never reach them.
	secret := hex.EncodeToString(make([]byte, 16))
	_, err := NewLocalKMSClient(map[string]string{kekURI(1): secret})
	require.Error(t, err)
	require.NotContains(t, strings.ToLower(err.Error()), secret)
}

func TestKeyring_WrapDEKRejectsEmpty(t *testing.T) {
	t.Parallel()
	kr, err := NewLocalKeyring(kekURI(1), map[string]string{kekURI(1): hexKey(1)})
	require.NoError(t, err)

	_, _, err = kr.WrapDEK(nil)
	require.Error(t, err, "an empty DEK-keyset must be rejected")
}

// TestKeyring_KMSClientIsSwappable proves the KMS bridge: the keyring works
// identically over any registry.KMSClient, so moving the KEK to a cloud KMS is
// a client registration (gcp-kms://…), not a crypto-code change. The fake here
// stands in for such a provider.
func TestKeyring_KMSClientIsSwappable(t *testing.T) {
	t.Parallel()
	// fakekms carries the key material inside the URI itself
	// (fake-kms://<base64 keyset>), so a valid URI must be generated by
	// NewKeyURI, not hand-written.
	uri, err := fakekms.NewKeyURI()
	require.NoError(t, err)

	cl, err := fakekms.NewClient(uri)
	require.NoError(t, err)

	kr, err := NewKeyring(uri, cl)
	require.NoError(t, err)

	dek := testDEK(t)
	wrapped, kekID, err := kr.WrapDEK(dek)
	require.NoError(t, err)
	require.Equal(t, uri, kekID)

	got, err := kr.UnwrapDEK(wrapped, kekID)
	require.NoError(t, err)
	requireSameDEK(t, dek, got)
}
