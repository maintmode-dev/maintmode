package statecodec

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestSignedStateCodec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		codec := initService(t)

		encoded, err := codec.Encode(ctx, &entity.OAuthState{
			Nonce:       "n1",
			OriginalURI: "/dashboard",
		})
		require.NoError(t, err)
		require.Contains(t, encoded, ".")

		got, err := codec.Decode(ctx, encoded)
		require.NoError(t, err)
		require.Equal(t, "n1", got.Nonce)
		require.Equal(t, "/dashboard", got.OriginalURI)
		require.Greater(t, got.ExpiresAt, xtime.UTCNow().Unix())
	})

	t.Run("tampered", func(t *testing.T) {
		t.Parallel()
		codec := initService(t)

		encoded, err := codec.Encode(ctx, &entity.OAuthState{
			Nonce:       "n",
			OriginalURI: "/",
		})
		require.NoError(t, err)

		t.Run("change payload", func(t *testing.T) {
			t.Parallel()

			encPayload, encSig, err := extractParts(encoded)
			require.NoError(t, err)

			// Flip a character in payload — signature won't match.
			tamperedPayload := "X" + encPayload[1:]
			_, err = codec.Decode(ctx, tamperedPayload+"."+encSig)
			require.ErrorIs(t, err, apperr.ErrOAuthStateTampered)
		})

		t.Run("change sign", func(t *testing.T) {
			t.Parallel()

			encPayload, encSig, err := extractParts(encoded)
			require.NoError(t, err)

			// Flip a character in signature.
			tamperedSig := "X" + encSig[1:]
			_, err = codec.Decode(ctx, encPayload+"."+tamperedSig)
			require.ErrorIs(t, err, apperr.ErrOAuthStateTampered)
		})
	})

	t.Run("expired", func(t *testing.T) {
		t.Parallel()
		frozen := time.Unix(1_700_000_000, 0)

		codec := initService(t)
		codec.nowFn = func() time.Time { return frozen }

		encoded, err := codec.Encode(ctx, &entity.OAuthState{Nonce: "n", OriginalURI: "/"})
		require.NoError(t, err)

		codecLater := initService(t)
		_, err = codecLater.Decode(ctx, encoded)
		require.ErrorIs(t, err, apperr.ErrOAuthStateExpired)
	})

	t.Run("bad format", func(t *testing.T) {
		t.Parallel()
		codec := initService(t)

		for name, raw := range map[string]string{
			"empty":      "",
			"no dot":     "abc",
			"too many":   "a.b.c",
			"non base64": "not-base64!.also-not!",
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				_, err := codec.Decode(ctx, raw)
				require.ErrorIs(t, err, apperr.ErrOAuthStateTampered)
			})
		}
	})

	t.Run("different secret rejected", func(t *testing.T) {
		t.Parallel()

		codec1 := initService(t)
		codec2 := initService(t)
		codec2.secret = append([]byte("different"), codec2.secret...)

		encoded, err := codec1.Encode(ctx, &entity.OAuthState{Nonce: "n", OriginalURI: "/"})
		require.NoError(t, err)

		_, err = codec2.Decode(ctx, encoded)
		require.ErrorIs(t, err, apperr.ErrOAuthStateTampered)
	})
}
