package deferrednotifications

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestSchedule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, mocks := initService(t)

	maint, createdNotifications := makeNotifications(ctx, t, svc)

	mocks.scheduler.EXPECT().
		ScheduleDelayed(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(xuuid.New(), nil).
		Times(len(createdNotifications))

	require.NoError(t, svc.Schedule(ctx, maint.ID))

	notifications, err := svc.ListByMaint(ctx, maint.ID)
	require.NoError(t, err)
	require.Equal(t, len(notifications), len(createdNotifications))
	notificationsM := lo.SliceToMap(notifications, func(item *entity.DeferredNotification) (uuid.UUID, *entity.DeferredNotification) {
		return item.ID, item
	})
	for _, created := range createdNotifications {
		notification, ok := notificationsM[created.ID]
		require.True(t, ok, "not found notification with id %s", created.ID)
		require.True(t, notification.IsScheduled())
	}
}
