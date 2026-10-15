package maintnotify

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
)

// The link must reach the message from the maintenance itself, on both paths
// that carry it: the started event and the reminder.
func TestNotifyCarriesTaskTrackerURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const link = "https://jira.example.com/browse/OPS-42"
	maint := func() *entity.Maintenance {
		return &entity.Maintenance{ID: uuid.New(), Title: "DB upgrade", TaskTrackerURL: link}
	}

	for name, notify := range map[string]func(t *testing.T, n *Service){
		"started": func(_ *testing.T, n *Service) {
			n.NotifyMaintLifecycle(ctx, entity.NotifyEventMaintStarted, maint())
		},
		"reminder": func(t *testing.T, n *Service) {
			t.Helper()
			require.NoError(t, n.NotifyMaintReminder(ctx, maint()))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			n, mocks := initNotifier(t)
			expectNoMentions(mocks)
			mocks.notifyTarget.EXPECT().
				ListByMaint(gomock.Any(), gomock.Any()).
				Return([]*entity.NotifyTarget{target(entity.NotifyTransportTelegram, "-100500")}, nil)

			rec := recordSends(mocks, 1)

			notify(t, n)

			sent := rec.all()
			require.Len(t, sent, 1)
			require.Contains(t, sent[0].body, "\nTask: "+link)
		})
	}
}
