package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestGoogleOAuthLogin(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	t.Run("redirects to oauth provider", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodGet, "/auth/login/oauth/google", http.NoBody)
		c, rec := echotest.ContextConfig{
			Request: request,
		}.ToContextRecorder(t)

		err := impl.GoogleOAuthLogin(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusTemporaryRedirect, rec.Code)

		cookieNonce, ok := lo.Find(rec.Result().Cookies(), func(c *http.Cookie) bool {
			return c.Name == cookieNonceName
		})
		require.True(t, ok)
		require.NotEmpty(t, cookieNonce.Value)
		require.Equal(t, cookieNoncePath, cookieNonce.Path)
		require.Equal(t, cookieNonceMaxAge, cookieNonce.MaxAge)
		require.True(t, cookieNonce.HttpOnly)
		require.True(t, cookieNonce.Secure)

		location := rec.Header().Get("Location")
		require.NotEmpty(t, location)
		require.Contains(t, location, cfg.OauthProviders.Stub.RedirectURL)

		u, err := url.Parse(location)
		require.NoError(t, err)

		query := u.Query()
		b64State := query.Get("state")
		assertState(t, &entity.OAuthState{
			Nonce:       cookieNonce.Value,
			OriginalURI: "",
		}, b64State)
	})

	t.Run("preserves original_uri in state", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodGet, "/auth/login/oauth/google?original_uri=/calendar", http.NoBody)
		c, rec := echotest.ContextConfig{
			Request: request,
			QueryValues: url.Values{
				"original_uri": {"/calendar"},
			},
		}.ToContextRecorder(t)

		err := impl.GoogleOAuthLogin(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusTemporaryRedirect, rec.Code)

		cookieNonce, ok := lo.Find(rec.Result().Cookies(), func(c *http.Cookie) bool {
			return c.Name == cookieNonceName
		})
		require.True(t, ok)
		require.NotEmpty(t, cookieNonce.Value)
		require.Equal(t, cookieNoncePath, cookieNonce.Path)
		require.Equal(t, cookieNonceMaxAge, cookieNonce.MaxAge)
		require.True(t, cookieNonce.HttpOnly)
		require.True(t, cookieNonce.Secure)

		location := rec.Header().Get("Location")
		require.NotEmpty(t, location)
		require.Contains(t, location, cfg.OauthProviders.Stub.RedirectURL)

		u, err := url.Parse(location)
		require.NoError(t, err)

		query := u.Query()
		b64State := query.Get("state")
		assertState(t, &entity.OAuthState{
			Nonce:       cookieNonce.Value,
			OriginalURI: "/calendar",
		}, b64State)
	})
}

func assertState(t *testing.T, expected *entity.OAuthState, b64ActualState string) {
	t.Helper()

	require.NotEmpty(t, b64ActualState)

	state, err := entity.NewOAuthStateFromB64Json(b64ActualState)
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, expected, state)
}
