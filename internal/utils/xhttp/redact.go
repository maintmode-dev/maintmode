package xhttp

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// redacted replaces any value we refuse to write to a log.
const redacted = "[REDACTED]"

// maxLoggedBodyBytes bounds the log line, not the wire. A dump longer than this
// is truncated and marked; the request still goes out in full.
const maxLoggedBodyBytes = 4 << 10 // 4 KiB

// truncationMarker terminates a body dump cut short by maxLoggedBodyBytes, so a
// reader can tell a truncated payload from a short one.
const truncationMarker = "…[TRUNCATED]"

// sensitiveHeaders are redacted wholesale. This is a blocklist, not an allow-list,
// and that is the right trade for headers specifically: the header namespace is
// bounded and its secret-bearing names are well known, unlike paths or bodies. The
// residual risk of a miss is capped because bodies are off by default — which is
// also why any future integration adding a custom credential header (X-Api-Key and
// friends) MUST extend this list rather than rely on that cap.
//
// The map is deliberately a union of request-side and response-side names: it is
// applied to both req.Header and resp.Header. Do not "clean up" Set-Cookie from the
// request path or Authorization from the response path.
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

// redactHeaders returns a copy safe to log, looking names up through
// http.CanonicalHeaderKey: a caller that assigns req.Header["authorization"]
// directly bypasses the canonicalization Set/Add would have done, and a raw
// map-key comparison would silently miss it.
//
// It never mutates the caller's header map, and the reason is stronger than "the
// Authorization header would be stripped from the request being sent" — though
// that alone would be an outage. http.Client re-enters RoundTrip with the SAME
// *http.Request on retries and redirects, so a mutating redactor corrupts the
// header on the second attempt even in a single-goroutine flow.
//
// Note the return type: a value-flattening copy, NOT maps.Clone. maps.Clone is a
// shallow copy whose []string values still alias the caller's slices, so writing
// v[0] = redacted through it mutates the live request. That is the obvious
// implementation and it is wrong.
func redactHeaders(h http.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}

	out := make(map[string]string, len(h))
	for name, values := range h {
		canonical := http.CanonicalHeaderKey(name)
		if _, sensitive := sensitiveHeaders[canonical]; sensitive {
			out[canonical] = redacted
			continue
		}

		out[canonical] = strings.Join(values, ", ")
	}

	return out
}

// safeURL renders a URL for logging: scheme and host survive, every path segment is
// replaced by [REDACTED] keeping the segment COUNT, and every query value is
// replaced keeping the KEY. Keys and segment count are the diagnostically useful,
// near-universally non-secret half.
//
// Exact output, asserted verbatim in the tests — two implementations that differ
// here must not both pass:
//
//	https://api.telegram.org/botSECRET/sendMessage?access_token=SECRET&x=1
//	→ https://api.telegram.org/[REDACTED]/[REDACTED]?access_token=[REDACTED]&x=[REDACTED]
//
// There is deliberately no "does this path segment look like a credential"
// heuristic. Of the four callers exactly one puts a secret in the path (telegram:
// /bot<token>/sendMessage); the other three carry plain routes. A heuristic would
// have to be loose enough to catch bot<digits>:<base64> and tight enough to spare
// /cloud/v1/instances/heartbeat, and BOTH failure directions are silent — a miss
// leaks a token with no signal, an over-match destroys the path with no signal.
//
// A nil URL yields the empty string rather than panicking: logging is a side
// effect, and it must never be the reason a request dies.
func safeURL(u *url.URL) string {
	if u == nil {
		return ""
	}

	// Assembled by hand rather than through url.URL.String(): the marker contains
	// [ and ], which String() percent-escapes into %5BREDACTED%5D. Every part here
	// is already either structural or redacted, so there is nothing left to encode
	// — and a log line is for a human to read.
	var b strings.Builder

	if u.Scheme != "" {
		b.WriteString(u.Scheme)
		b.WriteString("://")
	}
	// Userinfo is deliberately dropped: "user:pass@" is a credential by definition.
	b.WriteString(u.Host)
	b.WriteString(maskPath(u.Path))

	if query := maskQuery(u.Query()); query != "" {
		b.WriteString("?")
		b.WriteString(query)
	}

	return b.String()
}

// maskPath replaces every non-empty segment with the redaction marker, preserving
// the number of segments and the leading/trailing slashes around them.
func maskPath(path string) string {
	if path == "" {
		return ""
	}

	segments := strings.Split(path, "/")
	for i, segment := range segments {
		// Empty segments come from the leading slash and from any trailing or
		// doubled one. Rewriting them would invent structure that is not there.
		if segment == "" {
			continue
		}

		segments[i] = redacted
	}

	return strings.Join(segments, "/")
}

// maskQuery replaces every value while keeping the keys, which is the half that
// carries the diagnostics and effectively never carries the secret. Keys are sorted
// so the rendering is stable across calls — map iteration order is not.
func maskQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	pairs := make([]string, 0, len(values))
	for key := range values {
		pairs = append(pairs, key+"="+redacted)
	}
	slices.Sort(pairs)

	return strings.Join(pairs, "&")
}
