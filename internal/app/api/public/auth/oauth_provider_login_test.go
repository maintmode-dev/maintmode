package auth

import (
	"context"
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
		assertState(t, impl, &entity.OAuthState{
			Nonce: cookieNonce.Value,
			// No original_uri supplied → normalized to the default landing path
			// at entry by safeOriginalURI.
			OriginalURI: "/",
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
		assertState(t, impl, &entity.OAuthState{
			Nonce:       cookieNonce.Value,
			OriginalURI: "/calendar",
		}, b64State)
	})

	t.Run("sanitizes unsafe original_uri before it enters the signed state", func(t *testing.T) {
		t.Parallel()

		// A scheme-relative URI is an open-redirect vector; it must be reduced to a
		// same-origin path before being baked into the signed state, so the callback
		// can never hand the frontend "//evil.com" as a navigation target.
		request := httptest.NewRequest(http.MethodGet, "/auth/login/oauth/google?original_uri=//evil.com", http.NoBody)
		c, rec := echotest.ContextConfig{
			Request: request,
			QueryValues: url.Values{
				"original_uri": {"//evil.com"},
			},
		}.ToContextRecorder(t)

		err := impl.GoogleOAuthLogin(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusTemporaryRedirect, rec.Code)

		cookieNonce, ok := lo.Find(rec.Result().Cookies(), func(c *http.Cookie) bool {
			return c.Name == cookieNonceName
		})
		require.True(t, ok)

		u, err := url.Parse(rec.Header().Get("Location"))
		require.NoError(t, err)

		assertState(t, impl, &entity.OAuthState{
			Nonce:       cookieNonce.Value,
			OriginalURI: "/",
		}, u.Query().Get("state"))
	})
}

func assertState(t *testing.T, impl *Implementation, expected *entity.OAuthState, encodedState string) {
	t.Helper()

	require.NotEmpty(t, encodedState)

	state, err := impl.stateCodec.Decode(context.Background(), encodedState)
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, expected.Nonce, state.Nonce)
	require.Equal(t, expected.OriginalURI, state.OriginalURI)
	require.Greater(t, state.ExpiresAt, int64(0))
}
