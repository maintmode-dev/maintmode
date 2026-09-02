package userinvitations

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	usersstore "github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

var errRollbackListRoles = errors.New("rollback list-roles test tx")

// Runs inside a rolled-back REPEATABLE READ transaction (see the users store
// counterpart for the shared-DB and isolation rationale): only live pending
// invitations hold a seat — expired-pending and revoked rows do not.
func TestListPendingRoles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	err := dbtx.NewTxManager(db).WithinTx(ctx, func(ctx context.Context) error {
		before, err := store.ListPendingRoles(ctx)
		require.NoError(t, err)
		beforeUsage := entity.BucketSeats(nil, before)

		inviter, err := usersstore.NewStore(db).Create(ctx, &entity.User{
			Email: uuid.NewString() + "@email.com",
			Name:  "Inviter" + t.Name(),
			Roles: []entity.Role{entity.RoleAdmin},
		})
		require.NoError(t, err)

		now := time.Now().UTC()
		create := func(roles []entity.Role, status entity.InvitationStatus, expiresAt time.Time) {
			_, err := store.Create(ctx, &entity.Invitation{
				Email:       uuid.NewString() + "@email.com",
				Roles:       roles,
				TokenHash:   uuid.NewString(),
				Status:      status,
				InvitedByID: inviter.ID,
				ExpiresAt:   expiresAt,
				SentAt:      now,
			})
			require.NoError(t, err)
		}

		create([]entity.Role{entity.RoleEditor}, entity.InvitationStatusPending, now.Add(time.Hour))   // live → editor seat
		create([]entity.Role{entity.RoleGuest}, entity.InvitationStatusPending, now.Add(-time.Hour))   // expired-pending → no seat
		create([]entity.Role{entity.RoleReviewer}, entity.InvitationStatusRevoked, now.Add(time.Hour)) // revoked → no seat
		// Persisted 'expired' (as the rotation sweep leaves it): must not hold a
		// seat even though expires_at is in the future — the seat gate keys off the
		// stored status, so a rotated invite frees its seat.
		create([]entity.Role{entity.RoleGuest}, entity.InvitationStatusExpired, now.Add(time.Hour))

		after, err := store.ListPendingRoles(ctx)
		require.NoError(t, err)
		afterUsage := entity.BucketSeats(nil, after)

		require.Len(t, after, len(before)+1, "only the live pending invitation holds a seat")
		require.Equal(t, beforeUsage.Editor.Pending+1, afterUsage.Editor.Pending)
		require.Equal(t, beforeUsage.Guest.Pending, afterUsage.Guest.Pending)
		require.Equal(t, beforeUsage.Reviewer.Pending, afterUsage.Reviewer.Pending)

		return errRollbackListRoles
	}, dbtx.WithIsolation(sql.LevelRepeatableRead))
	require.ErrorIs(t, err, errRollbackListRoles)
}
