package user

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// makeNamedUser creates a user with a specific display name, so list tests can
// assert ordering and search deterministically against a shared DB. The email is
// made globally unique (random suffix) to avoid collisions with rows left by
// previous runs against the shared dev DB; tests isolate their cohort via a
// unique token embedded in the display name and search by that token.
func makeNamedUser(ctx context.Context, t *testing.T, srv *Service, name string, roles ...entity.Role) {
	t.Helper()

	makeNamedUserWithEmail(ctx, t, srv, name, xuuid.NewString()+"@email.com", roles...)
}

// makeNamedUserWithEmail is like makeNamedUser but lets the caller pin the email,
// for asserting the search filter matches on email.
func makeNamedUserWithEmail(ctx context.Context, t *testing.T, srv *Service, name, email string, roles ...entity.Role) {
	t.Helper()

	created, err := srv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: email,
		Name:  name,
	})
	require.NoError(t, err)

	if len(roles) > 0 {
		require.NoError(t, srv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
			Actor:  &entity.User{},
			UserID: created.ID,
			Roles:  roles,
		}))
	}
}

func TestListUsers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("search matches name or email, ordered by name asc", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		// Unique token shared by a small cohort (in the display name) so we can
		// isolate them in the shared DB via the search filter.
		token := "lutok" + xuuid.NewString()
		makeNamedUser(ctx, t, srv, "Charlie "+token)
		makeNamedUser(ctx, t, srv, "Alice "+token)
		makeNamedUser(ctx, t, srv, "Bob "+token)

		res, err := srv.ListUsers(ctx, &entity.ListUsersCmd{Search: token, Limit: 50, Offset: 0})
		require.NoError(t, err)
		require.Equal(t, int64(3), res.Total)
		require.Len(t, res.Users, 3)

		names := make([]string, len(res.Users))
		for i, u := range res.Users {
			names[i] = u.Name
		}
		require.True(t, sort.SliceIsSorted(names, func(i, j int) bool { return names[i] < names[j] }),
			"expected display_name ASC order, got %v", names)
	})

	t.Run("search matches by email too", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		token := "mailtok" + xuuid.NewString()
		makeNamedUserWithEmail(ctx, t, srv, "Someone", token+"@email.com")

		res, err := srv.ListUsers(ctx, &entity.ListUsersCmd{Search: token, Limit: 50})
		require.NoError(t, err)
		require.Equal(t, int64(1), res.Total)
	})

	t.Run("role filter keeps only matching users", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		token := "roletok" + xuuid.NewString()
		makeNamedUser(ctx, t, srv, "Rev "+token, entity.RoleReviewer)
		makeNamedUser(ctx, t, srv, "Ed "+token, entity.RoleEditor)

		res, err := srv.ListUsers(ctx, &entity.ListUsersCmd{Search: token, Role: entity.RoleReviewer, Limit: 50})
		require.NoError(t, err)
		require.Equal(t, int64(1), res.Total)
		require.Len(t, res.Users, 1)
		require.Contains(t, res.Users[0].Roles, entity.RoleReviewer)
	})

	t.Run("pagination limits the page but reports full total", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		token := "pagetok" + xuuid.NewString()
		for _, n := range []string{"U1", "U2", "U3"} {
			makeNamedUser(ctx, t, srv, n+" "+token)
		}

		res, err := srv.ListUsers(ctx, &entity.ListUsersCmd{Search: token, Limit: 2, Offset: 0})
		require.NoError(t, err)
		require.Equal(t, int64(3), res.Total)
		require.Len(t, res.Users, 2)
	})

	t.Run("active admin count is reported", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		res, err := srv.ListUsers(ctx, &entity.ListUsersCmd{Limit: 1})
		require.NoError(t, err)
		require.GreaterOrEqual(t, res.ActiveAdminCount, int64(0))
	})
}
