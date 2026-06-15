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
// @Summary OAuth callback (JSON by default, HTML opt-in) (deprecated for production)
// @Deprecated
// @Description **Deprecated for production.** Production clients should use
// @Description the BFF-owned flow: the frontend BFF completes OAuth with
// @Description Google directly and posts the resulting ID token to
// @Description `POST /api/v1/auth/exchange/google`. This endpoint is kept only for
// @Description local prototype / HTML-mode testing.
// @Description
// @Description Handles the redirect from Google after the user grants consent.
// @Description Validates the signed `state` and the nonce cookie, exchanges the
// @Description authorization `code` for tokens, and returns the result.
// @Description
// @Description **Default response — JSON**
// @Description Returns `OAuthCallbackJSONResponse` with the access token, refresh
// @Description token, and `original_uri` in the response body. The refresh token
// @Description is **not** set as a cookie — the caller (typically a server-side
// @Description BFF) persists it in its own session storage. This is the contract
// @Description used by production server-side integrations (NextAuth, BFFs) that
// @Description must never expose tokens to the browser.
// @Description
// @Description JSON mode is selected when **any** of the following holds:
// @Description - `Accept: application/json` request header
// @Description - `Content-Type: application/json` request header
// @Description - the signed state was issued with `oauth_callback_type=json`
// @Description   (see `/login/oauth/google?oauth_callback_type=json`, which is
// @Description   also the default if the parameter is omitted)
// @Description
// @Description **HTML mode (legacy / local testing)** — opt-in via the login
// @Description handler with `?oauth_callback_type=html`. In this mode the
// @Description handler sets an HttpOnly, Secure, SameSite=Strict refresh-token
// @Description cookie on the backend domain and returns an HTML handoff page
// @Description that stashes the access token in `sessionStorage` and redirects
// @Description to `frontendURL + original_uri`. Kept for the prototype frontend
// @Description and local debugging only — production clients should not use it.
// @Description
// @Description Both modes clear the nonce cookie on success.
// @Tags Auth
// @Produce json
// @Produce html
// @Param Accept header string false "Optional override: `application/json` forces JSON mode regardless of the state flag (JSON is already the default)."
// @Param code query string true "OAuth authorization code returned by the provider"
// @Param state query string true "Signed opaque state (HMAC + expiry) issued by /login/oauth/google; carries the HTML/JSON mode flag set at login time"
// @Success 200 {object} apiauthmodels.OAuthCallbackJSONResponse "Default JSON mode: token pair and original_uri in body, no refresh cookie set"
// @Success 200 {string} string "Opt-in HTML mode: handoff page that stores access_token in sessionStorage and redirects; refresh cookie set"
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
		UserAgent:    c.Request().UserAgent(),
	})
	if err != nil {
		xlog.Error(ctx, "authentication failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	if wantsJSON(c, state) {
		// JSON mode: refresh token returned in body; no cookie set so the BFF
		// owns its storage strategy.
		c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
		return c.JSON(http.StatusOK, apiauthmodels.ToOAuthCallbackJSONResponse(pair, state.OriginalURI))
	}

	setRefreshCookie(c, pair.RefreshToken)
	return i.callbackHTML(ctx, c, pair.AccessToken, state.OriginalURI)
}

func wantsJSON(c *echo.Context, state *entity.OAuthState) bool {
	header := c.Request().Header

	return header.Get(echo.HeaderContentType) == echo.MIMEApplicationJSON ||
		strings.Contains(header.Get(echo.HeaderAccept), echo.MIMEApplicationJSON) ||
		state.OAuthCallbackType == entity.OAuthCallbackTypeJSON
}

// callbackHTML returns a small HTML page that stores the access token
// in sessionStorage and redirects to the app root. This avoids exposing
// the token in URL query params or fragments.
func (i *Implementation) callbackHTML(ctx context.Context, c *echo.Context, accessToken, originalURI string) error {
	if originalURI == "" {
		originalURI = "/"
	}
	// original_uri is already normalized to a safe relative path at login entry
	// (safeOriginalURI), and url.JoinPath keeps the result on frontendURL's origin
	// regardless — so no extra validation is needed here.
	redirectURL, err := url.JoinPath(i.frontendURL, safeOriginalURI(originalURI))
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
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
