package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
)

func TestGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := refreshtoken.NewStore(db)

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		dbToken, err := store.GetByTokenHash(ctx, "some token hash")
		require.Nil(t, dbToken)
		require.EqualError(t, err, apperr.ErrRefreshTokenNotFound.Error())
	})
}
