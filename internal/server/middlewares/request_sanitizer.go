package middlewares

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/ruko1202/xhttp/sanitize"

	"github.com/ruko1202/maintmode/internal/utils/xsanitize"
)

// redacted replaces any value we refuse to write to a log.
const redacted = "[REDACTED]"

// sensitiveQueryParams are masked in logged request URIs.
//
// This is a blocklist rather than an allow-list because the query namespace of
// THIS service is bounded and known: these four names are the only ones that
// ever carry a credential, and masking every parameter would cost the
// diagnostics that make a 404 on the wrong route distinguishable from a 500 on
// the right one — the exact trade the shared sanitizer's doc comment records
// having already been made once.
var sensitiveQueryParams = map[string]struct{}{
	// The provider's authorization code. Live until redeemed, and redeemable by
	// whoever holds it plus our client secret.
	"code": {},
	// The dance's CSRF state. Half of the pair that completes a callback: the
	// other half is the signature in the browser's cookie, and a log line
	// holding the state narrows an attacker's problem to stealing one cookie.
	"state": {},
	// A provider id_token, if one ever reaches a query string.
	"id_token": {},
	// The PKCE verifier. It should never appear in a URL at all; masking it
	// costs nothing and covers a debug redirect that puts it there.
	"code_verifier": {},
}

// sensitiveBodyFields are masked in logged request and response bodies.
var sensitiveBodyFields = map[string]struct{}{
	"access_token":  {},
	"refresh_token": {},
	"id_token":      {},
	"code":          {},
	"password":      {},
	"session_nonce": {},
}

var _ sanitize.Sanitizer = RequestSanitizer{}

// RequestSanitizer is the redaction policy for INBOUND request logging.
//
// It is a wrapper rather than a change to xsanitize, and that follows this
// repository's own documented decision: xsanitize deliberately leaves path and
// query readable, having once masked them and rolled that back as "paying a real
// diagnostic price", and its comment prescribes wrapping for a caller whose
// secret does live in the URL — as the Telegram gateway already does. That
// instance is handed to every outbound client, so widening it to fix an inbound
// problem would regress diagnostics for license, Slack and JWKS.
//
// RUK-291 is the first caller whose inbound URLs carry credentials: an OAuth
// callback arrives as /callback?code=<live authorization code>&state=<...>.
//
// Embedding xsanitize.Sanitizer keeps the header policy shared, so a sensitive
// header added there reaches this type too.
type RequestSanitizer struct {
	xsanitize.Sanitizer
}

// NewRequestSanitizer returns the sanitizer for the request-logging middleware.
func NewRequestSanitizer() RequestSanitizer {
	return RequestSanitizer{}
}

// SanitizeURL masks credential-bearing query parameters, leaving the path and
// every other parameter readable.
//
// The embedded implementation is called first so userinfo stripping keeps
// applying; only the query is rewritten here.
func (s RequestSanitizer) SanitizeURL(rawURL string) string {
	base := s.Sanitizer.SanitizeURL(rawURL)
	if base == redacted || !strings.Contains(base, "?") {
		return base
	}

	u, err := url.Parse(base)
	if err != nil {
		// Logging is a side effect and must never be the reason a secret
		// escapes, so an unparseable URL yields the marker rather than itself.
		return redacted
	}

	// url.Query() drops everything after a parse error (a bare semicolon, since
	// Go 1.17). Falling through to `base` would then log the original URI
	// UNREDACTED, and silently returning the partial query would drop
	// parameters from the log without saying so. Neither is acceptable for a
	// value that may carry a live authorization code, so an unparseable query
	// redacts wholesale.
	if _, err := url.ParseQuery(u.RawQuery); err != nil {
		u.RawQuery = redacted

		return u.String()
	}

	q := u.Query()
	masked := false

	for name := range q {
		if _, sensitive := sensitiveQueryParams[strings.ToLower(name)]; sensitive {
			q.Set(name, redacted)
			masked = true
		}
	}

	if !masked {
		return base
	}

	u.RawQuery = q.Encode()

	return u.String()
}

// SanitizeBody masks credential fields in a logged body.
//
// Body logging is dev-only, but the test stand declares environment: dev, so
// without this the dance's own tests would write whole token pairs — access and
// refresh — into the log at Debug.
//
// A body that is not a JSON object is returned unchanged: it cannot be
// inspected field by field, and a blanket redaction would delete the diagnostic
// value that turning body logging on was for.
func (s RequestSanitizer) SanitizeBody(b []byte) []byte {
	if len(b) == 0 {
		return b
	}

	var decoded any
	if err := json.Unmarshal(b, &decoded); err != nil {
		return b
	}

	// Recursive, not top-level only. Today's bodies are flat, so a shallow pass
	// would leak nothing — but "flat" is a property of the current endpoints
	// rather than of this function, and a wrapped response ({"data":{...}}) is
	// exactly the shape someone adds later without thinking about the logger.
	masked := maskSensitive(decoded)
	if !masked {
		return b
	}

	out, err := json.Marshal(decoded)
	if err != nil {
		// Re-encoding failed, so the safe direction is to drop the body rather
		// than fall back to the unmasked original.
		return []byte(redacted)
	}

	return out
}

// maskSensitive walks a decoded JSON value in place, replacing the values of
// blocked keys wherever they appear. It reports whether anything was masked, so
// an untouched body can be returned verbatim rather than re-encoded — a
// re-encode reorders keys and reformats numbers, which costs log readability for
// nothing.
func maskSensitive(node any) bool {
	switch typed := node.(type) {
	case map[string]any:
		masked := false

		for name, value := range typed {
			if _, sensitive := sensitiveBodyFields[strings.ToLower(name)]; sensitive {
				typed[name] = redacted
				masked = true

				continue
			}

			if maskSensitive(value) {
				masked = true
			}
		}

		return masked
	case []any:
		masked := false

		for _, value := range typed {
			if maskSensitive(value) {
				masked = true
			}
		}

		return masked
	default:
		return false
	}
}
