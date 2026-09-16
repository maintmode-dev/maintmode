package xvalidation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xvalidation"
)

// A URL carrying userinfo is refused, and the reason is storage rather than
// syntax.
//
// This rule validates the fields an integration row is built from, and a row's
// config is stored in CLEARTEXT -- only its secrets column is encrypted. So a
// "https://user:pass@idp.example" issuer would put a credential in a plaintext
// column, hand it to every reader of the admin API, and write it into the AAD
// of the provider's client_secret, where changing it later strands the secret.
//
// Nothing legitimate needs it: a discovery document is public, and the client
// authenticates with client_id and client_secret. So the only thing userinfo
// can be here is a mistake worth refusing at the edge.
func TestHTTPSURL_RefusesUserinfo(t *testing.T) {
	t.Parallel()

	for name, raw := range map[string]string{
		"user and password": "https://user:pass@idp.example/",
		"user alone":        "https://user@idp.example/",
		"empty password":    "https://user:@idp.example/",
		// A password with no user is still a credential, and it is the form
		// most likely to be a copy-paste accident rather than a deliberate URL.
		"password alone": "https://:pass@idp.example/",
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()

			err := xvalidation.HTTPSURL(raw)
			require.Error(t, err, "%q carries a credential into a cleartext column", raw)
			require.NotContains(t, err.Error(), "pass",
				"the message must not repeat the credential it is refusing")
		})
	}
}

// The rest of the rule, so the userinfo check cannot be added by breaking
// something else. Each case names the property it pins.
func TestHTTPSURL_Verdicts(t *testing.T) {
	t.Parallel()

	t.Run("allows a public https issuer", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, xvalidation.HTTPSURL("https://accounts.google.com"))
	})

	t.Run("allows an empty value", func(t *testing.T) {
		t.Parallel()

		// validation.Required reports an empty field; this rule judges form, and
		// reporting the same omission twice gives two errors for one mistake.
		require.NoError(t, xvalidation.HTTPSURL(""))
	})

	t.Run("refuses plain http", func(t *testing.T) {
		t.Parallel()

		require.Error(t, xvalidation.HTTPSURL("http://idp.example/"))
	})

	t.Run("refuses a relative url", func(t *testing.T) {
		t.Parallel()

		require.Error(t, xvalidation.HTTPSURL("/auth/callback"))
	})

	t.Run("refuses an internal address", func(t *testing.T) {
		t.Parallel()

		// The SSRF half of the rule; internal_host_test.go has the exhaustive table.
		require.Error(t, xvalidation.HTTPSURL("https://169.254.169.254/latest/meta-data"))
	})

	t.Run("refuses a non-string", func(t *testing.T) {
		t.Parallel()

		require.Error(t, xvalidation.HTTPSURL(42))
	})
}
