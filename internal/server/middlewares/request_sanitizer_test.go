package middlewares_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/server/middlewares"
)

// TestSanitizeURLMasksDanceCredentials covers the leak RUK-291 introduces: an
// OAuth callback arrives with a live authorization code in its query string, and
// the request logger writes the URI verbatim.
func TestSanitizeURLMasksDanceCredentials(t *testing.T) {
	t.Parallel()

	s := middlewares.NewRequestSanitizer()

	tests := map[string]struct {
		in          string
		mustNotHave []string
		mustHave    []string
	}{
		// One case per parameter, deliberately. An earlier version asserted the
		// code and the state together, and a mutation removing ONLY "code" from
		// the blocklist still passed, because the state was masked and the
		// combined assertion could not tell which secret had survived.
		"oauth callback code": {
			in:          "/auth/api/v1/login/oauth/google/callback?code=4/0AY0e-live&state=abc123",
			mustNotHave: []string{"4/0AY0e-live"},
			// The route must stay readable: masking it wholesale is what the
			// shared sanitizer already rolled back once.
			mustHave: []string{"/auth/api/v1/login/oauth/google/callback", "code="},
		},
		"oauth callback state": {
			in:          "/auth/api/v1/login/oauth/google/callback?code=4/0AY0e-live&state=abc123",
			mustNotHave: []string{"abc123"},
			mustHave:    []string{"state="},
		},
		"pkce verifier": {
			in:          "/callback?code_verifier=super-secret-verifier",
			mustNotHave: []string{"super-secret-verifier"},
		},
		"id token": {
			in:          "/x?id_token=eyJhbGciOi.payload.sig",
			mustNotHave: []string{"eyJhbGciOi.payload.sig"},
		},
		// The invitation token is a 7-day bearer credential: GetByTokenHash is
		// the only thing authenticating an accept, so a log line holding it is
		// a week-long standing grant. It reaches a query string on
		// /login/oauth/{provider}/start?invitation=<raw>.
		"invitation token on the dance start": {
			in:          "/auth/api/v1/login/oauth/google/start?invitation=inv-live-token",
			mustNotHave: []string{"inv-live-token"},
			mustHave:    []string{"/auth/api/v1/login/oauth/google/start", "invitation="},
		},
		// Same credential, older surface: the public preview takes the raw
		// token as ?token=. This exposure predates the dance and is fixed here
		// because masking one twin and leaving the other is indefensible.
		"invitation token on the public preview": {
			in:          "/auth/api/v1/users/invitations/preview?token=inv-live-token",
			mustNotHave: []string{"inv-live-token"},
			mustHave:    []string{"/auth/api/v1/users/invitations/preview", "token="},
		},
		"benign parameters survive untouched": {
			in:       "/api/v1/users?limit=50&search=alice",
			mustHave: []string{"limit=50", "search=alice"},
		},
		"error redirects stay diagnosable": {
			in:       "/auth/oauth/callback?error=state_reused",
			mustHave: []string{"error=state_reused"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := s.SanitizeURL(tt.in)

			// Compare against the DECODED form, not the raw string. Encode()
			// percent-escapes the output, so a leaked Google code appears as
			// "4%2F0AY0e-live" rather than "4/0AY0e-live" — and a NotContains
			// on the literal passes while the secret sits in the log. A
			// mutation removing "code" from the blocklist caught this test
			// doing exactly that.
			decoded, err := url.QueryUnescape(got)
			require.NoError(t, err)

			for _, secret := range tt.mustNotHave {
				assert.NotContains(t, decoded, secret, "a credential reached the log")
			}
			for _, keep := range tt.mustHave {
				assert.Contains(t, got, keep, "diagnostics were masked away")
			}
		})
	}
}

// TestSanitizeBodyMasksTokenPairs is the third leak site. Body logging is
// dev-only, but the test stand declares environment: dev, so without this the
// dance's own tests write whole token pairs into the log.
func TestSanitizeBodyMasksTokenPairs(t *testing.T) {
	t.Parallel()

	s := middlewares.NewRequestSanitizer()

	t.Run("a token pair response is masked", func(t *testing.T) {
		t.Parallel()

		got := string(s.SanitizeBody([]byte(`{"access_token":"live-access","refresh_token":"live-refresh","expires_in":900}`)))

		assert.NotContains(t, got, "live-access")
		assert.NotContains(t, got, "live-refresh")
		// A non-secret field survives, so the record still says what it was.
		assert.Contains(t, got, "900")
	})

	t.Run("a one-time code request is masked", func(t *testing.T) {
		t.Parallel()

		got := string(s.SanitizeBody([]byte(`{"code":"one-time-opaque"}`)))
		assert.NotContains(t, got, "one-time-opaque")
	})

	// One subtest per field. Asserting several together lets a mutation that
	// drops one entry pass, because the others are still masked — the same shape
	// of hole the URL test had. password matters most of the three: the
	// break-glass sign-in posts it, and body logging is on in dev.
	t.Run("every blocked field is masked independently", func(t *testing.T) {
		t.Parallel()

		fields := map[string]string{
			"access_token":  `{"access_token":"SECRET-VALUE"}`,
			"refresh_token": `{"refresh_token":"SECRET-VALUE"}`,
			"id_token":      `{"id_token":"SECRET-VALUE"}`,
			"code":          `{"code":"SECRET-VALUE"}`,
			"password":      `{"password":"SECRET-VALUE"}`,
			"session_nonce": `{"session_nonce":"SECRET-VALUE"}`,
		}

		for field, body := range fields {
			t.Run(field, func(t *testing.T) {
				t.Parallel()

				assert.NotContains(t, string(s.SanitizeBody([]byte(body))), "SECRET-VALUE",
					"%s reached the log", field)
			})
		}
	})

	t.Run("a non-JSON body is left alone", func(t *testing.T) {
		t.Parallel()

		const body = "plain text, not a credential"
		assert.Equal(t, body, string(s.SanitizeBody([]byte(body))))
	})

	t.Run("a body with nothing sensitive is untouched", func(t *testing.T) {
		t.Parallel()

		const body = `{"name":"alice","limit":10}`
		assert.Equal(t, body, string(s.SanitizeBody([]byte(body))))
	})
}

