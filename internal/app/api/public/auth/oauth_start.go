package auth

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// StartOAuthDance godoc
// @Summary Begin the backend-driven OAuth dance
// @Description Mints CSRF state and a PKCE verifier, hands the browser the state's signature and the verifier as httpOnly cookies, and redirects to the provider. The dance itself is stored nowhere: those two cookies carry it. An optional invitation token is exchanged for an opaque single-use handle, which is what the third httpOnly cookie carries; the handle-to-invitation mapping is the one piece held server-side, so the raw token -- a multi-day bearer credential -- never travels to the provider, the browser, or a log line. An unknown token is not reported here: it yields a handle like any other and simply fails to apply at the callback. Answers 302 on success; a provider outside the supported set answers 400. This is the backend-owned alternative to the BFF flow behind /login/oauth/exchange/google, which stays live.
// @Tags Auth
// @Produce json
// @Param provider path string true "Provider id" Enums(google)
// @Param invitation query string false "Invitation token, when signing in from an invitation link"
// @Success 302 "Redirect to the provider's authorization endpoint"
// @Failure 400 {object} httperrors.ErrorResponse "Unsupported provider"
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/login/oauth/{provider}/start [get]
func (i *Implementation) StartOAuthDance(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.OAuthDance.Start")
	defer span.End()
	op := "oauth dance start"

	// The allow-list runs FIRST, before any secret is minted or stored.
	//
	// Ordering matters twice over: an unknown provider must cost nothing, and
	// this route's {provider} segment shares a path space with the static
	// /login/oauth/exchange/google, so an unchecked parameter is how a request
	// for one route ends up handled by another.
	provider, ok := entity.DanceProvider(c.Param("provider"))
	if !ok {
		xlog.Warn(ctx, "oauth dance requested for an unsupported provider",
			xfield.String("provider", c.Param("provider")))

		// JSON, not a redirect. There is no trusted frontend target to send an
		// error to at this point, and echoing an arbitrary path segment into a
		// Location header is how open redirects start.
		return httperrors.ToAPIError(c, op, fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, c.Param("provider")))
	}

	// The invitation token, when this dance began from an invitation link. It is
	// read here and handed straight to the service: it never reaches the
	// provider, the redirect, or a log line (the request sanitizer masks it).
	dance, err := i.authSrv.StartDance(ctx, provider, c.QueryParam("invitation"))
	if err != nil {
		xlog.Error(ctx, "failed to start the oauth dance", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// The dance proper is stored nowhere: the browser carries it as the state's
	// SIGNATURE and the verifier itself, both httpOnly. (An invited dance is the
	// exception, and only for the invitation — see the handle cookie below.)
	//
	// The provider receives the plaintext state, so whoever observes the
	// redirect URL holds one half of the pair and not the other. That asymmetry
	// is the binding — with the honest limit that a signature authenticates a
	// callback without making it one-shot, since re-signing the same state
	// inside the window yields the same value every time.
	i.setDanceCookie(c, oauthStateCookie, dance.StateSignature, dance.TTL)
	i.setDanceCookie(c, oauthVerifierCookie, dance.Verifier, dance.TTL)

	// Written on EVERY start, not only an invited one, so an absent handle
	// actively cancels whatever the browser was holding.
	//
	// Writing it only when present would leave a previous dance's handle alive:
	// the state and verifier are overwritten either way, so an abandoned
	// invitation would silently attach itself to the next ordinary sign-in —
	// and with a different account, surface as email_mismatch on a login that
	// had nothing to do with any invitation. Cookies are not origin-isolated
	// either, so "only ever set it" also leaves room for a planted handle.
	if dance.InvitationHandle == "" {
		i.expireDanceCookie(c, oauthInvitationCookie)
	} else {
		i.setDanceCookie(c, oauthInvitationCookie, dance.InvitationHandle, dance.TTL)
	}

	return c.Redirect(http.StatusFound, dance.AuthorizationURL)
}
