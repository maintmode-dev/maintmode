package userpicker

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestListAssignableMapsQueryToListUsers(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	var gotCmd *entity.ListUsersCmd
	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd *entity.ListUsersCmd) (*entity.ListUsersResult, error) {
			gotCmd = cmd
			return &entity.ListUsersResult{
				Users: []*entity.User{{ID: uuid.New(), Name: "Active", Email: "a@e.com"}},
				Total: 1,
			}, nil
		})

	q := &entity.ListAssignableUsersQuery{Search: "alice", Roles: []entity.Role{entity.RoleReviewer}, Limit: 50, Offset: 20}
	res, err := srv.ListAssignable(context.Background(), q)
	require.NoError(t, err)

	// "assignable" maps onto a ListUsersCmd with ExcludeBlocked always set; the
	// query's search/roles/limit/offset pass through.
	require.True(t, gotCmd.ExcludeBlocked)
	require.Equal(t, "alice", gotCmd.Search)

	// SearchMessengerTags must stay false: the picker returns no telegram/slack
	// tags, so matching them would only turn this endpoint into an oracle
	// answering "whose tag is this" for every role from guest up. The store-side
	// gate is covered by TestListSearchMessengerTagsGate; this pins the caller
	// that relies on it, so flipping the flag here cannot pass unnoticed.
	require.False(t, gotCmd.SearchMessengerTags)
	require.Equal(t, []entity.Role{entity.RoleReviewer}, gotCmd.Roles)
	require.Equal(t, int64(50), gotCmd.Limit)
	require.Equal(t, int64(20), gotCmd.Offset)

	require.Equal(t, int64(1), res.Total)
	require.Len(t, res.Users, 1)
	require.Equal(t, int64(50), res.Limit)
	require.Equal(t, int64(20), res.Offset)
}
