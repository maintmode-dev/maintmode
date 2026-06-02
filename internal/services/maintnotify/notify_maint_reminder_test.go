package maintnotify

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestNotifyMaintReminder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("sends to every notify target", func(t *testing.T) {
		t.Parallel()

		n, mocks := initNotifier(t)

		mocks.notifyTarget.EXPECT().
			ListByMaint(gomock.Any(), gomock.Any()).
			Return([]*entity.NotifyTarget{{
				ID:        uuid.New(),
				Transport: entity.NotifyTransportSlack,
				ChannelID: t.Name(),
			}}, nil)

		mocks.sender.EXPECT().
			Send(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil)

		require.NoError(t, n.NotifyMaintReminder(ctx, &entity.Maintenance{ID: uuid.New()}))
	})

	t.Run("no notify targets is a no-op", func(t *testing.T) {
		t.Parallel()

		n, mocks := initNotifier(t)

		mocks.notifyTarget.EXPECT().
			ListByMaint(gomock.Any(), gomock.Any()).
			Return([]*entity.NotifyTarget{}, nil)

		require.NoError(t, n.NotifyMaintReminder(ctx, &entity.Maintenance{ID: uuid.New()}))
	})

	t.Run("propagates a target-resolution error for retry", func(t *testing.T) {
		t.Parallel()

		n, mocks := initNotifier(t)

		mocks.notifyTarget.EXPECT().
			ListByMaint(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("db down"))

		// The reminder path returns the error so the goque worker retries the
		// task (unlike the best-effort lifecycle path).
		require.Error(t, n.NotifyMaintReminder(ctx, &entity.Maintenance{ID: uuid.New()}))
	})
}
