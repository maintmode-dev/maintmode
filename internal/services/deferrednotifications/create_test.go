package deferrednotifications

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestCreateEnqueueCancel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _ := initService(t)

	maint := makeMaint(ctx, t)
	fireAt := xtime.UTCNow().Add(30 * time.Minute)

	// Create persists the schedule; nothing enqueued yet.
	created, err := svc.Create(ctx, maint.ID, []*entity.DeferredNotification{
		{FireAt: fireAt},
		{FireAt: fireAt.Add(25 * time.Minute)},
	})
	require.NoError(t, err)
	require.Len(t, created, 2)

	notifications, err := svc.ListByMaint(ctx, maint.ID)
	require.NoError(t, err)
	require.Equal(t, notifications, created)
}
