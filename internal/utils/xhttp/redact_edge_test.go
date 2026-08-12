package xhttp

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file covers the URL and header shapes that the main table in redact_test.go
// does not reach. Each case is labeled GUARANTEE (a property the package promises
// and must keep) or DOCUMENTED LIMITATION (current behavior recorded on purpose, so
// that a future change to it is a visible decision rather than an accident).

// safeURL only ever writes Scheme, Host, Path and the masked query. Everything else
// url.URL can carry is dropped. That is the safe direction, but "dropped" is a
// property worth pinning: the fields below are the ones a well-meaning future edit
// would most plausibly add back for diagnostics, and each of them can carry a
// credential.
func TestSafeURLDropsNonRenderedComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
		why  string
	}{
		{
			// GUARANTEE. The fragment is never sent to a server by net/http, but it
			// IS carried on the *url.URL that safeURL is handed, and OAuth implicit
			// flow puts access tokens exactly there (#access_token=...). It is not
			// masked, it is omitted — which is stronger. Pinned so that "the fragment
			// is missing from the log, let me add it" becomes a red test.
			name: "fragment is dropped, not masked",
			raw:  "https://example.com/a#access_token=" + testToken,
			want: "https://example.com/[REDACTED]",
			why:  "a fragment can carry an implicit-flow access token",
		},
		{
			// DOCUMENTED LIMITATION. An opaque URL (RFC 3986 "scheme:rest", no
			// authority) keeps everything after the colon in u.Opaque, which safeURL
			// never renders. The output degrades to a bare "scheme://" — useless for
			// diagnostics but leak-free. No caller of this package builds opaque URLs
			// (all four go through http/https), so this is recorded, not fixed.
			name: "opaque URL degrades to scheme only",
			raw:  "mailto:" + testToken + "@example.com",
			want: "mailto://",
			why:  "u.Opaque is never rendered; output is useless but safe",
		},
		{
			// GUARANTEE. A scheme-relative URL still masks its path. safeURL skips the
			// "://" separator when Scheme is empty rather than emitting a stray one.
			name: "scheme-relative URL keeps host and masks path",
			raw:  "//example.com/" + testToken + "/x",
			want: "example.com/[REDACTED]/[REDACTED]",
			why:  "no scheme must not mean no masking",
		},
		{
			// GUARANTEE. A path-only URL (no scheme, no host) must not render a
			// leading "://" or an empty authority, and must still mask.
			name: "path-only URL",
			raw:  "/" + testToken + "/x?token=" + testToken,
			want: "/[REDACTED]/[REDACTED]?token=[REDACTED]",
			why:  "relative URLs must mask too",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(tt.raw)
			require.NoError(t, err)

			got := safeURL(u)
			require.Equal(t, tt.want, got, tt.why)
			require.NotContains(t, got, testToken, "no shape of URL may leak the token")
		})
	}
}

// Host is rendered verbatim, and that is deliberate — but "verbatim" has to survive
// the host shapes that are easy to get wrong when someone reimplements this by
// splitting on ':' to strip a port.
func TestSafeURLPreservesHostShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			// GUARANTEE. An IPv6 literal contains colons inside brackets. A port-strip
			// implemented with strings.Split(host, ":") mangles it; this pins the
			// bracketed form including the port.
			name: "IPv6 literal with port",
			raw:  "https://[2001:db8::1]:8443/" + testToken,
			want: "https://[2001:db8::1]:8443/[REDACTED]",
		},
		{
			// GUARANTEE. A port is diagnostically valuable and never secret, so it
			// survives.
			name: "host with port",
			raw:  "https://console.example.com:9443/cloud/v1",
			want: "https://console.example.com:9443/[REDACTED]/[REDACTED]",
		},
		{
			// GUARANTEE. Userinfo is dropped even when a port follows it. The existing
			// table covers userinfo without a port; the combination is where a naive
			// "cut everything before @" would take the port with it.
			name: "userinfo with port drops only the credential",
			raw:  "https://user:" + testToken + "@example.com:8443/x",
			want: "https://example.com:8443/[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(tt.raw)
			require.NoError(t, err)

			got := safeURL(u)
			require.Equal(t, tt.want, got)
			require.NotContains(t, got, testToken)
		})
	}
}

