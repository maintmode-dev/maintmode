package invitation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/userinvitations"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// makeTerminalAged inserts a terminal invitation and back-dates its created_at,
// so the retention sweep is eligible to delete it.
func makeTerminalAged(ctx context.Context, t *testing.T, s *Service, status entity.InvitationStatus, createdAt time.Time) uuid.UUID {
	t.Helper()
	adminID := makeAdmin(ctx, t, s).ID
	store := userinvitations.NewStore(db)
	inv, err := store.Create(ctx, &entity.Invitation{
		Email:       uniqueEmail(t),
		Roles:       entity.DefaultRoles,
		TokenHash:   newUUID().String(),
		Status:      status,
		InvitedByID: adminID,
		ExpiresAt:   xtime.UTCNow().Add(24 * time.Hour),
		SentAt:      xtime.UTCNow(),
	})
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`UPDATE user_invitations SET created_at = $1 WHERE id = $2`, createdAt, inv.ID)
	require.NoError(t, err)
	return inv.ID
}

func rowExists(ctx context.Context, t *testing.T, id uuid.UUID) bool {
	t.Helper()
	var n int
	require.NoError(t, db.GetContext(ctx, &n,
		`SELECT count(*) FROM user_invitations WHERE id = $1`, id))
	return n > 0
}

// TestPrune_DrainsAcrossBatches inserts more old terminal rows than one batch
// holds and asserts a single Prune call clears them all.
func TestPrune_DrainsAcrossBatches(t *testing.T) {
	ctx := context.Background()
	s, _ := initService(t)
	old := xtime.UTCNow().AddDate(-2, 0, 0)

	ids := make([]uuid.UUID, 0, 5)
	for range 5 {
		ids = append(ids, makeTerminalAged(ctx, t, s, entity.InvitationStatusExpired, old))
	}

	// retention 1 year → the 2-year-old rows are all eligible; batchLimit 2 forces
	// several batches.
	require.NoError(t, s.Prune(ctx, 365*24*time.Hour, 2))

	for _, id := range ids {
		require.False(t, rowExists(ctx, t, id), "drain loop must delete every old terminal row")
	}
}

// TestPrune_NonPositiveBatchLimitStillDrains guards the loop-termination fix.
func TestPrune_NonPositiveBatchLimitStillDrains(t *testing.T) {
	ctx := context.Background()
	s, _ := initService(t)
	old := xtime.UTCNow().AddDate(-2, 0, 0)

	id := makeTerminalAged(ctx, t, s, entity.InvitationStatusRevoked, old)

	require.NoError(t, s.Prune(ctx, 365*24*time.Hour, 0))

	require.False(t, rowExists(ctx, t, id),
		"batchLimit 0 must fall back to the default and still delete old terminal rows")
}

// TestPrune_NonPositiveRetentionDoesNotPurgeAll guards against a misconfigured
// (zero or negative) retention. A negative retention would push the cutoff into
// the future and make every terminal row eligible — deleting all invite history.
// The service must clamp retention<=0 to a safe default so a config typo fails
// safe rather than purging recent terminal rows.
func TestPrune_NonPositiveRetentionDoesNotPurgeAll(t *testing.T) {
	ctx := context.Background()
	s, _ := initService(t)

	// A terminal row created just now — must survive any sane retention window.
	recentID := makeTerminalAged(ctx, t, s, entity.InvitationStatusAccepted, xtime.UTCNow())

	// Negative retention: without a clamp, cutoff = now-(-1h) = now+1h, so
	// created_at < cutoff matches this recent row and it would be deleted.
	require.NoError(t, s.Prune(ctx, -time.Hour, 100))

	require.True(t, rowExists(ctx, t, recentID),
		"a negative retention must be clamped, not delete recent terminal rows")
}

// TestPrune_KeepsRowsWithinRetention is a no-op when the terminal row is recent.
func TestPrune_KeepsRowsWithinRetention(t *testing.T) {
	ctx := context.Background()
	s, _ := initService(t)

	id := makeTerminalAged(ctx, t, s, entity.InvitationStatusAccepted, xtime.UTCNow())

	require.NoError(t, s.Prune(ctx, 365*24*time.Hour, 100))

	require.True(t, rowExists(ctx, t, id), "terminal rows within retention are kept")
}
