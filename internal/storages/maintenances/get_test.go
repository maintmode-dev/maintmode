package maintenances

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := NewStore(db)

	t.Run("ErrNoRows", func(t *testing.T) {
		t.Parallel()

		dbMaint, err := store.Get(ctx, xuuid.New())
		require.EqualError(t, err, apperr.ErrMaintNotFound.Error())
		require.Nil(t, dbMaint)
	})
}