// Query masking keeps keys and drops values. These are the query shapes where "drop
// the value" is ambiguous, and where an implementation could plausibly diverge.
func TestSafeURLQueryShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
		why  string
	}{
		{
			// GUARANTEE. A repeated key must collapse to ONE masked entry, not one per
			// value. Emitting "a=[REDACTED]&a=[REDACTED]" would disclose how many
			// values were present, and a per-value implementation is the natural one
			// to write.
			name: "repeated key collapses to one masked pair",
			raw:  "https://example.com/x?a=" + testToken + "&a=other",
			want: "https://example.com/[REDACTED]?a=[REDACTED]",
			why:  "value COUNT is not diagnostics worth disclosing",
		},
		{
			// GUARANTEE. An empty value is still masked. Rendering "a=" would leak the
			// fact that the parameter was empty; more importantly, a branch that
			// special-cases empty values is a branch that could special-case others.
			name: "empty value is masked like any other",
			raw:  "https://example.com/x?a=",
			want: "https://example.com/[REDACTED]?a=[REDACTED]",
			why:  "no value shape gets a bypass",
		},
		{
			// GUARANTEE. A bare key with no '=' at all parses to an empty value and is
			// rendered the same way.
			name: "valueless key",
			raw:  "https://example.com/x?debug",
			want: "https://example.com/[REDACTED]?debug=[REDACTED]",
			why:  "a bare flag is still a key",
		},
		{
			// DOCUMENTED LIMITATION, and the sharpest one in this file. KEYS are
			// preserved verbatim on the stated assumption that they are
			// "near-universally non-secret" (safeURL doc comment). A secret used AS a
			// key therefore reaches the log in full. No caller does this — the four
			// production callers use fixed key names — but the assumption is an
			// assumption, and it should be visible rather than implicit.
			name: "a secret used as a query KEY is NOT redacted",
			raw:  "https://example.com/x?" + testToken + "=1",
			want: "https://example.com/[REDACTED]?" + testToken + "=[REDACTED]",
			why:  "keys are preserved by design; see the bounded-gap note below",
		},
		{
			// GUARANTEE. url.Values.Get drops pairs it cannot unescape. A query that
			// is partly malformed must lose the malformed pair rather than fall back
			// to rendering RawQuery — falling back would ship the raw secret.
			name: "malformed pair is dropped, not passed through raw",
			raw:  "https://example.com/x?bad=%zz&token=" + testToken,
			want: "https://example.com/[REDACTED]?token=[REDACTED]",
			why:  "a parse failure must never degrade into echoing RawQuery",
		},
		{
			// GUARANTEE. Go rejects ';' as a separator outright, so the whole query
			// parses to nothing and the rendering omits it. Silent loss of
			// diagnostics, but the secret does not survive — which is the direction
			// that matters.
			name: "semicolon-separated query is dropped wholesale",
			raw:  "https://example.com/x?token=" + testToken + ";other=1",
			want: "https://example.com/[REDACTED]",
			why:  "an unparseable query must vanish rather than leak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(tt.raw)
			require.NoError(t, err)

			require.Equal(t, tt.want, safeURL(u), tt.why)
		})
	}
}

// DOCUMENTED LIMITATION, KNOWN AND BOUNDED.
//
// safeURL masks the DECODED path (u.Path), so a percent-encoded segment is decoded
// before it is replaced. That is fine — the whole segment is replaced either way.
// What this test pins is the case where a segment decodes to something containing a
// slash: %2F becomes '/' in u.Path, so one wire segment renders as TWO masked
// segments. The secret is still gone; only the segment COUNT, which safeURL promises
// to preserve, is wrong.
//
// This is recorded rather than filed as a bug because the guarantee that matters
// (no secret in the output) holds, and the one that bends (segment count) is
// diagnostic sugar. If this ever becomes load-bearing, mask u.EscapedPath() instead.
func TestSafeURLEncodedSlashSplitsSegmentCount(t *testing.T) {
	t.Parallel()

	u, err := url.Parse("https://example.com/a%2Fb/c")
	require.NoError(t, err)

	got := safeURL(u)

	assert.Equal(t, "https://example.com/[REDACTED]/[REDACTED]/[REDACTED]", got,
		"an encoded slash inflates the segment count; the redaction itself is unaffected")
	assert.NotContains(t, got, "a/b", "the decoded segment content must not survive")
}

