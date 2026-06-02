package deferrednotifications

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestCancel(t *testing.T) {
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

	//Enqueue schedules one reminder task per reminder and records task ids.
	require.NoError(t, svc.Schedule(ctx, maint.ID))

	// Cancel cancels every recorded task.
	require.NoError(t, svc.Cancel(ctx, maint.ID))
}
