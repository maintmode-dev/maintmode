package deferrednotifications

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestSetTaskIDAndListTaskIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	maint := makeMaint(ctx, t)
	created, err := store.CreateMany(ctx, maint.ID, sampleNotifications(xtime.UTCNow()))
	require.NoError(t, err)

	// no task ids yet
	notificationsBefore, err := store.ListByMaint(ctx, maint.ID)
	require.NoError(t, err)
	require.NotEmpty(t, notificationsBefore)
	for _, n := range notificationsBefore {
		require.Nil(t, n.GoqueTaskID)
		require.False(t, n.IsScheduled())
	}

	// set a task id on each reminder
	want := make([]uuid.UUID, 0, len(created))
	for _, n := range created {
		taskID := xuuid.New()
		require.NoError(t, store.SetTaskID(ctx, n.ID, taskID))
		want = append(want, taskID)
	}

	notificationsAfter, err := store.ListByMaint(ctx, maint.ID)
	require.NoError(t, err)
	require.NotEmpty(t, notificationsAfter)
	for _, n := range notificationsAfter {
		require.NotNil(t, n.GoqueTaskID)
		require.Contains(t, want, lo.FromPtr(n.GoqueTaskID))
		require.True(t, n.IsScheduled())
	}
}
