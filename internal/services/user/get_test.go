package user

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
)

func TestGetRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		roles, err := srv.GetRoles(ctx, uuid.New())
		require.Nil(t, roles)
		require.ErrorIs(t, err, apperr.ErrUserNotFound)
	})
}
