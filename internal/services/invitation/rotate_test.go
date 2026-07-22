package invitation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/userinvitations"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// makeExpiredPending inserts a pending invitation already past its expiry, so
// the rotation sweep is eligible to flip it.
func makeExpiredPending(ctx context.Context, t *testing.T, s *Service) *entity.Invitation {
	t.Helper()
	adminID := makeAdmin(ctx, t, s).ID
	store := userinvitations.NewStore(db)
	inv, err := store.Create(ctx, &entity.Invitation{
		Email:       uniqueEmail(t),
		Roles:       entity.DefaultRoles,
		TokenHash:   newUUID().String(),
		Status:      entity.InvitationStatusPending,
		InvitedByID: adminID,
		ExpiresAt:   xtime.UTCNow().Add(-24 * time.Hour),
		SentAt:      xtime.UTCNow().Add(-8 * 24 * time.Hour),
	})
	require.NoError(t, err)
	return inv
}

// TestRotate_DrainsAcrossBatches inserts more expired-pending rows than one batch
// holds and asserts a single Rotate call flips them all.
func TestRotate_DrainsAcrossBatches(t *testing.T) {
	ctx := context.Background()
	s, _ := initService(t)

	invs := make([]*entity.Invitation, 0, 5)
	for range 5 {
		invs = append(invs, makeExpiredPending(ctx, t, s))
	}

	// batchLimit 2 forces several batches through the drain loop.
	require.NoError(t, s.Rotate(ctx, 2))

	store := userinvitations.NewStore(db)
	for _, inv := range invs {
		got, err := store.GetByID(ctx, inv.ID)
		require.NoError(t, err)
		require.Equal(t, entity.InvitationStatusExpired, got.Status,
			"drain loop must flip every expired-pending row across batches")
	}
}

// TestRotate_DrainsWhenCountIsExactMultipleOfBatch exercises the stop condition's
// harder path: when the eligible count is an exact multiple of batchLimit, the
// final data batch comes back FULL (flipped == batchLimit), so the loop must run
// one extra empty batch (flipped 0 < batchLimit) to detect drain. This proves the
// loop keys off "short batch", not "empty result", and a full final batch does
// not terminate early leaving rows behind.
func TestRotate_DrainsWhenCountIsExactMultipleOfBatch(t *testing.T) {
	ctx := context.Background()
	s, _ := initService(t)

	// 4 eligible rows with batchLimit 2 → batches of 2, 2, then 0.
	invs := make([]*entity.Invitation, 0, 4)
	for range 4 {
		invs = append(invs, makeExpiredPending(ctx, t, s))
	}

	require.NoError(t, s.Rotate(ctx, 2))

	store := userinvitations.NewStore(db)
	for _, inv := range invs {
		got, err := store.GetByID(ctx, inv.ID)
		require.NoError(t, err)
		require.Equal(t, entity.InvitationStatusExpired, got.Status,
			"an exact-multiple count must fully drain, not leave the last full batch behind")
	}
}

// TestRotate_NonPositiveBatchLimitStillDrains guards the loop-termination fix: a
// non-positive batchLimit falls back to the default rather than spinning.
func TestRotate_NonPositiveBatchLimitStillDrains(t *testing.T) {
	ctx := context.Background()
	s, _ := initService(t)

	inv := makeExpiredPending(ctx, t, s)

	require.NoError(t, s.Rotate(ctx, 0))

	store := userinvitations.NewStore(db)
	got, err := store.GetByID(ctx, inv.ID)
	require.NoError(t, err)
	require.Equal(t, entity.InvitationStatusExpired, got.Status,
		"batchLimit 0 must fall back to the default and still flip expired-pending rows")
}
