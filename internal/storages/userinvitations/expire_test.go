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

// getStatus reads back the stored status of an invitation by id, so the tests
// assert on the persisted column rather than any read-time derivation.
func getStatus(ctx context.Context, t *testing.T, id uuid.UUID) entity.InvitationStatus {
	t.Helper()
	inv, err := store.GetByID(ctx, id)
	require.NoError(t, err)
	return inv.Status
}

// TestExpireOlderThan_FlipsOnlyPastPending is the core rotation check: a pending
// invitation past its expiry flips to the persisted 'expired' status; a pending
// one still in the future, and terminal (accepted/revoked) rows, are untouched.
func TestExpireOlderThan_FlipsOnlyPastPending(t *testing.T) {
	ctx := context.Background()
	inviter := makeInviter(ctx, t, "Expire "+uuid.NewString())
	now := xtime.UTCNow()

	pastPending := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, now.Add(-24*time.Hour))
	livePending := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, now.Add(24*time.Hour))
	accepted := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusAccepted, now.Add(-24*time.Hour))
	revoked := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusRevoked, now.Add(-24*time.Hour))

	affected, err := store.ExpireOlderThan(ctx, now, 1000)
	require.NoError(t, err)
	require.GreaterOrEqual(t, affected, int64(1), "at least the past-pending row must flip")

	require.Equal(t, entity.InvitationStatusExpired, getStatus(ctx, t, pastPending.ID))
	require.Equal(t, entity.InvitationStatusPending, getStatus(ctx, t, livePending.ID))
	require.Equal(t, entity.InvitationStatusAccepted, getStatus(ctx, t, accepted.ID))
	require.Equal(t, entity.InvitationStatusRevoked, getStatus(ctx, t, revoked.ID))
}

// TestExpireOlderThan_FreesPendingSlot proves the product goal: once a pending
// invitation is rotated to 'expired' it leaves the partial-unique pending index,
// so a fresh invite for the same email can be created (the previous invariant
// allowed only one active pending per email).
func TestExpireOlderThan_FreesPendingSlot(t *testing.T) {
	ctx := context.Background()
	inviter := makeInviter(ctx, t, "Slot "+uuid.NewString())
	now := xtime.UTCNow()
	email := uuid.NewString() + "@email.com"

	makeInvitation(ctx, t, inviter.ID, email,
		entity.InvitationStatusPending, now.Add(-24*time.Hour))

	_, err := store.ExpireOlderThan(ctx, now, 1000)
	require.NoError(t, err)

	// A second active-pending create for the same email would violate the partial
	// unique index if the old row were still pending; after rotation it succeeds.
	_, err = store.Create(ctx, &entity.Invitation{
		Email:       email,
		Roles:       entity.DefaultRoles,
		TokenHash:   uuid.NewString(),
		Status:      entity.InvitationStatusPending,
		InvitedByID: inviter.ID,
		ExpiresAt:   now.Add(24 * time.Hour),
		SentAt:      now,
	})
	require.NoError(t, err, "rotating the old pending invite must free the email's pending slot")
}

// TestExpireOlderThan_IsIdempotent proves the correctness property the rotation
// sweep relies on for multi-replica / clock-skew safety: a second run over an
// already-rotated row finds nothing eligible (it is no longer pending), so it
// affects zero rows and does not touch the stored status.
func TestExpireOlderThan_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	inviter := makeInviter(ctx, t, "Idempotent "+uuid.NewString())
	now := xtime.UTCNow()

	inv := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, now.Add(-24*time.Hour))

	first, err := store.ExpireOlderThan(ctx, now, 1000)
	require.NoError(t, err)
	require.GreaterOrEqual(t, first, int64(1))
	require.Equal(t, entity.InvitationStatusExpired, getStatus(ctx, t, inv.ID))

	// Second sweep on the same boundary: the row is already 'expired', so it is no
	// longer eligible. This specific row contributes zero to the affected count.
	require.Equal(t, entity.InvitationStatusExpired, getStatus(ctx, t, inv.ID),
		"a second rotation must leave the already-expired row unchanged")
	_, err = store.ExpireOlderThan(ctx, now, 1000)
	require.NoError(t, err)
	require.Equal(t, entity.InvitationStatusExpired, getStatus(ctx, t, inv.ID))
}

// TestExpireOlderThan_BoundaryIsStrict pins the exact-boundary contract the read
// path relies on: rotation uses strict `expires_at < now`, so a row whose expiry
// equals the sweep instant is NOT yet expired and must stay pending. This keeps
// the sweep consistent with list.go (pending = expires_at >= now, expired =
// expires_at < now) and list_roles.go (seat held iff expires_at >= now) — an
// off-by-one between < and >= would leak the boundary row between the two views.
func TestExpireOlderThan_BoundaryIsStrict(t *testing.T) {
	ctx := context.Background()
	inviter := makeInviter(ctx, t, "Boundary "+uuid.NewString())

	// Fix the boundary instant and place the row's expiry exactly on it.
	boundary := xtime.UTCNow()
	atBoundary := makeInvitation(ctx, t, inviter.ID,
		uuid.NewString()+"@email.com",
		entity.InvitationStatusPending, boundary)

	affected, err := store.ExpireOlderThan(ctx, boundary, 1000)
	require.NoError(t, err)
	_ = affected // other rows may exist on the shared DB; assert on this row only

	require.Equal(t, entity.InvitationStatusPending, getStatus(ctx, t, atBoundary.ID),
		"a row with expires_at == now must NOT flip (strict <), staying pending")
}

// TestExpireOlderThan_RespectsBatchLimit checks the id-subquery bound: with more
// eligible rows than the limit, one call flips at most `limit` of them.
func TestExpireOlderThan_RespectsBatchLimit(t *testing.T) {
	ctx := context.Background()
	inviter := makeInviter(ctx, t, "Batch "+uuid.NewString())
	now := xtime.UTCNow()

	for range 3 {
		makeInvitation(ctx, t, inviter.ID,
			uuid.NewString()+"@email.com",
			entity.InvitationStatusPending, now.Add(-24*time.Hour))
	}

	affected, err := store.ExpireOlderThan(ctx, now, 2)
	require.NoError(t, err)
	require.LessOrEqual(t, affected, int64(2), "one batch must never flip more than the limit")
}
