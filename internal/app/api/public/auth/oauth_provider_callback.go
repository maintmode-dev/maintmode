package auth

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
)

// GoogleOauthCallback godoc
// @Summary OAuth callback
// @Description Handles OAuth provider callback, sets refresh token cookie, and returns redirect HTML with access token handoff.
// @Tags Auth
// @Produce html
// @Param code query string true "Authorization code"
// @Param state query string true "Opaque state with nonce"
// @Success 200 {string} string "HTML page for frontend redirect"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid state/code"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/login/oauth/google/callback [get]
// GoogleOauthCallback handles the redirect from Google after consent.
func (i *Implementation) GoogleOauthCallback(c *echo.Context) error {
	return i.oauthCallback(c, entity.OAuthProviderGoogle)
}
func (i *Implementation) oauthCallback(c *echo.Context, provider entity.OAuthProvider) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), fmt.Sprintf("api.Auth.%s.OauthCallback", provider))
	defer span.End()
	op := "oauth provider callback"

	nonceCookie, err := c.Cookie(cookieNonceName)
	if err != nil {
		xlog.Error(ctx, "failed to extract nonce", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(apperr.ErrInvalidOAuthState))
	}

	state, err := i.parseState(c.QueryParam("state"), provider)
	if err != nil {
		xlog.Error(ctx, "failed to unmarshal state", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(apperr.ErrInvalidOAuthState))
	}

	if nonceCookie.Value != state.Nonce {
		xlog.Error(ctx, "mismatch state",
			xfield.String("query state nonce", state.Nonce),
			xfield.String("cookie nonce", nonceCookie.Value),
		)
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(apperr.ErrInvalidOAuthState))
	}

	clearCookieNonce(c)

	pair, err := i.authSrv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
		Provider:     provider,
		CallbackCode: c.QueryParam("code"),
		ClientIP:     c.RealIP(),
	})
	if err != nil {
		xlog.Error(ctx, "authentication failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	setRefreshCookie(c, pair.RefreshToken)
	return i.callbackHTML(ctx, c, pair.AccessToken, state.OriginalURI)
}

func (i *Implementation) parseState(stateData string, provider entity.OAuthProvider) (*entity.OAuthState, error) {
	switch provider {
	case entity.OAuthProviderGoogle:
		return entity.NewOAuthStateFromB64Json(stateData)
	default:
		return nil, fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, provider)
	}
}

// callbackHTML returns a small HTML page that stores the access token
// in sessionStorage and redirects to the app root. This avoids exposing
// the token in URL query params or fragments.
func (i *Implementation) callbackHTML(ctx context.Context, c *echo.Context, accessToken, originalURI string) error {
	if originalURI == "" {
		originalURI = "/"
	}
	redirectURL, err := url.JoinPath(i.frontendURL, originalURI)
	if err != nil {
		xlog.Error(ctx, "failed to join frontend URL", xfield.Error(err))
		return err
	}

	c.Response().Header().Set("Cache-Control", "no-store")

	return c.HTML(http.StatusOK, fmt.Sprintf(`<!DOCTYPE html>
<html><body><script>
sessionStorage.setItem('access_token','%s');
window.location.replace('%s');
</script></body></html>`,
		html.EscapeString(accessToken), html.EscapeString(redirectURL)),
	)
}

var (
	cookieRefreshTokenName   = fmt.Sprintf("%s-refresh_token", config.GetAuthAppBuildMeta().AppName)
	cookieRefreshTokenMaxAge = 30 * 24 * 60 * 60 // 30 days
	cookieRefreshTokenPath   = "/auth"
)

func setRefreshCookie(c *echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     cookieRefreshTokenName,
		Value:    token,
		Path:     cookieRefreshTokenPath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   cookieRefreshTokenMaxAge, // 30 days
	})
}

func clearRefreshCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     cookieRefreshTokenName,
		Value:    "",
		Path:     cookieRefreshTokenPath,
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	})
}
