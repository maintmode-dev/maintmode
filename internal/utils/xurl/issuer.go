// Package xurl normalizes URLs whose spelling must not change their meaning.
package xurl

import (
	"net/url"
	"strings"
)

// NormalizeIssuer reduces an OIDC issuer URL to the form two spellings of the
// same issuer share: no surrounding space, no trailing slash, lowercase scheme
// and host.
//
// It is one function because two callers must agree, and the cost of their
// disagreeing is paid in a place neither of them looks.
//
//   - The secret path seals a login provider's client_secret against its
//     issuer. The AAD is the exact bytes, so an issuer that normalizes one way
//     at seal time and another at open time produces a secret that cannot be
//     decrypted -- surfacing much later as ErrIntegrationUnreadable on a
//     sign-in nobody connects to the edit that caused it.
//   - The discovery resolver keys its provider cache and its failure cool-off
//     by issuer. Two spellings of one issuer used to be two cache entries and
//     two outbound fetches; worse, a cool-off recorded under one spelling did
//     not suppress retries under the other.
//
// The stability guard beside the secret path is the reason the rule is this
// one and not something stricter. It treats a trailing slash or a differently
// cased host as "not a change" and lets an update through without a new
// secret -- correctly, since discovery trims the slash and hosts are
// case-insensitive. Comparing raw strings instead would flag a trailing space
// as a change (noise) while a case-only edit slipped past (silence): wrong in
// both directions at once.
//
// Path, query and fragment are left ALONE, case included. An issuer's path is
// case-sensitive per RFC 3986, and a provider serving /Realms/Corp is a
// different issuer from one serving /realms/corp -- folding it would merge two
// providers into one cache entry and one AAD.
func NormalizeIssuer(raw string) string {
	// Trimmed on both sides of the slash, not once: " https://idp.example/ "
	// leaves a trailing space behind after the suffix comes off.
	//
	// TrimRight, not TrimSuffix: TrimSuffix removes ONE slash per call, so a
	// value ending in several of them normalized to a shorter string on every
	// pass. That breaks idempotence, and idempotence is the property the AAD
	// rests on -- seal normalizes once, open normalizes again, and the two must
	// produce the same bytes.
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimSpace(strings.TrimRight(trimmed, "/"))

	// An unparseable value is lowercased whole and returned. It cannot be a
	// usable issuer, and the callers refuse it on their own terms -- what
	// matters here is that the same junk normalizes to the same bytes, so a
	// secret sealed under it can still be opened.
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return strings.ToLower(trimmed)
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)

	return parsed.String()
}
