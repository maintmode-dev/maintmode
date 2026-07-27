package apimodels

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

// TestUpdateDraftMaintRequestDeferredNotificationsWire pins the JSON decoding
// half of the tri-state contract, which the mapping above depends on. Absent
// and null must both arrive as nil ("unchanged") and an empty array as a
// non-nil empty slice ("clear"). null is asserted explicitly because it reaches
// nil by a different mechanism than an absent field: an absent field leaves the
// destination untouched, whereas null actively resets it. They coincide only
// because binding always decodes into a zero-valued request struct — so the
// field is seeded with a non-nil value here to prove null really clears it
// rather than merely being skipped.
func TestUpdateDraftMaintRequestDeferredNotificationsWire(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		body        string
		seeded      bool
		expectNil   bool
		expectedLen int
	}{
		{
			// Decoded into a zero-valued struct, as the real binder does: an
			// absent field simply leaves the destination untouched.
			name:      "absent field means unchanged",
			body:      `{"title":"t"}`,
			expectNil: true,
		}, {
			// Seeded non-nil: null must actively reset the field, which is a
			// different mechanism from the absent case above.
			name:      "null means unchanged",
			body:      `{"title":"t","deferred_notifications":null}`,
			seeded:    true,
			expectNil: true,
		}, {
			name:        "empty array means clear",
			body:        `{"title":"t","deferred_notifications":[]}`,
			expectedLen: 0,
		}, {
			name:        "non-empty array means replace",
			body:        `{"title":"t","deferred_notifications":[{"fire_at":"2026-08-01T09:00:00Z"}]}`,
			expectedLen: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := &UpdateDraftMaintRequest{}
			if tc.seeded {
				req.DeferredNotifications = lo.ToPtr([]*DeferredNotification{{FireAt: time.Now()}})
			}

			require.NoError(t, json.Unmarshal([]byte(tc.body), req))

			if tc.expectNil {
				require.Nil(t, req.DeferredNotifications, "must decode to nil so the service leaves reminders unchanged")

				return
			}

			require.NotNil(t, req.DeferredNotifications, "must stay non-nil so the service can tell 'clear' from 'unchanged'")
			require.Len(t, lo.FromPtr(req.DeferredNotifications), tc.expectedLen)
		})
	}
}
