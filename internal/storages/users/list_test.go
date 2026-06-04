package users

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestListExcludeBlocked verifies the assignment-picker filter: with
// ExcludeBlocked the blocked user is hidden, without it the admin list still
// sees the blocked user, and block→hide / unblock→show round-trips correctly.
func TestListExcludeBlocked(t *testing.T) {
	// Subtests block then unblock one shared row in order, so they run
	// sequentially; the parent test is deliberately not parallelized.

	ctx := context.Background()
	store := NewStore(db)

	// Unique token isolates this test's rows from other parallel tests sharing
	// the DB, so Total counts are deterministic.
	token := "picker-" + uuid.NewString()

	active, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Active " + token,
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	blocked, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Blocked " + token,
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	blockedAt := time.Now()
	blocked.BlockedAt = &blockedAt
	require.NoError(t, store.Update(ctx, blocked))

	listIDs := func(excludeBlocked bool) []uuid.UUID {
		t.Helper()
		users, total, err := store.List(ctx, &entity.ListUsersCmd{
			Search:         token,
			Limit:          50,
			Offset:         0,
			ExcludeBlocked: excludeBlocked,
		})
		require.NoError(t, err)
		require.Equal(t, int64(len(users)), total)
		return lo.Map(users, func(u *entity.User, _ int) uuid.UUID { return u.ID })
	}

	t.Run("ExcludeBlocked hides blocked user", func(t *testing.T) {
		ids := listIDs(true)
		require.Contains(t, ids, active.ID)
		require.NotContains(t, ids, blocked.ID)
	})

	t.Run("admin list (no exclude) keeps blocked user", func(t *testing.T) {
		ids := listIDs(false)
		require.Contains(t, ids, active.ID)
		require.Contains(t, ids, blocked.ID)
	})

	t.Run("unblock makes the user visible to the picker again", func(t *testing.T) {
		blocked.BlockedAt = nil
		require.NoError(t, store.Update(ctx, blocked))

		ids := listIDs(true)
		require.Contains(t, ids, active.ID)
		require.Contains(t, ids, blocked.ID)
	})
}

// TestListRolesFilter verifies the roles[] filter is OR: a user matches when it
// has ANY of the listed roles. The token isolates this test's rows.
func TestListRolesFilter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)
	token := "roles-" + uuid.NewString()

	reviewer, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Reviewer " + token,
		Roles: []entity.Role{entity.RoleReviewer},
	})
	require.NoError(t, err)

	editor, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Editor " + token,
		Roles: []entity.Role{entity.RoleEditor},
	})
	require.NoError(t, err)

	guest, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Guest " + token,
		Roles: []entity.Role{entity.RoleGuest},
	})
	require.NoError(t, err)

	listIDs := func(roles ...entity.Role) []uuid.UUID {
		t.Helper()
		users, total, err := store.List(ctx, &entity.ListUsersCmd{
			Search: token,
			Roles:  roles,
			Limit:  50,
		})
		require.NoError(t, err)
		require.Equal(t, int64(len(users)), total)
		return lo.Map(users, func(u *entity.User, _ int) uuid.UUID { return u.ID })
	}

	t.Run("single role keeps only that role", func(t *testing.T) {
		t.Parallel()
		ids := listIDs(entity.RoleReviewer)
		require.Contains(t, ids, reviewer.ID)
		require.NotContains(t, ids, editor.ID)
		require.NotContains(t, ids, guest.ID)
	})

	t.Run("multiple roles are OR'd", func(t *testing.T) {
		t.Parallel()
		ids := listIDs(entity.RoleReviewer, entity.RoleEditor)
		require.Contains(t, ids, reviewer.ID)
		require.Contains(t, ids, editor.ID)
		require.NotContains(t, ids, guest.ID)
	})
}

// TestListIDsFilter verifies the ids[] batch filter restricts the result to the
// requested ids (author-resolution path).
func TestListIDsFilter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)
	token := "ids-" + uuid.NewString()

	u1, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "One " + token,
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	u2, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Two " + token,
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	u3, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Three " + token,
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	users, total, err := store.List(ctx, &entity.ListUsersCmd{
		IDs:   []uuid.UUID{u1.ID, u3.ID},
		Limit: 50,
	})
	require.NoError(t, err)
	require.Equal(t, int64(len(users)), total)

	ids := lo.Map(users, func(u *entity.User, _ int) uuid.UUID { return u.ID })
	require.Contains(t, ids, u1.ID)
	require.Contains(t, ids, u3.ID)
	require.NotContains(t, ids, u2.ID)
}
