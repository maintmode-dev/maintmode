package deferrednotifications

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestReplace_CancelsAlreadyEnqueued(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, mocks := initService(t)

	maint, createdNotifications := makeNotifications(ctx, t, svc)

	mocks.scheduler.EXPECT().
		ScheduleDelayed(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(xuuid.New(), nil).
		Times(len(createdNotifications))
	mocks.scheduler.EXPECT().
		Cancel(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(len(createdNotifications))

	require.NoError(t, svc.Schedule(ctx, maint.ID))

	// Replacing the schedule must cancel the previously enqueued task.
	err := svc.Replace(ctx, maint.ID, []*entity.DeferredNotification{
		{FireAt: xtime.UTCNow().Add(2 * time.Hour)},
	})
	require.NoError(t, err)

	// Old rows gone, new one present without a task id yet.
	notifications, err := svc.ListByMaint(ctx, maint.ID)
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	for _, n := range notifications {
		require.False(t, n.IsScheduled())
	}

	// schedule a new one
	mocks.scheduler.EXPECT().
		ScheduleDelayed(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(xuuid.New(), nil).
		Times(1)

	require.NoError(t, svc.Schedule(ctx, maint.ID))

	notifications, err = svc.ListByMaint(ctx, maint.ID)
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	for _, n := range notifications {
		require.True(t, n.IsScheduled())
	}
}
