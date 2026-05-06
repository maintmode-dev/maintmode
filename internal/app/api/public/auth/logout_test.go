package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xhttp"
)

func TestLogout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		tokenPair := issueTokenPair(ctx, t, impl)

		request := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
		request.AddCookie(&http.Cookie{
			Name:     cookieRefreshTokenName,
			Value:    tokenPair.RefreshToken,
			Path:     cookieRefreshTokenPath,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   cookieRefreshTokenMaxAge, // 30 days
		})
		xhttp.SetBearerToken(request, tokenPair.AccessToken)
		c, rec := echotest.ContextConfig{
			Request: request,
		}.ToContextRecorder(t)

		err := impl.Logout(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		report, err := impl.authSrv.Introspect(ctx, tokenPair.AccessToken)
		require.NoError(t, err)
		require.False(t, report.Active)
	})

	t.Run("empty refresh token", func(t *testing.T) {
		t.Parallel()

		tokenPair := issueTokenPair(ctx, t, impl)

		request := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
		xhttp.SetBearerToken(request, tokenPair.AccessToken)
		c, rec := echotest.ContextConfig{
			Request: request,
		}.ToContextRecorder(t)

		err := impl.Logout(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		report, err := impl.authSrv.Introspect(ctx, tokenPair.AccessToken)
		require.NoError(t, err)
		require.False(t, report.Active)
	})
}

func TestLogoutAll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		tokenPair := issueTokenPair(ctx, t, impl)

		request := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
		request.AddCookie(&http.Cookie{
			Name:     cookieRefreshTokenName,
			Value:    tokenPair.RefreshToken,
			Path:     cookieRefreshTokenPath,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   cookieRefreshTokenMaxAge, // 30 days
		})
		xhttp.SetBearerToken(request, tokenPair.AccessToken)
		c, rec := echotest.ContextConfig{
			Request: request,
		}.ToContextRecorder(t)

		err := impl.LogoutAll(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		report, err := impl.authSrv.Introspect(ctx, tokenPair.AccessToken)
		require.NoError(t, err)
		require.False(t, report.Active)
	})

	t.Run("empty refresh token", func(t *testing.T) {
		t.Parallel()

		tokenPair := issueTokenPair(ctx, t, impl)

		request := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
		xhttp.SetBearerToken(request, tokenPair.AccessToken)
		c, rec := echotest.ContextConfig{
			Request: request,
		}.ToContextRecorder(t)

		err := impl.LogoutAll(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		report, err := impl.authSrv.Introspect(ctx, tokenPair.AccessToken)
		require.NoError(t, err)
		require.False(t, report.Active)
	})
}
