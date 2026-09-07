package auth

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/apperr"
)

// Query parameters the dance exchanges with the provider and the frontend.
const (
	paramCode  = "code"
	paramState = "state"
	paramError = "error"
)

// Redirect codes the frontend renders: a closed, stable set that RUK-292 maps
// to messages, so adding one is a cross-ticket contract change.
//
// Distinct failures collapse onto one code deliberately — the browser learns
// only that the dance did not complete, and which cause stays in the audit
// trail. errCodeStateInvalid in particular covers every way a callback can fail
// to present a state this backend signed.
const (
	errCodeAccessDenied = "access_denied"
	errCodeStateInvalid = "state_invalid"
	errCodeProvider     = "provider_error"
	errCodeInternal     = "internal_error"
)

// danceFailureCode maps a failed dance to the code the browser is sent home
// with: the service reports what went wrong, this decides what the frontend is
// told.
//
// A refused account is the MOST LIKELY failure in production, where signup is
// invite-only, and reads as a denial rather than an internal fault — nothing is
// broken, the person needs an invitation.
//
// A free function so the mapping can be tested without the whole auth service:
// the local stand runs open signup and never produces that error, so a mutation
// collapsing this to internal_error otherwise passes every handler test.
func danceFailureCode(err error) string {
	switch {
	case errors.Is(err, apperr.ErrOAuthProviderDenied):
		return providerErrorCode(err)
	case errors.Is(err, apperr.ErrSignupDisabled), errors.Is(err, apperr.ErrUserBlocked):
		return errCodeAccessDenied
	case errors.Is(err, apperr.ErrOAuthDanceStateInvalid), errors.Is(err, apperr.ErrUnsupportedProvider):
		return errCodeStateInvalid
	case errors.Is(err, apperr.ErrOAuthExchangeFailed):
		return errCodeProvider
	default:
		return errCodeInternal
	}
}

// providerErrorCode separates a user declining consent from a provider
// misbehaving: "I changed my mind" and "the provider is broken" are different
// things to whoever reads the trail, and only the first is a denial.
func providerErrorCode(err error) string {
	if strings.Contains(err.Error(), errCodeAccessDenied) {
		return errCodeAccessDenied
	}

	return errCodeProvider
}

// redirectFailure sends the browser home with a readable code. Never JSON: the
// user is mid-navigation, and a JSON body would strand them on a blank page.
func (i *Implementation) redirectFailure(c *echo.Context, code string) error {
	return i.redirectHome(c, url.Values{paramError: {code}})
}

// redirectHome is the one place this handler emits a 302, so the cache
// directive cannot be forgotten on the branch that matters: the success
// redirect carries a live one-time code, and a 302 with no directives is
// heuristically cacheable by a shared proxy (RFC 9111 §4.2.2).
func (i *Implementation) redirectHome(c *echo.Context, q url.Values) error {
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")

	return c.Redirect(http.StatusFound, i.frontendURLWith(q))
}

// frontendURLWith builds the redirect target from CONFIGURED values only. There
// is no return_to parameter, in this ticket or as a hook for a later one: a
// redirect target taken from the request is an open redirect.
//
// Built with net/url rather than by concatenation: JoinPath settles the slash
// between origin and path — the earlier version trimmed one by hand — and
// String() escapes what needs escaping instead of trusting the inputs to be
// clean.
func (i *Implementation) frontendURLWith(q url.Values) string {
	target, err := url.Parse(i.frontendURL)
	if err != nil {
		// Unreachable in practice: the config gate refuses to register these
		// routes without a frontend_url, and a value that will not parse would
		// have failed at wiring. Falling back to the raw string keeps a
		// misconfigured stand pointing somewhere visible rather than at "".
		return i.frontendURL
	}

	target = target.JoinPath(i.frontendCallbackPath)
	target.RawQuery = q.Encode()

	return target.String()
}
