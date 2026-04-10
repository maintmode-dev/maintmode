package token

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

func TestIssueRefreshToken(t *testing.T) {
	ctx := context.Background()
	srv := initService(t)

	t.Run("ok", func(t *testing.T) {
		raw, hashed, err := srv.GenerateRefreshToken(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, raw)
		require.NotEmpty(t, hashed)
		require.NotEqual(t, raw, hashed)

		h := xhash.HashSha256([]byte(raw))
		require.Equal(t, hashed, h)
	})

	t.Run("unique", func(t *testing.T) {
		raw1, hashed1, err := srv.GenerateRefreshToken(ctx)
		require.NoError(t, err)

		raw2, hashed2, err := srv.GenerateRefreshToken(ctx)
		require.NoError(t, err)

		require.NotEqual(t, raw1, raw2)
		require.NotEqual(t, hashed1, hashed2)
	})
}
