package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

func TestMe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		user, err := impl.userSrv.GetOrCreateByOAuthInfo(ctx, &entity.OAuthProviderUserInfo{
			ID:    "oauth-" + uuid.NewString(),
			Email: uuid.NewString() + "@test.local",
			Name:  "Me Test User",
		})
		require.NoError(t, err)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, user)

		err = impl.Me(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apiauthmodels.MeResponse](t, rec.Body)
		require.Equal(t, user.ID.String(), got.ID)
		require.Equal(t, user.Email, got.Email)
		require.Equal(t, user.Name, got.DisplayName)
		require.Equal(t, string(entity.OAuthProviderGoogle), got.OAuthProvider)
	})

	t.Run("missing user in context", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.Me(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
