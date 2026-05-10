package auth

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
)

// GoogleOauthCallback godoc
// @Summary OAuth callback (HTML or JSON via content negotiation)
// @Description Handles the redirect from Google after the user grants consent.
// @Description Validates the signed `state` and the nonce cookie, exchanges the
// @Description authorization `code` for tokens, and returns the result in one of
// @Description two response modes selected via the `Accept` request header:
// @Description
// @Description **JSON mode** — `Accept: application/json`
// @Description Returns `OAuthCallbackJSONResponse` with the access token, refresh
// @Description token, and `original_uri` in the response body. No `Set-Cookie` for
// @Description the refresh token is issued — the caller (typically a server-side
// @Description BFF) is expected to persist tokens in its own session storage.
// @Description Use this mode for production server-side integrations (NextAuth,
// @Description session-backed BFFs) that must not expose tokens to the browser.
// @Description
// @Description **HTML mode** — any other `Accept` value (default)
// @Description Sets an HttpOnly, Secure, SameSite=Strict refresh-token cookie on
// @Description the backend domain and returns an HTML handoff page that places
// @Description the access token into `sessionStorage` and redirects to
// @Description `frontendURL + original_uri`. Kept for the legacy prototype frontend.
// @Description
// @Description Both modes clear the nonce cookie on success.
// @Tags Auth
// @Produce json
// @Produce html
// @Param Accept header string false "Set to `application/json` to receive the JSON response; omit or use any other value for the HTML handoff."
// @Param code query string true "OAuth authorization code returned by the provider"
// @Param state query string true "Signed opaque state (HMAC + expiry) issued by /login/oauth/google"
// @Success 200 {string} string "HTML mode: handoff page that stores access_token in sessionStorage and redirects; refresh cookie set"
// @Success 200 {object} apiauthmodels.OAuthCallbackJSONResponse "JSON mode: token pair and original_uri in body, no refresh cookie set"
// @Failure 400 {object} httperrors.ErrorResponse "Missing/invalid/expired/tampered state or nonce mismatch"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Header 200 {string} Set-Cookie "Refresh token cookie (HTML mode only; not set in JSON mode)"
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

	state, err := i.stateCodec.Decode(ctx, c.QueryParam("state"))
	if err != nil {
		xlog.Error(ctx, "failed to decode oauth state", xfield.Error(err))
		// Decode returns ErrOAuthStateExpired / ErrOAuthStateTampered; both surface as 400 via httperrors.
		if errors.Is(err, apperr.ErrOAuthStateExpired) || errors.Is(err, apperr.ErrOAuthStateTampered) {
			return httperrors.ToAPIError(c, op, err)
		}
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

	if wantsJSON(c) {
		// JSON mode: refresh token returned in body; no cookie set so the BFF
		// owns its storage strategy.
		c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
		return c.JSON(http.StatusOK, apiauthmodels.ToOAuthCallbackJSONResponse(pair, state.OriginalURI))
	}

	setRefreshCookie(c, pair.RefreshToken)
	return i.callbackHTML(ctx, c, pair.AccessToken, state.OriginalURI)
}

func wantsJSON(c *echo.Context) bool {
	header := c.Request().Header

	return header.Get(echo.HeaderContentType) == echo.MIMEApplicationJSON ||
		strings.Contains(header.Get(echo.HeaderAccept), echo.MIMEApplicationJSON)
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
