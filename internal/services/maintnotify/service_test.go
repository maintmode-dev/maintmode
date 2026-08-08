package maintnotify

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestNotifyMaintLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		n, mocks := initNotifier(t)
		expectNoMentions(mocks)

		mocks.notifyTarget.EXPECT().
			ListByMaint(gomock.Any(), gomock.Any()).
			Return([]*entity.NotifyTarget{{
				ID:                 uuid.New(),
				ChannelID:          uuid.New(),
				Transport:          entity.NotifyTransportSlack,
				TransportChannelID: t.Name(),
			}}, nil)

		// No ref comes back, so there is no root to record and SetRootRef is
		// never called — gomock would fail the test on an unexpected call.
		mocks.sender.EXPECT().
			Send(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), nil).
			Return(entity.SendResult{}, nil)

		n.NotifyMaintLifecycle(ctx, entity.NotifyEventMaintStarted, &entity.Maintenance{ID: uuid.New()})
	})

	t.Run("invalid event", func(t *testing.T) {
		t.Parallel()

		n, _ := initNotifier(t)
		t.Run("RejectsStepKind", func(t *testing.T) {
			t.Parallel()

			n.NotifyMaintLifecycle(ctx,
				entity.NotifyEventStepStarted,
				&entity.Maintenance{ID: uuid.New()},
			)
		})

		t.Run("RejectsMaintKind", func(t *testing.T) {
			t.Parallel()

			n.NotifyStepLifecycle(ctx,
				entity.NotifyEventMaintStarted,
				&entity.Maintenance{ID: uuid.New()},
				&entity.MaintenanceStep{ID: uuid.New()},
			)
		})
	})

	t.Run("no notify targets", func(t *testing.T) {
		t.Parallel()

		n, mocks := initNotifier(t)
		expectNoMentions(mocks)

		mocks.notifyTarget.EXPECT().
			ListByMaint(gomock.Any(), gomock.Any()).
			Return([]*entity.NotifyTarget{}, nil)

		n.NotifyMaintLifecycle(ctx, entity.NotifyEventMaintStarted, &entity.Maintenance{ID: uuid.New()})
	})
}
