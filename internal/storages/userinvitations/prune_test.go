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

// makeInvitationCreatedAt inserts an invitation and back-dates its created_at, so
// prune tests can place a row on either side of the retention cutoff (Create
// stamps created_at server-side, so a raw UPDATE is used to age the row).
func makeInvitationCreatedAt(
	ctx context.Context,
	t *testing.T,
	inviterID uuid.UUID,
	status entity.InvitationStatus,
	createdAt time.Time,
) *entity.Invitation {
	t.Helper()
	inv := makeInvitation(ctx, t, inviterID,
		uuid.NewString()+"@email.com", status, xtime.UTCNow().Add(24*time.Hour))
	_, err := db.ExecContext(ctx,
		`UPDATE user_invitations SET created_at = $1 WHERE id = $2`, createdAt, inv.ID)
	require.NoError(t, err)
	return inv
}

// existsByID reports whether the invitation row is still present.
func existsByID(ctx context.Context, t *testing.T, id uuid.UUID) bool {
	t.Helper()
	var n int
	err := db.GetContext(ctx, &n,
		`SELECT count(*) FROM user_invitations WHERE id = $1`, id)
	require.NoError(t, err)
	return n > 0
}

// TestPruneTerminalOlderThan_DeletesOnlyOldTerminal is the core retention check:
// terminal rows (expired/accepted/revoked) older than the cutoff are deleted;
// recent terminal rows and pending rows (even old ones) are kept.
func TestPruneTerminalOlderThan_DeletesOnlyOldTerminal(t *testing.T) {
	ctx := context.Background()
	inviter := makeInviter(ctx, t, "Prune "+uuid.NewString())
	now := xtime.UTCNow()
	old := now.AddDate(-2, 0, 0)    // 2 years old
	cutoff := now.AddDate(-1, 0, 0) // retention 1 year

	oldExpired := makeInvitationCreatedAt(ctx, t, inviter.ID, entity.InvitationStatusExpired, old)
	oldAccepted := makeInvitationCreatedAt(ctx, t, inviter.ID, entity.InvitationStatusAccepted, old)
	oldRevoked := makeInvitationCreatedAt(ctx, t, inviter.ID, entity.InvitationStatusRevoked, old)
	oldPending := makeInvitationCreatedAt(ctx, t, inviter.ID, entity.InvitationStatusPending, old)
	freshExpired := makeInvitationCreatedAt(ctx, t, inviter.ID, entity.InvitationStatusExpired, now)

	deleted, err := store.PruneTerminalOlderThan(ctx, cutoff, 1000)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(3), "the three old terminal rows must be deleted")

	require.False(t, existsByID(ctx, t, oldExpired.ID), "old expired must be pruned")
	require.False(t, existsByID(ctx, t, oldAccepted.ID), "old accepted must be pruned")
	require.False(t, existsByID(ctx, t, oldRevoked.ID), "old revoked must be pruned")
	require.True(t, existsByID(ctx, t, oldPending.ID), "pending must never be pruned")
	require.True(t, existsByID(ctx, t, freshExpired.ID), "terminal rows within retention are kept")
}

// TestPruneTerminalOlderThan_BoundaryIsStrict pins the exact-boundary contract:
// prune uses strict `created_at < cutoff`, so a terminal row created exactly at
// the cutoff instant is still within retention and must survive.
func TestPruneTerminalOlderThan_BoundaryIsStrict(t *testing.T) {
	ctx := context.Background()
	inviter := makeInviter(ctx, t, "PruneBoundary "+uuid.NewString())

	cutoff := xtime.UTCNow().AddDate(-1, 0, 0)
	atBoundary := makeInvitationCreatedAt(ctx, t, inviter.ID, entity.InvitationStatusExpired, cutoff)

	_, err := store.PruneTerminalOlderThan(ctx, cutoff, 1000)
	require.NoError(t, err)

	require.True(t, existsByID(ctx, t, atBoundary.ID),
		"a terminal row with created_at == cutoff must NOT be deleted (strict <)")
}

// TestPruneTerminalOlderThan_RespectsBatchLimit checks the id-subquery bound.
func TestPruneTerminalOlderThan_RespectsBatchLimit(t *testing.T) {
	ctx := context.Background()
	inviter := makeInviter(ctx, t, "PruneBatch "+uuid.NewString())
	now := xtime.UTCNow()
	old := now.AddDate(-2, 0, 0)
	cutoff := now.AddDate(-1, 0, 0)

	for range 3 {
		makeInvitationCreatedAt(ctx, t, inviter.ID, entity.InvitationStatusExpired, old)
	}

	deleted, err := store.PruneTerminalOlderThan(ctx, cutoff, 2)
	require.NoError(t, err)
	require.LessOrEqual(t, deleted, int64(2), "one batch must never delete more than the limit")
}
