package user

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_user "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/user"
	"github.com/ruko1202/maintmode/internal/services/license"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// initServiceWithAuditRecorder builds a service whose audit publisher is a mock
// recording every published action, so the tag tests can assert both what lands
// in the trail and — just as importantly — that nothing is published when
// nothing changed. Every other dependency hits the real test DB.
//
// The returned pointer is to a slice the mock appends to; read it after the call
// under test.
func initServiceWithAuditRecorder(t *testing.T) (*Service, *[]audit.Action) {
	t.Helper()

	ctrl := gomock.NewController(t)
	publisher := mock_user.NewMockAuditPublisher(ctrl)

	published := make([]audit.Action, 0)
	publisher.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, action audit.Action) error {
			published = append(published, action)
			return nil
		}).
		AnyTimes()

	srv := NewService(
		dbtx.NewTxManager(db),
		users.NewStore(db),
		useridentities.NewStore(db),
		publisher,
		&fakeTokenRevoker{},
		license.NewNoop(),
		false,
	)

	return srv, &published
}

// tagsChanged asserts exactly one UserTagsChanged action was published and
// returns it.
func tagsChanged(t *testing.T, published []audit.Action) audit.UserTagsChanged {
	t.Helper()

	require.Len(t, published, 1)
	action, ok := published[0].(audit.UserTagsChanged)
	require.True(t, ok, "expected audit.UserTagsChanged, got %T", published[0])

	return action
}

func TestUpdateUserTags(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("single field change audits exactly one entry", func(t *testing.T) {
		t.Parallel()

		srv, published := initServiceWithAuditRecorder(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv)
		*published = nil // drop the role-assignment audit of the actor

		updated, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true,
			TelegramTag:    lo.ToPtr("@ruslan"),
		})
		require.NoError(t, err)
		require.Equal(t, "@ruslan", lo.FromPtr(updated.TelegramTag))
		require.Nil(t, updated.SlackTag)

		got, err := srv.GetByID(ctx, target.ID)
		require.NoError(t, err)
		require.Equal(t, "@ruslan", lo.FromPtr(got.TelegramTag))

		action := tagsChanged(t, *published)
		require.Equal(t, actor.ID, action.Actor.ID)
		require.Equal(t, target.ID, action.Target.ID)
		require.Equal(t, []entity.AuditFieldChange{
			{Field: "telegram_tag", Old: "", New: "@ruslan"},
		}, action.Changes)
	})

	t.Run("both fields change audits two entries with old values", func(t *testing.T) {
		t.Parallel()

		srv, published := initServiceWithAuditRecorder(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv)

		_, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true, TelegramTag: lo.ToPtr("@old_tg"),
			SetSlackTag: true, SlackTag: lo.ToPtr("old.slack"),
		})
		require.NoError(t, err)
		*published = nil

		updated, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true, TelegramTag: lo.ToPtr("@new_tg"),
			SetSlackTag: true, SlackTag: lo.ToPtr("new.slack"),
		})
		require.NoError(t, err)
		require.Equal(t, "@new_tg", lo.FromPtr(updated.TelegramTag))
		require.Equal(t, "new.slack", lo.FromPtr(updated.SlackTag))

		action := tagsChanged(t, *published)
		require.Equal(t, []entity.AuditFieldChange{
			{Field: "telegram_tag", Old: "@old_tg", New: "@new_tg"},
			{Field: "slack_tag", Old: "old.slack", New: "new.slack"},
		}, action.Changes)
	})

	t.Run("resending the same value publishes nothing", func(t *testing.T) {
		t.Parallel()

		srv, published := initServiceWithAuditRecorder(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv)

		cmd := &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true,
			TelegramTag:    lo.ToPtr("@ruslan"),
		}
		_, err := srv.UpdateUserTags(ctx, cmd)
		require.NoError(t, err)
		*published = nil

		updated, err := srv.UpdateUserTags(ctx, cmd)
		require.NoError(t, err)
		require.Equal(t, "@ruslan", lo.FromPtr(updated.TelegramTag))
		require.Empty(t, *published, "unchanged value must not be audited")
	})

	t.Run("only the moved field enters the diff", func(t *testing.T) {
		t.Parallel()

		srv, published := initServiceWithAuditRecorder(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv)

		_, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true, TelegramTag: lo.ToPtr("@stays"),
			SetSlackTag: true, SlackTag: lo.ToPtr("moves"),
		})
		require.NoError(t, err)
		*published = nil

		// Both keys present, but only slack_tag actually differs.
		_, err = srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true, TelegramTag: lo.ToPtr("@stays"),
			SetSlackTag: true, SlackTag: lo.ToPtr("moved"),
		})
		require.NoError(t, err)

		action := tagsChanged(t, *published)
		require.Equal(t, []entity.AuditFieldChange{
			{Field: "slack_tag", Old: "moves", New: "moved"},
		}, action.Changes)
	})

	t.Run("empty command changes nothing and publishes nothing", func(t *testing.T) {
		t.Parallel()

		srv, published := initServiceWithAuditRecorder(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv)

		_, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true,
			TelegramTag:    lo.ToPtr("@kept"),
		})
		require.NoError(t, err)
		*published = nil

		updated, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:  actor,
			UserID: target.ID,
		})
		require.NoError(t, err)
		require.Equal(t, "@kept", lo.FromPtr(updated.TelegramTag))
		require.Empty(t, *published, "an empty patch is not an audited event")
	})

	t.Run("reset to null records the old value and an empty new one", func(t *testing.T) {
		t.Parallel()

		srv, published := initServiceWithAuditRecorder(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv)

		_, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true,
			TelegramTag:    lo.ToPtr("@boss"),
		})
		require.NoError(t, err)
		*published = nil

		// Present-but-nil is the explicit reset, the case a plain pointer cannot
		// express.
		updated, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true,
		})
		require.NoError(t, err)
		require.Nil(t, updated.TelegramTag)

		got, err := srv.GetByID(ctx, target.ID)
		require.NoError(t, err)
		require.Nil(t, got.TelegramTag)

		action := tagsChanged(t, *published)
		require.Equal(t, []entity.AuditFieldChange{
			{Field: "telegram_tag", Old: "@boss", New: ""},
		}, action.Changes)
	})

	t.Run("invalid tag rejects before writing and audits nothing", func(t *testing.T) {
		t.Parallel()

		srv, published := initServiceWithAuditRecorder(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv)

		_, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true,
			TelegramTag:    lo.ToPtr("@legit"),
		})
		require.NoError(t, err)
		*published = nil

		// @channel is a Slack broadcast, rejected there but legal on Telegram.
		_, err = srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true, TelegramTag: lo.ToPtr("@overwritten"),
			SetSlackTag: true, SlackTag: lo.ToPtr("@channel"),
		})
		require.ErrorIs(t, err, apperr.ErrInvalidMessengerTag)
		require.Empty(t, *published)

		// The valid sibling field in the same rejected request must not have
		// been written either.
		got, err := srv.GetByID(ctx, target.ID)
		require.NoError(t, err)
		require.Equal(t, "@legit", lo.FromPtr(got.TelegramTag))
		require.Nil(t, got.SlackTag)
	})

	t.Run("unknown user is not found", func(t *testing.T) {
		t.Parallel()

		srv, published := initServiceWithAuditRecorder(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		*published = nil

		_, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         uuid.New(),
			SetTelegramTag: true,
			TelegramTag:    lo.ToPtr("@ghost"),
		})
		require.ErrorIs(t, err, apperr.ErrUserNotFound)
		require.Empty(t, *published)
	})

	t.Run("unknown user is not found for an empty patch too", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithAuditRecorder(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)

		_, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:  actor,
			UserID: uuid.New(),
		})
		require.ErrorIs(t, err, apperr.ErrUserNotFound)
	})
}

