package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
)

func TestRefresh(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok via cookie", func(t *testing.T) {
		t.Parallel()

		tokenPair := issueTokenPair(ctx, t, impl)

		request := httptest.NewRequest(http.MethodPost, "/auth/refresh", http.NoBody)
		request.AddCookie(&http.Cookie{
			Name:  cookieRefreshTokenName,
			Value: tokenPair.RefreshToken,
		})
		c, rec := echotest.ContextConfig{
			Request: request,
		}.ToContextRecorder(t)

		err := impl.Refresh(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apiauthmodels.TokenPairResponse](t, rec.Body)
		require.NotEmpty(t, resp.AccessToken)
		require.NotEmpty(t, resp.RefreshToken)
		require.Greater(t, resp.ExpiresIn, 0)

		refreshCookie, ok := lo.Find(rec.Result().Cookies(), func(cookie *http.Cookie) bool {
			return cookie.Name == cookieRefreshTokenName
		})
		require.True(t, ok)
		require.Equal(t, resp.RefreshToken, refreshCookie.Value)
		require.Equal(t, cookieRefreshTokenPath, refreshCookie.Path)
	})

	t.Run("ok via json body fallback", func(t *testing.T) {
		t.Parallel()

		tokenPair := issueTokenPair(ctx, t, impl)

		body := refreshTokenJSONRequest{RefreshToken: tokenPair.RefreshToken}
		request := httptest.NewRequest(http.MethodPost, "/auth/refresh", http.NoBody)
		c, rec := echotest.ContextConfig{
			Request:  request,
			JSONBody: testjsonudils.AnyToJSONBytes(t, body),
		}.ToContextRecorder(t)

		err := impl.Refresh(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apiauthmodels.TokenPairResponse](t, rec.Body)
		require.NotEmpty(t, resp.AccessToken)
	})

	t.Run("empty refresh token", func(t *testing.T) {
		t.Parallel()

		body := refreshTokenJSONRequest{RefreshToken: ""}
		request := httptest.NewRequest(http.MethodPost, "/auth/refresh", http.NoBody)
		c, rec := echotest.ContextConfig{
			Request:  request,
			JSONBody: testjsonudils.AnyToJSONBytes(t, body),
		}.ToContextRecorder(t)

		err := impl.Refresh(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid refresh token", func(t *testing.T) {
		t.Parallel()

		body := refreshTokenJSONRequest{RefreshToken: "invalid-token-value"}
		request := httptest.NewRequest(http.MethodPost, "/auth/refresh", http.NoBody)
		c, rec := echotest.ContextConfig{
			Request:  request,
			JSONBody: testjsonudils.AnyToJSONBytes(t, body),
		}.ToContextRecorder(t)

		err := impl.Refresh(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("refresh token reuse rejected", func(t *testing.T) {
		t.Parallel()

		tokenPair := issueTokenPair(ctx, t, impl)

		// First refresh — succeeds
		c1, rec1 := echotest.ContextConfig{
			Request: httptest.NewRequest(http.MethodPost, "/auth/refresh", http.NoBody),
			JSONBody: testjsonudils.AnyToJSONBytes(t, refreshTokenJSONRequest{
				RefreshToken: tokenPair.RefreshToken,
			}),
		}.ToContextRecorder(t)

		err := impl.Refresh(c1)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec1.Code)

		// Second refresh with same token — should fail (reuse detected)
		c2, rec2 := echotest.ContextConfig{
			Request: httptest.NewRequest(http.MethodPost, "/auth/refresh", http.NoBody),
			JSONBody: testjsonudils.AnyToJSONBytes(t, refreshTokenJSONRequest{
				RefreshToken: tokenPair.RefreshToken,
			}),
		}.ToContextRecorder(t)

		err = impl.Refresh(c2)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, rec2.Code)
	})
}
