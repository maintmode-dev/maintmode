package xjwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

const testKID = "kid-local"

func newTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return key
}

// signTestToken mirrors the production signing path: ES256 with the kid in the header.
func signTestToken(t *testing.T, key *ecdsa.PrivateKey, kid string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Subject:   "alice@example.com",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	token.Header["kid"] = kid

	signed, err := token.SignedString(key)
	require.NoError(t, err)

	return signed
}

func TestNewLocalKeyFunc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("resolves the key by kid", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)

		kf, err := NewLocalKeyFunc(ctx, &key.PublicKey, testKID)
		require.NoError(t, err)

		resolved, err := kf.KeyfuncCtx(ctx)(mustParseUnverified(t, signTestToken(t, key, testKID)))
		require.NoError(t, err)

		pub, ok := resolved.(*ecdsa.PublicKey)
		require.True(t, ok, "keyfunc returned %T, want *ecdsa.PublicKey", resolved)
		require.True(t, pub.Equal(&key.PublicKey), "resolved key is not the public half that was passed in")
	})

	t.Run("verifies a token signed by that key", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)

		kf, err := NewLocalKeyFunc(ctx, &key.PublicKey, testKID)
		require.NoError(t, err)

		parsed, err := jwt.Parse(
			signTestToken(t, key, testKID),
			kf.KeyfuncCtx(ctx),
			jwt.WithValidMethods([]string{"ES256"}),
		)
		require.NoError(t, err)
		require.True(t, parsed.Valid)
	})

	t.Run("unknown kid is a key-not-found error", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)

		kf, err := NewLocalKeyFunc(ctx, &key.PublicKey, testKID)
		require.NoError(t, err)

		_, err = kf.KeyfuncCtx(ctx)(mustParseUnverified(t, signTestToken(t, key, "kid-other")))
		require.ErrorIs(t, err, jwkset.ErrKeyNotFound)
	})

	t.Run("rejects a token signed by a different key", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)

		kf, err := NewLocalKeyFunc(ctx, &key.PublicKey, testKID)
		require.NoError(t, err)

		_, err = jwt.Parse(
			signTestToken(t, newTestKey(t), testKID),
			kf.KeyfuncCtx(ctx),
			jwt.WithValidMethods([]string{"ES256"}),
		)
		require.ErrorIs(t, err, jwt.ErrTokenSignatureInvalid)
	})

	// The whole point of the local path: no HTTP client, no refresh, no listener
	// anywhere in this test binary — the storage is the in-memory one and holds
	// exactly the single key it was given.
	t.Run("has no network path", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)

		kf, err := NewLocalKeyFunc(ctx, &key.PublicKey, testKID)
		require.NoError(t, err)

		storage := kf.Storage()
		require.IsType(t, &jwkset.MemoryJWKSet{}, storage)

		keys, err := storage.KeyReadAll(ctx)
		require.NoError(t, err)
		require.Len(t, keys, 1)

		pub, ok := keys[0].Key().(*ecdsa.PublicKey)
		require.True(t, ok, "stored key is %T, want *ecdsa.PublicKey", keys[0].Key())
		require.True(t, pub.Equal(&key.PublicKey))
		require.Equal(t, testKID, keys[0].Marshal().KID)
	})
}

func mustParseUnverified(t *testing.T, signed string) *jwt.Token {
	t.Helper()

	token, _, err := jwt.NewParser().ParseUnverified(signed, jwt.MapClaims{})
	require.NoError(t, err)

	return token
}
