package notifytargets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	maint := makeMaint(ctx, t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		makeNotifyTargets(ctx, t, store, maint.ID)

		err := store.Delete(ctx, maint.ID)
		require.NoError(t, err)

		stored, err := store.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Empty(t, stored)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		err := store.Delete(ctx, xuuid.New())
		require.NoError(t, err)
	})
}
