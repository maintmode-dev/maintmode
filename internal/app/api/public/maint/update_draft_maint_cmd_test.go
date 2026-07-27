package apimaint

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// TestToUpdateMaintenanceCmdDeferredNotificationsTriState pins the only place
// that turns the wire pointer into the command's tri-state. The service decides
// "unchanged" versus "clear" purely by nil-ness of this field, so collapsing an
// empty slice to nil here would disable clearing across the whole API while
// every service-level test stayed green — they build the command directly and
// never cross this mapping.
func TestToUpdateMaintenanceCmdDeferredNotificationsTriState(t *testing.T) {
	t.Parallel()

	fireAt := time.Now().Add(23 * time.Hour).UTC().Truncate(time.Second)

	tests := []struct {
		name      string
		requested *[]*apimodels.DeferredNotification
		assert    func(t *testing.T, got *[]*entity.DeferredNotificationInput)
	}{
		{
			name:      "absent field leaves reminders unchanged",
			requested: nil,
			assert: func(t *testing.T, got *[]*entity.DeferredNotificationInput) {
				t.Helper()
				require.Nil(t, got, "a missing field must stay nil so the service leaves reminders alone")
			},
		},
		{
			name:      "empty array clears reminders",
			requested: lo.ToPtr([]*apimodels.DeferredNotification{}),
			assert: func(t *testing.T, got *[]*entity.DeferredNotificationInput) {
				t.Helper()
				require.NotNil(t, got, "an empty array must survive as a non-nil pointer, or clearing turns into a no-op")
				require.Empty(t, lo.FromPtr(got))
			},
		},
		{
			name:      "non-empty array replaces reminders",
			requested: lo.ToPtr([]*apimodels.DeferredNotification{{FireAt: fireAt}}),
			assert: func(t *testing.T, got *[]*entity.DeferredNotificationInput) {
				t.Helper()
				require.NotNil(t, got)
				require.Len(t, lo.FromPtr(got), 1)
				require.Equal(t, fireAt, lo.FromPtr(got)[0].FireAt)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd, err := toUpdateMaintenanceCmd(context.Background(), uuid.New(), validUpdateRequest(tt.requested))
			require.NoError(t, err)

			tt.assert(t, cmd.DeferredNotifications)
		})
	}
}
