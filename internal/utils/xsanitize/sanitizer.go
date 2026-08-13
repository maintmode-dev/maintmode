// Package xsanitize implements the redaction policy applied to outgoing HTTP
// logs. It is MaintMode's answer to xhttp/client.Sanitizer, whose default
// redacts nothing: the library owns the seam, this package owns what counts as
// a secret here.
//
// Pass it to every client construction — the safe default lives at the call
// site now, not in the transport:
//
//	client.NewClient(client.WithSanitizer(xsanitize.New()))
package xsanitize

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/ruko1202/xhttp/client"
)

// redacted replaces any value we refuse to write to a log.
const redacted = "[REDACTED]"

var _ client.Sanitizer = Sanitizer{}

// sensitiveHeaders are redacted wholesale. This is a blocklist, not an
// allow-list, and that is the right trade for headers specifically: the header
// namespace is bounded and its secret-bearing names are well known, unlike
// paths or bodies. The residual risk of a miss is capped because bodies are off
// by default — which is also why any future integration adding a custom
// credential header (X-Api-Key and friends) MUST extend this list rather than
// rely on that cap.
//
// The map is deliberately a union of request-side and response-side names: it
// is applied to both req.Header and resp.Header. Do not "clean up" Set-Cookie
// from the request path or Authorization from the response path.
//
// Keys are stored canonicalized, because that is how they are looked up.
var sensitiveHeaders = map[string]struct{}{
	http.CanonicalHeaderKey("Authorization"):       {},
	http.CanonicalHeaderKey("Proxy-Authorization"): {},
	http.CanonicalHeaderKey("Cookie"):              {},
	http.CanonicalHeaderKey("Set-Cookie"):          {},
	http.CanonicalHeaderKey("WWW-Authenticate"):    {}, // challenge material, response side
	http.CanonicalHeaderKey("Proxy-Authenticate"):  {},
}

// Sanitizer masks credentials in HTTP logs. The zero value is usable.
type Sanitizer struct{}

// New returns the sanitizer to hand to client.WithSanitizer.
func New() Sanitizer {
	return Sanitizer{}
}

// SanitizeHeaders returns a copy safe to log, looking names up through
// http.CanonicalHeaderKey: a caller that assigns req.Header["authorization"]
// directly bypasses the canonicalization Set/Add would have done, and a raw
// map-key comparison would silently miss it.
//
// It never mutates the caller's header map, and the reason is stronger than
// "the Authorization header would be stripped from the request being sent" —
// though that alone would be an outage. http.Client re-enters RoundTrip with
// the SAME *http.Request on retries and redirects, so a mutating redactor
// corrupts the header on the second attempt even in a single-goroutine flow.
//
// Note how the copy is built: value by value, NOT maps.Clone. maps.Clone is a
// shallow copy whose []string values still alias the caller's slices, so
// writing v[0] = redacted through it mutates the live request. That is the
// obvious implementation and it is wrong.
func (Sanitizer) SanitizeHeaders(h http.Header) http.Header {
	if len(h) == 0 {
		return nil
	}

	out := make(http.Header, len(h))
	for name, values := range h {
		canonical := http.CanonicalHeaderKey(name)
		if _, sensitive := sensitiveHeaders[canonical]; sensitive {
			out[canonical] = []string{redacted}
			continue
		}

		out[canonical] = slices.Clone(values)
	}

	return out
}

// SanitizeURL drops userinfo and leaves the rest of the URL readable.
//
//	https://user:pass@api.example.org/cloud/v1/heartbeat?page=2
//	→ https://api.example.org/cloud/v1/heartbeat?page=2
//
// Redaction here is limited to what is a credential by definition. Userinfo
// qualifies — "user:pass@" is nothing else. Path and query do not: none of the
// callers using this sanitizer put a secret in either (license hits a constant
// route, Slack hits /api/<method>, a JWKS endpoint is public, and none of them
// build a query string at all), while masking them costs exactly the detail
// that separates a 404 on the wrong route from a 500 on the right one.
//
// The earlier version masked every path segment and every query value, which
// rendered /cloud/v1/instances/heartbeat as four identical markers. That is
// paying a real diagnostic price against a threat this caller set does not
// have.
//
// A caller whose secret DOES live in the URL wraps this type and overrides this
// method — see the Telegram gateway, where the bot token sits in the path. That
// is why the seam belongs to the service and not to the transport: the rule is
// per-integration, and a heuristic guessing "does this segment look like a
// credential" fails silently in both directions.
//
// An unparseable URL yields the marker rather than the input: logging is a side
// effect, and it must never be the reason a secret escapes.
func (Sanitizer) SanitizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return redacted
	}

	if u.User == nil {
		return rawURL
	}

	stripped := *u
	stripped.User = nil

	return stripped.String()
}

// SanitizeBody leaves bodies alone. Their shape is arbitrary, so a secret
// inside one is unrecognizable and a blocklist cannot help; body logging is off
// unless explicitly enabled, and enabling it is the decision that carries the
// risk.
func (Sanitizer) SanitizeBody(b []byte) []byte {
	return b
}
