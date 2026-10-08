package userinvitations

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// The acceptance itself refuses a pending invitation already past its expiry,
// so a caller's earlier check cannot be raced by the clock.
func TestMarkAccepted_RefusesAnExpiredPendingInvitation(t *testing.T) {
	ctx := context.Background()
	inviter := makeInviter(ctx, t, "Accept "+uuid.NewString())
	now := xtime.UTCNow()

	expired := makeInvitation(ctx, t, inviter.ID, uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, now.Add(-time.Minute))
	live := makeInvitation(ctx, t, inviter.ID, uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, now.Add(time.Hour))

	accepted, err := store.MarkAccepted(ctx, expired.ID)
	require.NoError(t, err)
	require.False(t, accepted)
	require.Equal(t, entity.InvitationStatusPending, getStatus(ctx, t, expired.ID))

	accepted, err = store.MarkAccepted(ctx, live.ID)
	require.NoError(t, err)
	require.True(t, accepted)
	require.Equal(t, entity.InvitationStatusAccepted, getStatus(ctx, t, live.ID))
}