// GUARANTEE. Percent-encoded credential material is masked like anything else.
// Included because "%73%65%63%72%65%74" does not look like a secret to a reviewer
// skimming the code, and an implementation masking RawPath while reading Path (or
// vice versa) could pass the plain-text tests and fail this one.
func TestSafeURLMasksPercentEncodedSegment(t *testing.T) {
	t.Parallel()

	// "/%73%65%63%72%65%74/x" decodes to "/secret/x".
	u, err := url.Parse("https://example.com/%73%65%63%72%65%74/x")
	require.NoError(t, err)

	got := safeURL(u)

	assert.Equal(t, "https://example.com/[REDACTED]/[REDACTED]", got)
	assert.NotContains(t, got, "secret")
	assert.NotContains(t, got, "%73", "neither the encoded nor the decoded form may survive")
}

// DOCUMENTED LIMITATION, KNOWN AND BOUNDED — the accepted blocklist gap.
//
// sensitiveHeaders is a blocklist, and redact.go says so explicitly: a credential in
// a header name that is not on the list reaches the log in full. This test asserts
// that CURRENT BEHAVIOR rather than asserting safety, so that the gap is visible in
// the test output instead of living only in a comment.
//
// The gap is bounded by three things, all of which must stay true for it to remain
// acceptable:
//   - bodies are off by default, so an unlisted header is the only uncovered channel;
//   - the four production callers use Authorization exclusively, which IS listed;
//   - redact.go's contract requires any new integration carrying a credential in a
//     custom header to EXTEND sensitiveHeaders rather than rely on the above.
//
// If a future caller adds X-Api-Key and this test still passes unchanged, that
// caller broke the contract. Failing this test by adding the header to the blocklist
// is the correct way to "fix" it.
func TestUnlistedCredentialHeadersAreNotRedacted(t *testing.T) {
	t.Parallel()

	header := http.Header{}
	for _, name := range []string{
		"X-Api-Key",
		"X-Auth-Token",
		"Api-Key",
		"X-Amz-Security-Token",
		"Authentication", // note: NOT "Authorization" — one letter from being covered
	} {
		header.Set(name, testToken)
	}

	got := redactHeaders(header)

	for name, value := range got {
		assert.Equal(t, testToken, value,
			"%s is not on the blocklist, so it is logged verbatim — see this test's doc comment", name)
	}
}

// GUARANTEE. Near-misses of blocklisted names must not be matched by a substring or
// prefix check. This is the mirror of the test above: it pins that the blocklist is
// an exact-match set, so nobody "hardens" it into a strings.Contains that would
// silently redact X-Request-Authorization-Policy and destroy diagnostics.
func TestBlocklistMatchesExactNamesOnly(t *testing.T) {
	t.Parallel()

	header := http.Header{}
	header.Set("X-Authorization-Scheme", "bearer-v2")
	header.Set("Cookie-Policy", "strict")

	got := redactHeaders(header)

	assert.Equal(t, "bearer-v2", got["X-Authorization-Scheme"],
		"substring matching would over-redact and destroy diagnostics silently")
	assert.Equal(t, "strict", got["Cookie-Policy"])
}

// GUARANTEE. A header carrying an empty value list must not panic and must not
// disappear. http.Header{"Authorization": {}} is reachable through direct map
// assignment, and strings.Join over an empty slice yields "" — the danger being an
// implementation that treats "no values" as "nothing to redact" and later grows a
// v[0] index that panics.
func TestRedactHeadersHandlesEmptyValueSlices(t *testing.T) {
	t.Parallel()

	header := http.Header{
		"Authorization": {},
		"Content-Type":  {},
	}

	require.NotPanics(t, func() {
		got := redactHeaders(header)

		assert.Equal(t, redacted, got["Authorization"],
			"a listed header is redacted on name alone, regardless of its values")
		assert.Empty(t, got["Content-Type"])
	})
}

// GUARANTEE, and the deep half of the non-mutation contract.
//
// redact_test.go already asserts the caller's header survives redaction. This asserts
// the stronger property that the RETURNED map shares no backing array with the
// caller's value slices: maps.Clone would pass the existing test and fail this one,
// because its []string values still alias the original. The failure mode of getting
// this wrong is a live *http.Request losing its Authorization on a retry — a runtime
// 401, never a red test.
func TestRedactHeadersReturnsNoAliasedStorage(t *testing.T) {
	t.Parallel()

	values := []string{"application/json", "text/plain"}
	header := http.Header{"Accept": values}

	got := redactHeaders(header)
	require.Equal(t, "application/json, text/plain", got["Accept"])

	// Mutating the caller's slice after the fact must not change what was returned.
	values[0] = testToken

	assert.Equal(t, "application/json, text/plain", got["Accept"],
		"the returned copy must not alias the caller's value slice")
}
