package deferrednotifications

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestDeleteByMaint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	maint := makeMaint(ctx, t)
	_, err := store.CreateMany(ctx, maint.ID, sampleNotifications(xtime.UTCNow()))
	require.NoError(t, err)

	require.NoError(t, store.DeleteByMaint(ctx, maint.ID))

	listed, err := store.ListByMaint(ctx, maint.ID)
	require.NoError(t, err)
	require.Empty(t, listed)
}
