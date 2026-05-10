package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	statecodec "github.com/ruko1202/maintmode/internal/services/state_codec"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestGoogleOauthCallback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok full flow", func(t *testing.T) {
		t.Parallel()

		state, nonceCookie := makeGoogleOAuthLogin(t, impl, entity.OAuthCallbackTypeHTML)

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

		// Encode a valid state with a different nonce than the cookie.
		encoded, err := impl.stateCodec.Encode(ctx, &entity.OAuthState{
			Nonce:       "wrong-nonce",
			OriginalURI: "/",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/auth/login/oauth/google/callback?state="+encoded+"&code=string", http.NoBody)
		req.AddCookie(&http.Cookie{
			Name:  cookieNonceName,
			Value: "correct-nonce",
			Path:  cookieNoncePath,
		})
		c, rec := echotest.ContextConfig{
			Request: req,
		}.ToContextRecorder(t)

		err = impl.GoogleOauthCallback(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("ok JSON mode", func(t *testing.T) {
		t.Parallel()

		state, nonceCookie := makeGoogleOAuthLogin(t, impl, entity.OAuthCallbackTypeJSON)

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
		require.Contains(t, rec.Header().Get("Content-Type"), "application/json")

		// In JSON mode the refresh token must NOT be set as a cookie.
		_, hasRefresh := lo.Find(rec.Result().Cookies(), func(c *http.Cookie) bool {
			return c.Name == cookieRefreshTokenName
		})
		require.False(t, hasRefresh, "refresh token cookie must not be set in JSON mode")

		// Nonce cookie still cleared.
		clearedNonce, ok := lo.Find(rec.Result().Cookies(), func(c *http.Cookie) bool {
			return c.Name == cookieNonceName
		})
		require.True(t, ok)
		require.Equal(t, "", clearedNonce.Value)
		require.Equal(t, -1, clearedNonce.MaxAge)

		var body apiauthmodels.OAuthCallbackJSONResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.NotEmpty(t, body.Token.AccessToken)
		require.NotEmpty(t, body.Token.RefreshToken)
		require.Greater(t, body.Token.ExpiresIn, 0)
	})

	t.Run("expired state", func(t *testing.T) {
		t.Parallel()

		// Build a codec that has already expired (ttl < 0) so any encode is dead on arrival.
		expiredCodec := statecodec.NewService(
			[]byte(cfg.JWT.OAuthStateSigningKey),
			-time.Hour,
		)
		encoded, err := expiredCodec.Encode(ctx, &entity.OAuthState{Nonce: "n", OriginalURI: "/"})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/auth/login/oauth/google/callback?state="+encoded+"&code=string", http.NoBody)
		req.AddCookie(&http.Cookie{
			Name:  cookieNonceName,
			Value: "n",
			Path:  cookieNoncePath,
		})
		c, rec := echotest.ContextConfig{
			Request: req,
		}.ToContextRecorder(t)

		err = impl.GoogleOauthCallback(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func makeGoogleOAuthLogin(t *testing.T, impl *Implementation, oauthCallbackType string) (string, *http.Cookie) {
	t.Helper()

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet, "/auth/login/oauth/google", http.NoBody),
		QueryValues: url.Values{
			"original_uri":        {"/calendar"},
			"oauth_callback_type": {oauthCallbackType},
		},
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