// TestSanitizerKeepsTheSharedHeaderPolicy proves the embedding is real: adding a
// sensitive header name to xsanitize must reach this type, which a copied
// blocklist would not do.
func TestSanitizerKeepsTheSharedHeaderPolicy(t *testing.T) {
	t.Parallel()

	headers := middlewares.NewRequestSanitizer().SanitizeHeaders(map[string][]string{
		"Authorization": {"Bearer live-token"},
		"Cookie":        {"oauth_dance_nonce=live-nonce"},
		"Accept":        {"application/json"},
	})

	require.NotNil(t, headers)
	assert.NotContains(t, headers["Authorization"], "Bearer live-token")
	assert.NotContains(t, headers["Cookie"], "oauth_dance_nonce=live-nonce")
	assert.Equal(t, []string{"application/json"}, headers["Accept"])
}

// TestSanitizeURLFailsSafeOnAnUnparseableQuery covers the branch where Go's own
// parser gives up. A bare semicolon has been a parse error since Go 1.17, and
// url.Query() answers by returning what it managed — so a naive implementation
// either logs the original URI unredacted or drops parameters without saying so.
func TestSanitizeURLFailsSafeOnAnUnparseableQuery(t *testing.T) {
	t.Parallel()

	s := middlewares.NewRequestSanitizer()

	got := s.SanitizeURL("/auth/api/v1/login/oauth/google/callback?code=LIVE-CODE;b=c")

	assert.NotContains(t, got, "LIVE-CODE", "an unparseable query must never leak its values")
	// The route survives: knowing WHICH endpoint was hit is the diagnostic this
	// sanitizer exists to preserve.
	assert.Contains(t, got, "/auth/api/v1/login/oauth/google/callback")
}

// TestSanitizeURLRedactsWhateverGoRefusesToParse covers the first of SanitizeURL's
// two fail-safe branches, and records something I got wrong while writing it.
//
// The branches are NOT independent, and a mutation test proves it: removing the
// url.Parse guard entirely leaves every assertion here green, because whatever
// slips past it fails at ParseQuery a few lines later and is redacted there
// instead. So this test pins the OUTCOME — no secret in the output — rather than
// which guard produced it, since no input distinguishes them from the outside.
//
// Worth knowing which input hits which, because it is easy to assume wrongly: a
// broken percent-escape like %zz parses fine as a URL and fails at ParseQuery,
// while a raw control character fails at url.Parse. Both are reachable from the
// wire, and a mistyped callback URL carrying a live code lands on the
// RouteNotFound logger — which is why AC6 says no authorization code appears in
// ANY log line.
func TestSanitizeURLRedactsWhateverGoRefusesToParse(t *testing.T) {
	t.Parallel()

	s := middlewares.NewRequestSanitizer()

	for name, uri := range map[string]string{
		// Fails at ParseQuery; url.Parse accepts it.
		"broken percent escape": "/login/oauth/google/callback?code=LIVE-CODE&x=%zz",
		// Fails at url.Parse itself.
		"control character": "/login/oauth/google/callback?code=LIVE-CODE&x=" + string(rune(0x7f)),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.NotContains(t, s.SanitizeURL(uri), "LIVE-CODE",
				"a URI Go will not parse must never reach the log carrying its values")
		})
	}
}

// TestSanitizeBodyRecursesIntoNestedShapes guards a property that is currently
// invisible: today's bodies are flat, so a top-level-only pass would leak
// nothing and every existing test would still pass. A wrapped response is
// exactly what someone adds later without thinking about the logger.
func TestSanitizeBodyRecursesIntoNestedShapes(t *testing.T) {
	t.Parallel()

	s := middlewares.NewRequestSanitizer()

	tests := map[string]string{
		"nested object":         `{"data":{"access_token":"SECRET-VALUE"}}`,
		"array of objects":      `[{"code":"SECRET-VALUE"}]`,
		"deeply nested":         `{"a":{"b":{"c":{"refresh_token":"SECRET-VALUE"}}}}`,
		"array inside object":   `{"items":[{"password":"SECRET-VALUE"}]}`,
		"sibling of a safe key": `{"name":"alice","session":{"id_token":"SECRET-VALUE"}}`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.NotContains(t, string(s.SanitizeBody([]byte(body))), "SECRET-VALUE")
		})
	}
}
