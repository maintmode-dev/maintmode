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

	dance, _ := json.Marshal(apiauthmodels.ConnectProviderRequest{Mode: apiauthmodels.ConnectProviderDanceMode})

	t.Run("missing user in context -> 401", func(t *testing.T) {
		t.Parallel()

		c, rec := providerCtx(t, "google", dance)

		require.NoError(t, impl.ConnectProvider(c))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid provider -> 400", func(t *testing.T) {
		t.Parallel()

		c, rec := providerCtx(t, "facebook", dance)
		xecho.UserToEchoCtx(c, makeTestUser(ctx, t, impl))

		require.NoError(t, impl.ConnectProvider(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing mode -> 400", func(t *testing.T) {
		t.Parallel()

		c, rec := providerCtx(t, "github", []byte(`{}`))
		xecho.UserToEchoCtx(c, makeTestUser(ctx, t, impl))

		require.NoError(t, impl.ConnectProvider(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "cannot be blank")
	})

	// The id_token flow is gone: a client credential could be any token for the
	// provider, held by any application. An old body carrying one is refused,
	// never linked.
	t.Run("legacy id_token body -> 400", func(t *testing.T) {
		t.Parallel()

		user := makeTestUser(ctx, t, impl)
		c, rec := providerCtx(t, "github", []byte(`{"id_token":"tok"}`))
		xecho.UserToEchoCtx(c, user)

		require.NoError(t, impl.ConnectProvider(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "cannot be blank")

		providers, err := impl.userSrv.ListConnectedProviders(ctx, user.ID)
		require.NoError(t, err)
		require.NotContains(t, providers, entity.AuthMethodGithub)
	})

	// An unrecognized mode is refused rather than ignored.
	t.Run("unsupported mode -> 400", func(t *testing.T) {
		t.Parallel()

		body, _ := json.Marshal(apiauthmodels.ConnectProviderRequest{Mode: "waltz"})
		c, rec := providerCtx(t, "github", body)
		xecho.UserToEchoCtx(c, makeTestUser(ctx, t, impl))

		require.NoError(t, impl.ConnectProvider(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "must be a valid value")
	})

	t.Run("stub provider rejected -> 400", func(t *testing.T) {
		t.Parallel()

		c, rec := providerCtx(t, "stub", dance)
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

		// No built-in method offered, so nothing but the provider lets the
		// user in and the last-provider guard is what answers.
		impl := initImplWithoutBuiltins(t)

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
