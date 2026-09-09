package auth

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
)

// The cookies that carry a dance. The first two are the dance itself and are
// stored nowhere; the third names an invitation and is the one exception.
//
// The split is the binding: the provider gets the plaintext state and the
// browser only its SIGNATURE, so whoever observes the redirect URL holds one
// half and not the other. The verifier never travels at all — only its S256
// challenge does.
const (
	oauthStateCookie    = "oauth_state"
	oauthVerifierCookie = "oauth_code_verifier"
	// oauthInvitationCookie carries the handle for an invited dance. The
	// invitation token itself never travels: the state goes to the provider in
	// the clear, and the token is a bearer credential with a multi-day life, so
	// only an opaque handle leaves this backend and only in a cookie the
	// provider never sees.
	oauthInvitationCookie = "oauth_invitation"
)

// danceCookieValue reads one dance cookie, treating "absent" and "empty" as the
// same thing: neither can complete a dance, and no caller has a use for the
// difference.
func danceCookieValue(c *echo.Context, name string) string {
	cookie, err := c.Cookie(name)
	if err != nil {
		return ""
	}

	return cookie.Value
}

// setDanceCookie writes one of the dance cookies. The lifetime comes from the
// service alongside the values themselves — MaxAge is only a hint to the
// browser, since the deadline that is enforced sits inside the signature.
func (i *Implementation) setDanceCookie(c *echo.Context, name, value string, ttl time.Duration) {
	http.SetCookie(c.Response(), i.danceCookie(name, value, int(ttl.Seconds())))
}

// expireDanceCookies queues the removal of every dance cookie.
//
// The callback calls this FIRST, before any check, which is what makes "cleared
// on every exit" true by construction rather than by remembering it in seven
// branches.
//
// What it buys is worth stating precisely, because the obvious reading
// overstates it: a COOPERATING browser cannot carry a spent dance into the next
// attempt. An attacker replaying a captured pair with curl never honors
// Set-Cookie at all, so this bounds nothing for them — the signature's deadline
// and the provider burning the authorization code are what cover that case.
func (i *Implementation) expireDanceCookies(c *echo.Context) {
	for _, name := range []string{oauthStateCookie, oauthVerifierCookie, oauthInvitationCookie} {
		i.expireDanceCookie(c, name)
	}
}

// expireDanceCookie cancels one dance cookie.
//
// Split out because /start needs it for a single cookie: an uninvited dance
// must actively clear any invitation handle the browser still holds, since
// leaving it untouched is what lets an abandoned invitation attach itself to
// the next ordinary sign-in.
func (i *Implementation) expireDanceCookie(c *echo.Context, name string) {
	http.SetCookie(c.Response(), i.danceCookie(name, "", -1))
}

// danceCookie builds a dance cookie. Expiry reuses it rather than hand-rolling
// a second cookie, because a browser only replaces one whose name, Path and
// Domain all match — an expiry with a different Path leaves the original alive.
//
// Path is configured outright (app.oauth_cookie_path) and must be the EXTERNAL
// prefix: Caddy strips /auth before the handler sees the request, so a cookie
// scoped to the internal route is never sent back and no handler test would
// notice.
//
// Secure is NOT configured alongside it. It follows the redirect_uri's SCHEME,
// because the two failure directions are not symmetric: a missing-or-wrong Path
// breaks sign-in visibly enough to chase, while a Secure that is false when it
// should be true leaks the signature and the verifier and looks like nothing at
// all. Deriving it removes the chance to get it wrong. It is emphatically not
// keyed on the environment NAME: IsDev() covers dev, local and
// performance_test, and the deployed dev stand runs environment: dev behind
// Caddy on https — so an environment-keyed flag shipped both halves of a dance
// without Secure on a live HTTPS stand, with no HSTS to fall back on.
//
//nolint:gosec // G124: Secure is conditional by design, see the note above.
func (i *Implementation) danceCookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     i.danceCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   i.danceCookieSecure,
		// Lax, not Strict: the callback is a top-level navigation from the
		// provider, and Strict withholds cookies on exactly that.
		SameSite: http.SameSiteLaxMode,
	}
}
