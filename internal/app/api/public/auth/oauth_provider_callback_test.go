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

func TestGoogleOauthCallback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok full flow", func(t *testing.T) {
		t.Parallel()

		state, nonceCookie := makeGoogleOAuthLogin(t, impl)

		req := httptest.NewRequest(http.MethodGet, "/auth/login/oauth/google/callback", http.NoBody)
		req.AddCookie(nonceCookie)

		c, rec := echotest.ContextConfig{
			Request: req,
			QueryValues: url.Values{
				"state": []string{state},
				"code":  []string{"string"},
			},
		}.ToContextRecorder(t)

		err := impl.GoogleOauthCallback(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		// Verify refresh cookie is set
		refreshCookie, ok := lo.Find(rec.Result().Cookies(), func(c *http.Cookie) bool {
			return c.Name == cookieRefreshTokenName
		})
		require.True(t, ok)
		require.NotEmpty(t, refreshCookie.Value)
		require.Equal(t, cookieRefreshTokenPath, refreshCookie.Path)
		require.True(t, refreshCookie.HttpOnly)
		require.True(t, refreshCookie.Secure)

		// Verify nonce cookie is cleared
		nonceCookie, ok = lo.Find(rec.Result().Cookies(), func(c *http.Cookie) bool {
			return c.Name == cookieNonceName
		})
		require.True(t, ok)
		require.Equal(t, "", nonceCookie.Value)
		require.Equal(t, -1, nonceCookie.MaxAge)
	})

	t.Run("missing nonce cookie", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/auth/login/oauth/google/callback?state=some&code=string", http.NoBody)
		c, rec := echotest.ContextConfig{
			Request: req,
		}.ToContextRecorder(t)

		err := impl.GoogleOauthCallback(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid state", func(t *testing.T) {
		t.Parallel()

		// Set nonce cookie but pass invalid (non-base64) state
		req := httptest.NewRequest(http.MethodGet, "/auth/login/oauth/google/callback?state=invalid-state&code=string", http.NoBody)
		req.AddCookie(&http.Cookie{
			Name:  cookieNonceName,
			Value: "some-nonce",
			Path:  cookieNoncePath,
		})
		c, rec := echotest.ContextConfig{
			Request: req,
		}.ToContextRecorder(t)

		err := impl.GoogleOauthCallback(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("nonce mismatch", func(t *testing.T) {
		t.Parallel()

		// Create a valid state with a different nonce than the cookie
		state := &entity.OAuthState{
			Nonce:       "wrong-nonce",
			OriginalURI: "/",
		}
		stateB64 := state.ToB64Json(ctx)

		req := httptest.NewRequest(http.MethodGet, "/auth/login/oauth/google/callback?state="+stateB64+"&code=string", http.NoBody)
		req.AddCookie(&http.Cookie{
			Name:  cookieNonceName,
			Value: "correct-nonce",
			Path:  cookieNoncePath,
		})
		c, rec := echotest.ContextConfig{
			Request: req,
		}.ToContextRecorder(t)

		err := impl.GoogleOauthCallback(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func makeGoogleOAuthLogin(t *testing.T, impl *Implementation) (string, *http.Cookie) {
	t.Helper()

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet, "/auth/login/oauth/google", http.NoBody),
	}.ToContextRecorder(t)

	err := impl.GoogleOAuthLogin(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)

	// Extract state from redirect URL
	location := rec.Header().Get("Location")
	require.NotEmpty(t, location)

	parsed, err := url.Parse(location)
	require.NoError(t, err)

	state := parsed.Query().Get("state")
	require.NotEmpty(t, state)

	cookieNonce, ok := lo.Find(rec.Result().Cookies(), func(c *http.Cookie) bool {
		return c.Name == cookieNonceName
	})
	require.True(t, ok)
	require.NotEmpty(t, cookieNonce.Value)

	return state, cookieNonce
}
