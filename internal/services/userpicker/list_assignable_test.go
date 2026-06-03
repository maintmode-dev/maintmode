package userpicker

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	authgateway "github.com/ruko1202/maintmode/internal/gateways/auth"
)

// fakeAuthGateway records the query it received and returns canned users.
type fakeAuthGateway struct {
	gotQuery *entity.ListAssignableUsersQuery
	result   *authgateway.ListActiveUsersResult
	err      error
}

func (f *fakeAuthGateway) ListActiveUsers(_ context.Context, q *entity.ListAssignableUsersQuery) (*authgateway.ListActiveUsersResult, error) {
	f.gotQuery = q
	return f.result, f.err
}

func TestListAssignableDelegatesToGateway(t *testing.T) {
	t.Parallel()

	gw := &fakeAuthGateway{
		result: &authgateway.ListActiveUsersResult{
			Users:  []*entity.User{{ID: uuid.New(), Name: "Active", Email: "a@e.com"}},
			Total:  1,
			Limit:  50,
			Offset: 0,
		},
	}
	srv := NewService(gw)

	q := &entity.ListAssignableUsersQuery{Search: "alice", Role: entity.RoleReviewer, Limit: 50}
	res, err := srv.ListAssignable(context.Background(), q)
	require.NoError(t, err)

	// The query is forwarded unchanged to the auth gateway.
	require.Same(t, q, gw.gotQuery)
	require.Equal(t, int64(1), res.Total)
	require.Len(t, res.Users, 1)
	require.Equal(t, int64(50), res.Limit)
}
