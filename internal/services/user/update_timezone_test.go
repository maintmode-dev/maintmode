package user

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
)

func TestUpdateTimezone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	srv := initService(t)

	t.Run("set valid IANA persists", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		require.Nil(t, user.Timezone)

		updated, err := srv.UpdateTimezone(ctx, user.ID, lo.ToPtr("Asia/Nicosia"))
		require.NoError(t, err)
		require.NotNil(t, updated.Timezone)
		require.Equal(t, "Asia/Nicosia", *updated.Timezone)

		got, err := srv.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "Asia/Nicosia", *got.Timezone)
	})

	t.Run("nil resets to auto-detect", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		_, err := srv.UpdateTimezone(ctx, user.ID, lo.ToPtr("Europe/Berlin"))
		require.NoError(t, err)

		updated, err := srv.UpdateTimezone(ctx, user.ID, nil)
		require.NoError(t, err)
		require.Nil(t, updated.Timezone)

		got, err := srv.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.Nil(t, got.Timezone)
	})

	t.Run("empty string resets", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		_, err := srv.UpdateTimezone(ctx, user.ID, lo.ToPtr("UTC"))
		require.NoError(t, err)

		updated, err := srv.UpdateTimezone(ctx, user.ID, lo.ToPtr(""))
		require.NoError(t, err)
		require.Nil(t, updated.Timezone)

		got, err := srv.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.Nil(t, got.Timezone)
	})

	t.Run("whitespace-only resets", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		_, err := srv.UpdateTimezone(ctx, user.ID, lo.ToPtr("Europe/Berlin"))
		require.NoError(t, err)

		updated, err := srv.UpdateTimezone(ctx, user.ID, lo.ToPtr("   "))
		require.NoError(t, err)
		require.Nil(t, updated.Timezone)

		got, err := srv.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.Nil(t, got.Timezone)
	})

	t.Run("invalid IANA rejected, nothing persisted", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)

		_, err := srv.UpdateTimezone(ctx, user.ID, lo.ToPtr("Mars/Phobos"))
		require.ErrorIs(t, err, apperr.ErrInvalidTimezone)

		got, err := srv.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.Nil(t, got.Timezone, "invalid update must not touch the row")
	})
}
