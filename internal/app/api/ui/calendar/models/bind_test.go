package uimodels

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/calendardto"
)

// TestToAPIMaintenanceView_DeferredNotifications pins the reminder mapping of the
// UI read-view: fields are carried over verbatim, the store's fire_at ASC order
// survives the mapping, and scheduled reflects whether the reminder was enqueued.
func TestToAPIMaintenanceView_DeferredNotifications(t *testing.T) {
	t.Parallel()

	var (
		firstID  = uuid.New()
		secondID = uuid.New()

		earliest = time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
		latest   = time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	)

	maintEvent := &calendardto.Maintenance{
		DeferredNotifications: []*calendardto.MaintenanceDeferredNotification{
			{ID: firstID, FireAt: earliest, Scheduled: true},
			{ID: secondID, FireAt: latest, Scheduled: false},
		},
	}

	got := ToAPIMaintenanceView(maintEvent, nil, nil)

	require.Equal(t, []*DeferredNotificationView{
		{ID: firstID, FireAt: earliest, Scheduled: true},
		{ID: secondID, FireAt: latest, Scheduled: false},
	}, got.DeferredNotifications,
		"reminders must keep the store's fire_at ASC order and carry the resolved scheduled flag")
}

// TestToAPIMaintenanceView_DeferredNotificationsEmpty pins the contract the FE
// relies on: an empty schedule serializes as [], never null.
func TestToAPIMaintenanceView_DeferredNotificationsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		deferred []*calendardto.MaintenanceDeferredNotification
	}{
		{name: "nil slice", deferred: nil},
		{name: "empty slice", deferred: []*calendardto.MaintenanceDeferredNotification{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ToAPIMaintenanceView(&calendardto.Maintenance{DeferredNotifications: tt.deferred}, nil, nil)

			require.NotNil(t, got.DeferredNotifications)
			require.Empty(t, got.DeferredNotifications)

			raw, err := json.Marshal(got)
			require.NoError(t, err)
			require.Contains(t, string(raw), `"deferred_notifications":[]`,
				"an empty schedule must serialize as [], not null")
		})
	}
}
