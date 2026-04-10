package token

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJWKS(t *testing.T) {
	ctx := context.Background()
	svc := initService(t)

	t.Run("ok", func(t *testing.T) {
		jwks := svc.JWKS(ctx)
		require.Equal(t, 1, len(jwks.Keys))

		key := jwks.Keys[0]
		require.Equal(t, "EC", key.Kty)
		require.Equal(t, "P-256", key.Crv)
		require.Equal(t, svc.kid, key.Kid)
		require.Equal(t, "ES256", key.Alg)
		require.Equal(t, "sig", key.Use)
		require.NotEqual(t, key.X, key.Y)
	})
}
