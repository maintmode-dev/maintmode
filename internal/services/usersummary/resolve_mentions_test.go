package usersummary

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestResolveMentions_EmptyInputSkipsLookup pins that no ids means no query at
// all. Most maintenances carry no mentions, so a lookup here would add a
// user-service round trip to every notification for nothing.
func TestResolveMentions_EmptyInputSkipsLookup(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	mocks.users.EXPECT().ListUsers(gomock.Any(), gomock.Any()).Times(0)

	assert.Nil(t, srv.ResolveMentions(context.Background(), nil))
	assert.Nil(t, srv.ResolveMentions(context.Background(), []uuid.UUID{}))
	// A list of nothing but zero ids is empty after filtering, so it must also
	// stay off the wire.
	assert.Nil(t, srv.ResolveMentions(context.Background(), []uuid.UUID{uuid.Nil, uuid.Nil}))
}

// TestResolveMentions_OneBatchForManyIDs is the batching guard: N mentions must
// cost exactly one user-service call, not N.
func TestResolveMentions_OneBatchForManyIDs(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	first, second, third := uuid.New(), uuid.New(), uuid.New()

	var gotCmd *entity.ListUsersCmd
	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd *entity.ListUsersCmd) (*entity.ListUsersResult, error) {
			gotCmd = cmd

			return listUsersResult(
				&entity.User{ID: first, Name: "Alice", TelegramTag: lo.ToPtr("@alice")},
				&entity.User{ID: second, Name: "Bob", SlackTag: lo.ToPtr("bob.s")},
				&entity.User{ID: third, Name: "Carol"},
			), nil
		}).
		Times(1)

	got := srv.ResolveMentions(context.Background(), []uuid.UUID{first, second, third})

	require.Len(t, got, 3)
	assert.Equal(t, "Alice", got[0].Name)
	assert.Equal(t, "Bob", got[1].Name)
	assert.Equal(t, "Carol", got[2].Name)
	assert.Equal(t, lo.ToPtr("@alice"), got[0].TelegramTag)
	assert.Equal(t, lo.ToPtr("bob.s"), got[1].SlackTag)

	// Limit must be bound to the id count: ListUsers does not default it, so a
	// missing Limit emits LIMIT 0 and resolves nobody.
	require.Equal(t, int64(3), gotCmd.Limit)
	require.ElementsMatch(t, []uuid.UUID{first, second, third}, gotCmd.IDs)
}

// TestResolveMentions_PreservesInputOrderAndDedupes pins the contract: the
// rendered line follows the order the ids came in, each person appears once, and
// the zero id is dropped.
func TestResolveMentions_PreservesInputOrderAndDedupes(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	first, second := uuid.New(), uuid.New()

	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		Return(listUsersResult(
			&entity.User{ID: first, Name: "Alice"},
			&entity.User{ID: second, Name: "Bob"},
		), nil).
		Times(1)

	got := srv.ResolveMentions(context.Background(), []uuid.UUID{
		second, uuid.Nil, first, second, first,
	})

	require.Len(t, got, 2)
	assert.Equal(t, "Bob", got[0].Name, "input order wins: second id was listed first")
	assert.Equal(t, "Alice", got[1].Name)
}

// TestResolveMentions_BlockedUsersAreExcluded pins the deliberate asymmetry with
// ResolveOwner, which keeps a blocked owner and labels them "[blocked]".
func TestResolveMentions_BlockedUsersAreExcluded(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	blocked, active := uuid.New(), uuid.New()
	blockedAt := time.Now()

	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		Return(listUsersResult(
			&entity.User{ID: blocked, Name: "Blocked Bob", BlockedAt: &blockedAt, SlackTag: lo.ToPtr("bob.s")},
			&entity.User{ID: active, Name: "Alice"},
		), nil).
		Times(1)

	got := srv.ResolveMentions(context.Background(), []uuid.UUID{blocked, active})

	require.Len(t, got, 1)
	assert.Equal(t, "Alice", got[0].Name)
	for _, m := range got {
		assert.NotContains(t, m.Name, "Blocked Bob")
		assert.NotContains(t, m.Name, "[blocked]")
	}
}

// TestResolveMentions_UnresolvedIsDropped pins that an id auth cannot name is
// left out of the line entirely: there is no handle to ping and no name to
// print, so a placeholder would be noise.
func TestResolveMentions_UnresolvedIsDropped(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	known, unknown := uuid.New(), uuid.New()

	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		Return(listUsersResult(&entity.User{ID: known, Name: "Alice"}), nil).
		Times(1)

	got := srv.ResolveMentions(context.Background(), []uuid.UUID{known, unknown})

	// The unknown id yields no handle and no name, so it is left out rather
	// than rendered as a placeholder that pings nobody.
	require.Len(t, got, 1)
	assert.Equal(t, "Alice", got[0].Name)
}

// TestResolveMentions_AuthFailureDegrades is the no-error contract in action:
// dispatchSync has no idempotency key, so a resolver that could fail would turn
// "we could not name someone" into a goque retry that re-sends to every target.
// Auth being down must therefore degrade, never propagate.
func TestResolveMentions_AuthFailureDegrades(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	first, second := uuid.New(), uuid.New()

	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("auth unavailable")).
		Times(1)

	got := srv.ResolveMentions(context.Background(), []uuid.UUID{first, second})

	// Degrades to an empty line rather than propagating: nobody can be named,
	// and the notification still goes out.
	assert.Empty(t, got)
}
