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

// findItem returns the list item for the given invitation id, or nil. The store
// shares its DB with other parallel tests, so tests locate their own rows by id
// rather than asserting on the whole result set.
func findItem(items []*entity.InvitationListItem, id uuid.UUID) *entity.InvitationListItem {
	for _, item := range items {
		if item.Invitation.ID == id {
			return item
		}
	}
	return nil
}

func listAll(ctx context.Context, t *testing.T) []*entity.InvitationListItem {
	t.Helper()
	items, err := store.List(ctx, &entity.ListInvitationsCmd{})
	require.NoError(t, err)
	return items
}

// TestListResolvesInviterProfile is the core check for the inviter change: List
// must return the inviter's name and email resolved through the INNER JOIN on
// the auth users table, not a single handle.
func TestListResolvesInviterProfile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	inviter := makeInviter(ctx, t, "Inviter "+uuid.NewString())
	inv := makeInvitation(
		ctx, t,
		inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusPending,
		xtime.UTCNow().Add(24*time.Hour),
	)

	item := findItem(listAll(ctx, t), inv.ID)
	require.NotNil(t, item, "created invitation must appear in the list")

	require.Equal(t, inviter.ID, item.Invitation.InvitedByID)
	require.Equal(t, inviter.Name, item.Inviter.Name)
	require.Equal(t, inviter.Email, item.Inviter.Email)
}

// TestListMapsInvitationFields verifies the embedded invitation is mapped back
// faithfully through fromDB (not just the inviter columns of the join).
func TestListMapsInvitationFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	inviter := makeInviter(ctx, t, "Inviter "+uuid.NewString())
	email := uuid.NewString() + "@email.com"
	inv := makeInvitation(
		ctx, t,
		inviter.ID,
		email,
		entity.InvitationStatusPending,
		xtime.UTCNow().Add(24*time.Hour),
	)

	item := findItem(listAll(ctx, t), inv.ID)
	require.NotNil(t, item)

	require.Equal(t, inv.ID, item.Invitation.ID)
	require.Equal(t, email, item.Invitation.Email)
	require.Equal(t, entity.DefaultRoles, item.Invitation.Roles)
	require.Equal(t, entity.InvitationStatusPending, item.Invitation.Status)
}

// TestListStatusFilter exercises listWhereExpr: each filter selects the matching
// rows and excludes the others, and the derived pending/expired split keys off
// expires_at relative to now.
func TestListStatusFilter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	inviter := makeInviter(ctx, t, "Filter "+uuid.NewString())

	now := xtime.UTCNow()

	livePending := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, now.Add(24*time.Hour))

	// Stored pending but already past expiry: surfaces only under the derived
	// "expired" filter, never under "pending".
	expiredPending := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, now.Add(-24*time.Hour))

	accepted := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusAccepted, now.Add(24*time.Hour))

	revoked := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusRevoked, now.Add(24*time.Hour))

	listByStatus := func(status entity.InvitationStatus) []*entity.InvitationListItem {
		t.Helper()
		items, err := store.List(ctx, &entity.ListInvitationsCmd{Status: status})
		require.NoError(t, err)
		return items
	}

	has := func(items []*entity.InvitationListItem, id uuid.UUID) bool {
		return findItem(items, id) != nil
	}

	t.Run("pending shows only live pending", func(t *testing.T) {
		t.Parallel()
		items := listByStatus(entity.InvitationStatusPending)
		require.True(t, has(items, livePending.ID))
		require.False(t, has(items, expiredPending.ID))
		require.False(t, has(items, accepted.ID))
		require.False(t, has(items, revoked.ID))
	})

	t.Run("expired shows only expired pending", func(t *testing.T) {
		t.Parallel()
		items := listByStatus(entity.InvitationStatusExpired)
		require.True(t, has(items, expiredPending.ID))
		require.False(t, has(items, livePending.ID))
		require.False(t, has(items, accepted.ID))
		require.False(t, has(items, revoked.ID))
	})

	t.Run("accepted shows only accepted", func(t *testing.T) {
		t.Parallel()
		items := listByStatus(entity.InvitationStatusAccepted)
		require.True(t, has(items, accepted.ID))
		require.False(t, has(items, livePending.ID))
		require.False(t, has(items, expiredPending.ID))
		require.False(t, has(items, revoked.ID))
	})

	t.Run("revoked shows only revoked", func(t *testing.T) {
		t.Parallel()
		items := listByStatus(entity.InvitationStatusRevoked)
		require.True(t, has(items, revoked.ID))
		require.False(t, has(items, livePending.ID))
		require.False(t, has(items, expiredPending.ID))
		require.False(t, has(items, accepted.ID))
	})

	t.Run("no filter returns every status", func(t *testing.T) {
		t.Parallel()
		items := listByStatus("")
		require.True(t, has(items, livePending.ID))
		require.True(t, has(items, expiredPending.ID))
		require.True(t, has(items, accepted.ID))
		require.True(t, has(items, revoked.ID))
	})
}

// TestListExpiredFilterUnifiesPersistedAndDerived guards the RUK-211 read-path
// fix: once rotation persists status='expired', the "expired" filter must return
// both persisted-expired rows AND pending rows already past expiry that rotation
// has not yet flipped — otherwise a rotated invite would vanish from the admin
// list's expired view.
func TestListExpiredFilterUnifiesPersistedAndDerived(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	inviter := makeInviter(ctx, t, "ExpiredUnion "+uuid.NewString())
	now := xtime.UTCNow()

	// Persisted expired: status='expired' (as rotation would leave it). expires_at
	// is in the future to prove the filter keys off the stored status, not the time.
	persistedExpired := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusExpired, now.Add(24*time.Hour))

	// Derived expired: still stored pending but past expiry (not yet rotated).
	derivedExpired := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, now.Add(-24*time.Hour))

	// Live pending: must NOT appear under the expired filter.
	livePending := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, now.Add(24*time.Hour))

	items, err := store.List(ctx, &entity.ListInvitationsCmd{Status: entity.InvitationStatusExpired})
	require.NoError(t, err)

	require.NotNil(t, findItem(items, persistedExpired.ID), "persisted expired must appear")
	require.NotNil(t, findItem(items, derivedExpired.ID), "not-yet-rotated expired must appear")
	require.Nil(t, findItem(items, livePending.ID), "live pending must not appear under expired")
}

// TestListOrdersBySentAtDesc verifies the sent_at DESC ordering: the most
// recently sent invitation among this test's rows comes before the older one.
func TestListOrdersBySentAtDesc(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	inviter := makeInviter(ctx, t, "Order "+uuid.NewString())

	// Deterministic sent_at gap so the DESC ordering is unambiguous regardless
	// of insertion clock skew.
	older := makeInvitationAt(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, xtime.UTCNow().Add(24*time.Hour),
		xtime.UTCNow().Add(-time.Hour))
	newer := makeInvitationAt(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, xtime.UTCNow().Add(24*time.Hour),
		xtime.UTCNow())

	items := listAll(ctx, t)

	var olderIdx, newerIdx = -1, -1
	for i, item := range items {
		switch item.Invitation.ID {
		case older.ID:
			olderIdx = i
		case newer.ID:
			newerIdx = i
		}
	}

	require.NotEqual(t, -1, olderIdx)
	require.NotEqual(t, -1, newerIdx)
	require.Less(t, newerIdx, olderIdx, "newer sent_at must sort before older")
}
