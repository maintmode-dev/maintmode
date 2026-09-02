package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

func makeTestUser(ctx context.Context, t *testing.T, impl *Implementation) *entity.User {
	t.Helper()

	user, err := impl.userSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
		ID:    "oauth-" + uuid.NewString(),
		Email: uuid.NewString() + "@test.local",
		Name:  "Provider Test User",
	}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)
	return user
}

func providerCtx(t *testing.T, provider string, body []byte) (*echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, rec := echotest.ContextConfig{
		PathValues: echo.PathValues{{Name: "provider", Value: provider}},
		JSONBody:   body,
	}.ToContextRecorder(t)
	return c, rec
}

func TestConnectProviderHandler(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("missing user in context -> 401", func(t *testing.T) {
		t.Parallel()

		body, _ := json.Marshal(apiauthmodels.ConnectProviderRequest{IDToken: "tok"})
		c, rec := providerCtx(t, "google", body)

		require.NoError(t, impl.ConnectProvider(c))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid provider -> 400", func(t *testing.T) {
		t.Parallel()

		body, _ := json.Marshal(apiauthmodels.ConnectProviderRequest{IDToken: "tok"})
		c, rec := providerCtx(t, "facebook", body)
		xecho.UserToEchoCtx(c, makeTestUser(ctx, t, impl))

		require.NoError(t, impl.ConnectProvider(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing id_token -> 400", func(t *testing.T) {
		t.Parallel()

		body, _ := json.Marshal(apiauthmodels.ConnectProviderRequest{})
		c, rec := providerCtx(t, "github", body)
		xecho.UserToEchoCtx(c, makeTestUser(ctx, t, impl))

		require.NoError(t, impl.ConnectProvider(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("stub provider rejected -> 400", func(t *testing.T) {
		t.Parallel()

		body, _ := json.Marshal(apiauthmodels.ConnectProviderRequest{IDToken: "tok"})
		c, rec := providerCtx(t, "stub", body)
		xecho.UserToEchoCtx(c, makeTestUser(ctx, t, impl))

		require.NoError(t, impl.ConnectProvider(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestDisconnectProviderHandler(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("lockout - only provider -> 400 with message", func(t *testing.T) {
		t.Parallel()

		c, rec := providerCtx(t, "google", nil)
		xecho.UserToEchoCtx(c, makeTestUser(ctx, t, impl))

		require.NoError(t, impl.DisconnectProvider(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "sign-in method")
	})

	t.Run("ok - removes a non-last provider -> 204", func(t *testing.T) {
		t.Parallel()

		user := makeTestUser(ctx, t, impl)
		require.NoError(t, impl.userSrv.LinkIdentity(ctx, user.ID, entity.AuthMethodGithub, &entity.OAuthIDTokenClaims{
			Subject: "gh-" + uuid.NewString(),
			Email:   uuid.NewString() + "@test.local",
			Name:    "GH",
		}))

		c, rec := providerCtx(t, "github", nil)
		xecho.UserToEchoCtx(c, user)

		require.NoError(t, impl.DisconnectProvider(c))
		require.Equal(t, http.StatusNoContent, rec.Code)
	})
}
