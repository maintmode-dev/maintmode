package usersummary

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestResolveOwner_ColdCacheFetches is the reminder's main scenario: the mention
// is resolved from a background task with no preceding read, so the cache is
// cold. A miss must fetch and return the tags — "cache only, miss → nil" would
// silently disable the feature.
func TestResolveOwner_ColdCacheFetches(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	id := uuid.New()
	var gotCmd *entity.ListUsersCmd
	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd *entity.ListUsersCmd) (*entity.ListUsersResult, error) {
			gotCmd = cmd
			return listUsersResult(&entity.User{
				ID:          id,
				Name:        "Alice",
				Email:       "alice@example.com",
				TelegramTag: lo.ToPtr("@alice"),
				SlackTag:    lo.ToPtr("alice.s"),
			}), nil
		}).
		Times(1)

	got := srv.ResolveOwner(context.Background(), id)
	require.NotNil(t, got)
	require.Equal(t, &entity.UserMention{
		Name:        "Alice",
		TelegramTag: lo.ToPtr("@alice"),
		SlackTag:    lo.ToPtr("alice.s"),
	}, got)

	require.Equal(t, []uuid.UUID{id}, gotCmd.IDs)
	require.Equal(t, int64(1), gotCmd.Limit)
	// Blocked users are not excluded at the query level — the label is applied here.
	require.False(t, gotCmd.ExcludeBlocked)
}

// TestResolveOwner_WarmsCache pins that a resolved owner lands in the shared
// cache: the second call is served without touching the user service.
func TestResolveOwner_WarmsCache(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	id := uuid.New()
	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		Return(listUsersResult(&entity.User{
			ID:          id,
			Name:        "Cached",
			Email:       "c@example.com",
			TelegramTag: lo.ToPtr("@cached"),
		}), nil).
		Times(1)

	first := srv.ResolveOwner(context.Background(), id)
	require.Equal(t, "Cached", first.Name)
	require.Equal(t, lo.ToPtr("@cached"), first.TelegramTag)

	second := srv.ResolveOwner(context.Background(), id)
	require.Equal(t, first, second)
}

// TestResolveOwner_NilID pins the only case that yields nil: no owner to
// mention. Step events legitimately carry the zero id and must not render an
// "Unknown user" owner.
func TestResolveOwner_NilID(t *testing.T) {
	t.Parallel()

	srv, _ := initService(t)

	require.Nil(t, srv.ResolveOwner(context.Background(), uuid.Nil))
}

// TestResolveOwner_AuthDownDegrades pins the no-error invariant: a failing user
// service degrades to the labeled mention, never nil and never a panic.
func TestResolveOwner_AuthDownDegrades(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	id := uuid.New()
	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("auth down"))

	got := srv.ResolveOwner(context.Background(), id)
	require.NotNil(t, got, "a non-nil id must never resolve to nil")
	require.Equal(t, &entity.UserMention{Name: entity.UnknownUserName}, got)
}

// TestResolveOwner_UnknownUserDegrades covers the other unresolved shape: the
// user service answers, but without the requested id (user removed).
func TestResolveOwner_UnknownUserDegrades(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	id := uuid.New()
	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		Return(listUsersResult(&entity.User{ID: uuid.New(), Name: "Someone else"}), nil)

	got := srv.ResolveOwner(context.Background(), id)
	require.NotNil(t, got, "a non-nil id must never resolve to nil")
	require.Equal(t, &entity.UserMention{Name: entity.UnknownUserName}, got)
}

// TestResolveOwner_BlockedUserLabeledWithoutTags pins that a blocked owner is
// named with the "[blocked]" label and never handed out for pinging.
func TestResolveOwner_BlockedUserLabeledWithoutTags(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	id := uuid.New()
	blockedAt := time.Now()
	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		Return(listUsersResult(&entity.User{
			ID:          id,
			Name:        "Bob",
			Email:       "bob@example.com",
			BlockedAt:   &blockedAt,
			TelegramTag: lo.ToPtr("@bob"),
			SlackTag:    lo.ToPtr("bob.s"),
		}), nil)

	got := srv.ResolveOwner(context.Background(), id)
	require.NotNil(t, got)
	require.Equal(t, "Bob [blocked]", got.Name)
	require.Nil(t, got.TelegramTag, "a blocked user must not be pinged")
	require.Nil(t, got.SlackTag, "a blocked user must not be pinged")
}
