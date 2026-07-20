package users

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// errRollbackListRoles forces the test transaction to roll back so the created
// users never hit the shared dev DB.
var errRollbackListRoles = errors.New("rollback list-roles test tx")

// The shared dev DB holds users from other tests and runs, so absolute counts
// are meaningless. The test runs inside a rolled-back REPEATABLE READ
// transaction and asserts the delta: the snapshot makes concurrent commits
// invisible between the before/after reads (READ COMMITTED would leak them),
// while its own writes stay in-tx only.
func TestListActiveRoles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	err := dbtx.NewTxManager(db).WithinTx(ctx, func(ctx context.Context) error {
		before, err := store.ListActiveRoles(ctx)
		require.NoError(t, err)
		beforeUsage := entity.BucketSeats(before, nil)

		create := func(roles []entity.Role, blockedAt *time.Time) {
			_, err := store.Create(ctx, &entity.User{
				Email:     uuid.NewString() + "@email.com",
				Name:      "Name" + t.Name(),
				Roles:     roles,
				BlockedAt: blockedAt,
			})
			require.NoError(t, err)
		}

		now := time.Now().UTC()
		create([]entity.Role{entity.RoleAdmin, entity.RoleEditor}, nil) // → admin bucket
		create([]entity.Role{entity.RoleGuest}, nil)                    // → guest bucket
		create([]entity.Role{entity.RoleReviewer}, &now)                // blocked: no seat

		after, err := store.ListActiveRoles(ctx)
		require.NoError(t, err)
		afterUsage := entity.BucketSeats(after, nil)

		require.Len(t, after, len(before)+2, "blocked user must not hold a seat")
		require.Equal(t, beforeUsage.Admin.Active+1, afterUsage.Admin.Active)
		require.Equal(t, beforeUsage.Guest.Active+1, afterUsage.Guest.Active)
		require.Equal(t, beforeUsage.Reviewer.Active, afterUsage.Reviewer.Active)

		return errRollbackListRoles
	}, dbtx.WithIsolation(sql.LevelRepeatableRead))
	require.ErrorIs(t, err, errRollbackListRoles)
}
