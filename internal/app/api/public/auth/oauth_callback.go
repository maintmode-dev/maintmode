package auth

import (
	"net/url"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// OAuthDanceCallback godoc
// @Summary Complete the backend-driven OAuth dance
// @Description Verifies the signed state carried in the oauth_state cookie, exchanges the authorization code for tokens using the client secret and the PKCE verifier from the oauth_code_verifier cookie, resolves the user and redirects to the frontend with a one-time code. Both cookies are cleared on every exit. Always answers 302, success or failure: the user's browser is sitting on this URL, so a JSON error body would be a dead end.
// @Tags Auth
// @Produce json
// @Param provider path string true "Configured provider instance name, e.g. google"
// @Param code query string false "Authorization code from the provider"
// @Param state query string false "The state issued by /start"
// @Param error query string false "Error reported by the provider"
// @Success 302 "Redirect to the frontend carrying a one-time code"
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Router /api/v1/login/oauth/{provider}/callback [get]
func (i *Implementation) OAuthDanceCallback(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.OAuthDance.Callback")
	defer span.End()

	meta := &entity.AuditMetadata{IP: c.RealIP(), UserAgent: c.Request().UserAgent()}

	// STEP 0, before any check: read both cookies and queue their removal.
	//
	// Everything below works from these two locals. Re-reading the request after
	// the expiry has been queued is how a handler ends up with two different
	// answers for one cookie; and doing the clearing here rather than at the
	// point of success is what makes "cleared on every exit" true by
	// construction, instead of in seven branches that each had to remember.
	//
	// What it buys is bounded: a COOPERATING browser cannot carry a spent dance
	// into the next attempt. An attacker replaying with curl never honors
	// Set-Cookie, and the signature's deadline is what covers them.
	signature := danceCookieValue(c, oauthStateCookie)
	verifier := danceCookieValue(c, oauthVerifierCookie)
	invitationHandle := danceCookieValue(c, oauthInvitationCookie)

	i.expireDanceCookies(c)

	code, err := i.authSrv.CompleteDance(ctx, entity.DanceCallback{
		Provider:         c.Param("provider"),
		ProviderError:    c.QueryParam(paramError),
		State:            c.QueryParam(paramState),
		Code:             c.QueryParam(paramCode),
		StateSignature:   signature,
		Verifier:         verifier,
		InvitationHandle: invitationHandle,
	}, meta)
	if err != nil {
		xlog.Warn(ctx, "oauth dance did not complete", xfield.Error(err))
		return i.redirectFailure(c, danceFailureCode(err))
	}

	return i.redirectHome(c, url.Values{paramCode: {code}})
}
