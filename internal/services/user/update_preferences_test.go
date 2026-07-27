package user

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/apperr"
)

func TestUpdatePreferences(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	srv := initService(t)

	t.Run("set valid IANA persists", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		require.Nil(t, user.Timezone)

		updated, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetTimezone: true,
			Timezone:    lo.ToPtr("Asia/Nicosia"),
		})
		require.NoError(t, err)
		require.Equal(t, "Asia/Nicosia", lo.FromPtr(updated.Timezone))

		got, err := srv.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "Asia/Nicosia", lo.FromPtr(got.Timezone))
	})

	t.Run("present nil timezone resets to auto-detect", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		_, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetTimezone: true, Timezone: lo.ToPtr("Europe/Berlin"),
		})
		require.NoError(t, err)

		updated, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{SetTimezone: true})
		require.NoError(t, err)
		require.Nil(t, updated.Timezone)

		got, err := srv.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.Nil(t, got.Timezone)
	})

	t.Run("empty and whitespace-only timezone reset", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{"", "   "} {
			user := makeUser(ctx, t, srv)
			_, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
				SetTimezone: true, Timezone: lo.ToPtr("Europe/Berlin"),
			})
			require.NoError(t, err)

			updated, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
				SetTimezone: true, Timezone: lo.ToPtr(value),
			})
			require.NoError(t, err)
			require.Nil(t, updated.Timezone)

			got, err := srv.GetByID(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, got.Timezone)
		}
	})

	t.Run("invalid IANA rejected, nothing persisted", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)

		_, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetTimezone: true, Timezone: lo.ToPtr("Mars/Phobos"),
		})
		require.ErrorIs(t, err, apperr.ErrInvalidTimezone)

		got, err := srv.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.Nil(t, got.Timezone, "invalid update must not touch the row")
	})

	t.Run("absent field is left unchanged", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		_, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetTimezone: true, Timezone: lo.ToPtr("Asia/Nicosia"),
		})
		require.NoError(t, err)

		updated, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetTelegramTag: true, TelegramTag: lo.ToPtr("@ruslan"),
		})
		require.NoError(t, err)
		require.Equal(t, "Asia/Nicosia", lo.FromPtr(updated.Timezone))
		require.Equal(t, "@ruslan", lo.FromPtr(updated.TelegramTag))

		got, err := srv.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "Asia/Nicosia", lo.FromPtr(got.Timezone))
		require.Equal(t, "@ruslan", lo.FromPtr(got.TelegramTag))
	})

	t.Run("empty command touches nothing", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		_, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetTimezone: true, Timezone: lo.ToPtr("Asia/Nicosia"),
			SetSlackTag: true, SlackTag: lo.ToPtr("ruslan"),
		})
		require.NoError(t, err)

		updated, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{})
		require.NoError(t, err)
		require.Equal(t, "Asia/Nicosia", lo.FromPtr(updated.Timezone))
		require.Equal(t, "ruslan", lo.FromPtr(updated.SlackTag))
	})

	t.Run("tags persist verbatim", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		updated, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetTelegramTag: true, TelegramTag: lo.ToPtr("@ruslan"),
			SetSlackTag: true, SlackTag: lo.ToPtr("ruslan"),
		})
		require.NoError(t, err)
		require.Equal(t, "@ruslan", lo.FromPtr(updated.TelegramTag))
		require.Equal(t, "ruslan", lo.FromPtr(updated.SlackTag))

		got, err := srv.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "@ruslan", lo.FromPtr(got.TelegramTag))
		require.Equal(t, "ruslan", lo.FromPtr(got.SlackTag))
	})

	t.Run("present nil and empty tags reset", func(t *testing.T) {
		t.Parallel()

		for _, value := range []*string{nil, lo.ToPtr(""), lo.ToPtr("  ")} {
			user := makeUser(ctx, t, srv)
			_, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
				SetTelegramTag: true, TelegramTag: lo.ToPtr("@ruslan"),
				SetSlackTag: true, SlackTag: lo.ToPtr("ruslan"),
			})
			require.NoError(t, err)

			updated, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
				SetTelegramTag: true, TelegramTag: value,
				SetSlackTag: true, SlackTag: value,
			})
			require.NoError(t, err)
			require.Nil(t, updated.TelegramTag)
			require.Nil(t, updated.SlackTag)

			got, err := srv.GetByID(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, got.TelegramTag)
			require.Nil(t, got.SlackTag)
		}
	})

	t.Run("reserved value rejected on both transports", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)

		_, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetSlackTag: true, SlackTag: lo.ToPtr("@channel"),
		})
		require.ErrorIs(t, err, apperr.ErrInvalidMessengerTag)

		_, err = srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetTelegramTag: true, TelegramTag: lo.ToPtr("@channel"),
		})
		require.ErrorIs(t, err, apperr.ErrInvalidMessengerTag,
			"broadcast words are refused on both transports")

		// The reservation is exact: a handle that merely starts with one is fine.
		updated, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetTelegramTag: true, TelegramTag: lo.ToPtr("@channels"),
		})
		require.NoError(t, err)
		require.Equal(t, "@channels", lo.FromPtr(updated.TelegramTag))
	})

	t.Run("invalid tag rejected, whole patch unapplied", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)

		// The valid timezone shares the command with the rejected tag: it must
		// not land, because validation runs before the single write.
		_, err := srv.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetTimezone: true, Timezone: lo.ToPtr("Asia/Nicosia"),
			SetTelegramTag: true, TelegramTag: lo.ToPtr("@"),
		})
		require.ErrorIs(t, err, apperr.ErrInvalidMessengerTag)

		got, err := srv.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.Nil(t, got.Timezone)
		require.Nil(t, got.TelegramTag)
	})
}
