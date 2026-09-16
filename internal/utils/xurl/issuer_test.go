package xurl_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xurl"
)

// Spellings that mean one issuer must normalize to one string.
//
// This is the property the secret path depends on: the AAD is these exact
// bytes, so two spellings landing on two results means a client_secret sealed
// under one and reopened under the other -- an unreadable provider, surfacing
// at a sign-in long after the edit.
func TestNormalizeIssuer_FoldsEquivalentSpellings(t *testing.T) {
	t.Parallel()

	const want = "https://idp.example"

	for name, raw := range map[string]string{
		"as written":        "https://idp.example",
		"trailing slash":    "https://idp.example/",
		"surrounding space": " https://idp.example ",
		"slash then space":  " https://idp.example/ ",
		"uppercase host":    "https://IdP.Example",
		"uppercase scheme":  "HTTPS://idp.example",
		"both cased":        "HTTPS://IDP.EXAMPLE/",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, want, xurl.NormalizeIssuer(raw))
		})
	}
}

// What must NOT be folded. Each of these is a different issuer, and merging two
// of them would mean one cache entry and one AAD for two providers.
func TestNormalizeIssuer_KeepsDistinctIssuersApart(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ a, b string }{
		// RFC 3986 makes the path case-sensitive, and a Keycloak realm is a
		// path segment: /realms/corp and /realms/Corp are two realms.
		"path case":   {"https://idp.example/realms/corp", "https://idp.example/realms/Corp"},
		"path at all": {"https://idp.example", "https://idp.example/realms/corp"},
		"host":        {"https://idp.example", "https://idp.other.example"},
		"scheme":      {"https://idp.example", "http://idp.example"},
		"port":        {"https://idp.example", "https://idp.example:8443"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.NotEqual(t, xurl.NormalizeIssuer(tc.a), xurl.NormalizeIssuer(tc.b),
				"%q and %q are different issuers", tc.a, tc.b)
		})
	}
}

// Normalizing twice must change nothing. The secret path normalizes at seal
// time and again at open time; if a second pass moved the value, every secret
// would be sealed under bytes the reopen could not reproduce.
func TestNormalizeIssuer_IsIdempotent(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"https://idp.example/", " HTTPS://IdP.Example/ ", "https://idp.example/realms/corp",
		"not a url at all", "", "///",
	} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			once := xurl.NormalizeIssuer(raw)
			require.Equal(t, once, xurl.NormalizeIssuer(once),
				"a second pass over %q must be a no-op", raw)
		})
	}
}

// Input that cannot be an issuer still has to normalize deterministically: a
// secret sealed under junk must remain openable under the same junk.
func TestNormalizeIssuer_HandlesUnparseableInput(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", xurl.NormalizeIssuer(""))
	require.Equal(t, "", xurl.NormalizeIssuer("   "))
	require.Equal(t, xurl.NormalizeIssuer(" NOT A URL "), xurl.NormalizeIssuer("not a url"))
}