// TestMessengerTagsSurviveOtherUserMutations pins a guarantee nothing else
// asserts: the tags survive every other write to the user row.
//
// Store.Update names its columns explicitly and is shared by role grants,
// blocking and the tag patches alike, so a partial write anywhere on that path
// would blank the tags of whoever it touched. That does not happen today only
// because updateWithApply reads the whole row under FOR UPDATE before writing
// it back — an invariant that lives in the shape of the code and would not
// survive an innocent-looking optimisation of that helper.
//
// The failure would be quiet and remote from its cause: an admin grants someone
// a role, and that person silently stops being mentioned in notifications.
func TestMessengerTagsSurviveOtherUserMutations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const telegram = "@ruslan"

	setTags := func(t *testing.T, srv *Service, actor, target *entity.User) {
		t.Helper()

		updated, err := srv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
			Actor:          actor,
			UserID:         target.ID,
			SetTelegramTag: true,
			TelegramTag:    lo.ToPtr(telegram),
		})
		require.NoError(t, err)
		require.Equal(t, telegram, lo.FromPtr(updated.TelegramTag))
	}

	t.Run("granting a role keeps them", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithAuditRecorder(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv)
		setTags(t, srv, actor, target)

		granted, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			UserID: target.ID,
			Roles:  []entity.Role{entity.RoleEditor},
		})
		require.NoError(t, err)
		require.Contains(t, granted.Roles, entity.RoleEditor, "the grant itself must land")
		require.Equal(t, telegram, lo.FromPtr(granted.TelegramTag),
			"a role grant must not blank the messenger tag")
	})

	t.Run("blocking keeps them", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithAuditRecorder(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv)
		setTags(t, srv, actor, target)

		require.NoError(t, srv.BlockUser(ctx, &entity.BlockUserCmd{
			Actor:  actor,
			UserID: target.ID,
		}))

		// Blocking withholds the tag at render time, but the stored value must
		// stay: unblocking has to restore the mention without the user
		// re-entering anything.
		blocked, err := srv.GetByID(ctx, target.ID)
		require.NoError(t, err)
		require.True(t, blocked.IsBlocked())
		require.Equal(t, telegram, lo.FromPtr(blocked.TelegramTag),
			"blocking must not blank the messenger tag")
	})
}
