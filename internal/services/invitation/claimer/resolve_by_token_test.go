package claimer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// createInvitationWithToken stores an invitation whose raw link token the test
// knows, which createInvitation hides.
func createInvitationWithToken(
	ctx context.Context, t *testing.T, h *harness, expiresAt time.Time,
) (inv *entity.Invitation, rawToken string) {
	t.Helper()

	rawToken = xuuid.NewString()
	inv, err := h.store.Create(ctx, &entity.Invitation{
		Email:       uniqueEmail(t),
		Roles:       []entity.Role{entity.RoleEditor},
		TokenHash:   xhash.HashSha256([]byte(rawToken)),
		Status:      entity.InvitationStatusPending,
		ExpiresAt:   expiresAt,
		SentAt:      xtime.UTCNow().Add(-time.Minute),
		InvitedByID: makeUser(ctx, t, h).ID,
	})
	require.NoError(t, err)

	return inv, rawToken
}

func TestResolveByToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("a live invitation resolves with its email and roles", func(t *testing.T) {
		t.Parallel()
		h := initHarness(t)
		created, rawToken := createInvitationWithToken(ctx, t, h, xtime.UTCNow().Add(time.Hour))

		inv, err := h.claimer.ResolveByToken(ctx, rawToken)
		require.NoError(t, err)
		require.Equal(t, created.ID, inv.ID)
		require.Equal(t, created.Email, inv.Email)
		require.Equal(t, []entity.Role{entity.RoleEditor}, inv.Roles)
	})

	refusals := []struct {
		name  string
		token func(t *testing.T, h *harness) string
	}{
		{"empty token", func(*testing.T, *harness) string { return "" }},
		{"unknown token", func(*testing.T, *harness) string { return xuuid.NewString() }},
		{"expired", func(t *testing.T, h *harness) string {
			t.Helper()
			_, raw := createInvitationWithToken(ctx, t, h, xtime.UTCNow().Add(-time.Minute))
			return raw
		}},
		{"already accepted", func(t *testing.T, h *harness) string {
			t.Helper()
			inv, raw := createInvitationWithToken(ctx, t, h, xtime.UTCNow().Add(time.Hour))
			accepted, err := h.store.MarkAccepted(ctx, inv.ID)
			require.NoError(t, err)
			require.True(t, accepted)
			return raw
		}},
	}
	for _, tc := range refusals {
		t.Run(tc.name+" is refused as invalid", func(t *testing.T) {
			t.Parallel()
			h := initHarness(t)

			inv, err := h.claimer.ResolveByToken(ctx, tc.token(t, h))
			require.ErrorIs(t, err, apperr.ErrInvalidInvitation)
			require.Nil(t, inv)
		})
	}
}
