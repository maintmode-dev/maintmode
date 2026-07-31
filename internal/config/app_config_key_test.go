package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// newTestSigningKeyHex returns a freshly generated P-256 key together with its
// hex-encoded raw scalar, in exactly the form the JWT config carries it. It is
// generated rather than hardcoded so the tests carry no dependency on a
// deployment secrets file.
func newTestSigningKeyHex(t *testing.T) (key *ecdsa.PrivateKey, keyHex string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	// P-256 scalars are 32 bytes; FillBytes left-pads a short D so the encoding
	// is always the fixed width ParseRawPrivateKey expects.
	raw := make([]byte, 32)
	key.D.FillBytes(raw)

	return key, hex.EncodeToString(raw)
}

// invalidKeyCases are the malformed signing keys every parse entry point must
// reject. wantErr is a distinguishing substring, so a test cannot pass because
// of some unrelated failure: a decode failure and a curve-parse failure are
// different bugs and must not be conflated.
var invalidKeyCases = []struct {
	name    string
	key     string
	wantErr string
}{
	{name: "non-hex characters", key: "zzzz", wantErr: "failed to decode private key"},
	{name: "hex but far too short", key: "00", wantErr: "failed to parse private key"},
	{
		name: "hex but one byte short of the curve size",
		// 60 hex chars = 30 bytes, short of P-256's 32. Deliberately synthetic:
		// borrowing a prefix of the real dev key would read as a leaked secret.
		key:     "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e",
		wantErr: "failed to parse private key",
	},
	{name: "empty string", key: "", wantErr: "failed to parse private key"},
	{name: "odd number of hex digits", key: "abc", wantErr: "failed to decode private key"},
}

// ParsePrivateKey is the error-returning form of the signing-key parse. It
// exists so a config typo surfaces as an error to callers outside the bootstrap
// path instead of a panic — so "does not panic" is part of its contract and is
// asserted on every malformed input, not just the happy path.
func TestJWT_ParsePrivateKey(t *testing.T) {
	t.Parallel()

	t.Run("valid key parses onto the P-256 curve", func(t *testing.T) {
		t.Parallel()

		want, keyHex := newTestSigningKeyHex(t)

		got, err := JWT{PrivateKey: keyHex}.ParsePrivateKey()
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, elliptic.P256(), got.Curve)
		require.Zero(t, want.D.Cmp(got.D))
		require.True(t, want.PublicKey.Equal(&got.PublicKey))
	})

	for _, tc := range invalidKeyCases {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			t.Parallel()

			var (
				key *ecdsa.PrivateKey
				err error
			)

			require.NotPanics(t, func() {
				key, err = JWT{PrivateKey: tc.key}.ParsePrivateKey()
			})

			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, key)
		})
	}
}

// PublicKey hands a token verifier the key it needs without the private half
// leaking into its package. It must derive exactly the public half of what
// ParsePrivateKey returns, and must propagate — never swallow — a parse error.
func TestJWT_PublicKey(t *testing.T) {
	t.Parallel()

	t.Run("returns the public half of the parsed private key", func(t *testing.T) {
		t.Parallel()

		_, keyHex := newTestSigningKeyHex(t)
		cfg := JWT{PrivateKey: keyHex}

		private, err := cfg.ParsePrivateKey()
		require.NoError(t, err)

		got, err := cfg.PublicKey()
		require.NoError(t, err)
		require.NotNil(t, got)
		// Equal is the curve-aware comparison; DeepEqual would compare unexported
		// big.Int internals and can differ for equal values.
		require.True(t, private.PublicKey.Equal(got))
	})

	t.Run("public halves of two distinct keys are not equal", func(t *testing.T) {
		t.Parallel()

		_, firstHex := newTestSigningKeyHex(t)
		_, secondHex := newTestSigningKeyHex(t)

		first, err := JWT{PrivateKey: firstHex}.PublicKey()
		require.NoError(t, err)

		second, err := JWT{PrivateKey: secondHex}.PublicKey()
		require.NoError(t, err)

		require.False(t, first.Equal(second))
	})

	for _, tc := range invalidKeyCases {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			t.Parallel()

			var (
				key *ecdsa.PublicKey
				err error
			)

			require.NotPanics(t, func() {
				key, err = JWT{PrivateKey: tc.key}.PublicKey()
			})

			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, key)
		})
	}
}

// GeneratePrivateKey is the panicking wrapper used by the bootstrap path, which
// has nowhere to return an error to: an unusable signing key must stop the
// process from coming up. The panic on bad input is therefore the contract, and
// the happy path must agree with ParsePrivateKey since it only wraps it.
func TestJWT_GeneratePrivateKey(t *testing.T) {
	t.Parallel()

	t.Run("valid key matches ParsePrivateKey", func(t *testing.T) {
		t.Parallel()

		_, keyHex := newTestSigningKeyHex(t)
		cfg := JWT{PrivateKey: keyHex}

		want, err := cfg.ParsePrivateKey()
		require.NoError(t, err)

		var got *ecdsa.PrivateKey
		require.NotPanics(t, func() {
			got = cfg.GeneratePrivateKey()
		})

		require.NotNil(t, got)
		require.Zero(t, want.D.Cmp(got.D))
		require.True(t, want.PublicKey.Equal(&got.PublicKey))
	})

	for _, tc := range invalidKeyCases {
		t.Run("panics on "+tc.name, func(t *testing.T) {
			t.Parallel()

			require.Panics(t, func() {
				_ = JWT{PrivateKey: tc.key}.GeneratePrivateKey()
			})
		})
	}
}
