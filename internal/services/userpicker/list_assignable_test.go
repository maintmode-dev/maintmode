package userpicker

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestListAssignableDelegatesToGateway(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	mocks.users.EXPECT().
		ListActiveUsers(gomock.Any(), gomock.Any()).
		Return(&entity.ListAssignableUsersResult{
			Users:  []*entity.User{{ID: uuid.New(), Name: "Active", Email: "a@e.com"}},
			Total:  1,
			Limit:  50,
			Offset: 0,
		}, nil)

	q := &entity.ListAssignableUsersQuery{Search: "alice", Role: entity.RoleReviewer, Limit: 50}
	res, err := srv.ListAssignable(context.Background(), q)
	require.NoError(t, err)

	// The query is forwarded unchanged to the auth gateway.
	require.Equal(t, int64(1), res.Total)
	require.Len(t, res.Users, 1)
	require.Equal(t, int64(50), res.Limit)
}
