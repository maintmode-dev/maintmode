package integrationkinds_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

func TestCustom_ParseValidateSecretKeys(t *testing.T) {
	t.Parallel()

	in := integrationkinds.Custom
	require.Equal(t, "custom", in.Name())
	require.Equal(t, []string{"client_secret"}, in.SecretKeys())

	const config = `{
		"display_name":"Corporate SSO",
		"issuer_url":"https://idp.corp.example",
		"client_id":"client-abc",
		"redirect_uri":"https://app.example/auth/callback",
		"scopes":["openid","email"],
		"jwtverifier":{"allowed_hosted_domains":["corp.example"]}
	}`

	t.Run("parses config and merges the secret", func(t *testing.T) {
		t.Parallel()

		parsed, err := in.Parse(json.RawMessage(config), map[string]string{"client_secret": "s3cret"})
		require.NoError(t, err)

		s, ok := parsed.(integrationkinds.OIDCSettings)
		require.True(t, ok)
		require.Equal(t, "https://idp.corp.example", s.IssuerURL)
		require.Equal(t, "client-abc", s.ClientID)
		require.Equal(t, "s3cret", s.ClientSecret)
		require.Equal(t, []string{"openid", "email"}, s.Scopes)
		// The nesting is the contract: flattening it would silently drop an
		// operator's domain restriction when they copy their YAML across.
		require.Equal(t, []string{"corp.example"}, s.JWTVerify.AllowedHostedDomains)
		require.NoError(t, in.Validate(parsed))
	})

	// The secret is bound to the client it was issued for, so both identifiers
	// have to reach the AAD from the parsed settings.
	t.Run("exposes its AAD binding", func(t *testing.T) {
		t.Parallel()

		parsed, err := in.Parse(json.RawMessage(config), map[string]string{"client_secret": "s3cret"})
		require.NoError(t, err)

		bound, ok := parsed.(integrationkinds.ClientBound)
		require.True(t, ok, "a login provider must declare its AAD binding")

		issuer, client := bound.AADBinding()
		require.Equal(t, "https://idp.corp.example", issuer)
		require.Equal(t, "client-abc", client)
	})

	// Validation judges a secret by PRESENCE, never by shape: on update an
	// unchanged secret arrives as an opaque sentinel.
	t.Run("requires every field the dance needs", func(t *testing.T) {
		t.Parallel()

		tests := map[string]string{
			"display_name": `{"issuer_url":"https://i.example","client_id":"c","redirect_uri":"https://a.example/cb"}`,
			"issuer_url":   `{"display_name":"n","client_id":"c","redirect_uri":"https://a.example/cb"}`,
			"client_id":    `{"display_name":"n","issuer_url":"https://i.example","redirect_uri":"https://a.example/cb"}`,
			"redirect_uri": `{"display_name":"n","issuer_url":"https://i.example","client_id":"c"}`,
		}
		for field, cfg := range tests {
			t.Run("missing "+field, func(t *testing.T) {
				t.Parallel()

				parsed, err := in.Parse(json.RawMessage(cfg), map[string]string{"client_secret": "s"})
				require.NoError(t, err)
				require.ErrorContains(t, in.Validate(parsed), field)
			})
		}
	})

	t.Run("requires the client secret", func(t *testing.T) {
		t.Parallel()

		parsed, err := in.Parse(json.RawMessage(config), nil)
		require.NoError(t, err)
		require.ErrorContains(t, in.Validate(parsed), "client_secret")
	})

	// An issuer reached over plain http can be rewritten in flight, and it is
	// the value the client secret is bound to and sent to.
	t.Run("refuses a plaintext issuer", func(t *testing.T) {
		t.Parallel()

		parsed, err := in.Parse(json.RawMessage(
			`{"display_name":"n","issuer_url":"http://idp.example","client_id":"c","redirect_uri":"https://a.example/cb"}`),
			map[string]string{"client_secret": "s"})
		require.NoError(t, err)
		require.ErrorContains(t, in.Validate(parsed), "https")
	})

	// Empty means NO restriction, and that is deliberate: hd is Google-specific,
	// so requiring a non-empty list would refuse every sign-in through an issuer
	// that mints no such claim -- which is every corporate IdP this feature is for.
	t.Run("an empty hosted-domain list is valid", func(t *testing.T) {
		t.Parallel()

		parsed, err := in.Parse(json.RawMessage(
			`{"display_name":"n","issuer_url":"https://idp.example","client_id":"c","redirect_uri":"https://a.example/cb"}`),
			map[string]string{"client_secret": "s"})
		require.NoError(t, err)
		require.NoError(t, in.Validate(parsed))
	})
}

// The server fetches {issuer_url}/.well-known/openid-configuration on its own
// behalf, and issuer_url now arrives from an admin through the API rather than
// from a file only shell access could edit. Without this the fetch can be aimed
// at cloud metadata or swept across loopback and private ranges, which is SSRF
// created by moving providers into the registry.
func TestOIDC_RefusesAnIssuerOnlyTheServerCanReach(t *testing.T) {
	t.Parallel()

	in := integrationkinds.Custom

	refused := map[string]string{
		"cloud metadata":   "https://169.254.169.254/latest/meta-data",
		"loopback v4":      "https://127.0.0.1/idp",
		"loopback v6":      "https://[::1]/idp",
		"localhost byname": "https://localhost/idp",
		"private 10/8":     "https://10.0.0.5/idp",
		"private 192.168":  "https://192.168.1.10/idp",
		"private 172.16":   "https://172.16.0.1/idp",
		"link-local v6":    "https://[fe80::1]/idp",
		"unspecified":      "https://0.0.0.0/idp",

		// Non-dotted-quad spellings of the SAME addresses. net.ParseIP returns
		// nil for these, so a guard that stops at "not an IP literal, must be a
		// name" waves them through -- while the resolver maps them straight back
		// to loopback. This is the DIRECT form the guard exists to close, not the
		// DNS-rebinding form it deliberately does not.
		"loopback as an integer": "https://2130706433/idp",
		"loopback in hex":        "https://0x7f000001/idp",
		"metadata as an integer": "https://2852039166/latest/meta-data",

		// Dotted spellings that are ALSO not dotted quads. An octet may be
		// written in octal (a leading zero), and trailing octets may be omitted
		// entirely -- "127.1" is the two-part form, where the final part fills
		// the remaining 24 bits. net.ParseIP refuses all three, the resolver
		// accepts all three, so they land in the same gap as the integer forms
		// above while looking nothing like them.
		"loopback short form":   "https://127.1/idp",
		"loopback in octal":     "https://017700000001/idp",
		"loopback dotted octal": "https://0177.0.0.1/idp",

		// Carrier-grade NAT, outside net.IP.IsPrivate. It is the internal
		// service range on several clouds, and 100.100.100.200 is Alibaba's
		// metadata endpoint -- the exact target this guard names.
		"cgnat":            "https://100.64.0.1/idp",
		"alibaba metadata": "https://100.100.100.200/latest/meta-data",

		// IETF protocol assignments and the documentation ranges, also outside
		// IsPrivate.
		"ietf protocol range": "https://192.0.0.1/idp",
		"test-net-1":          "https://192.0.2.5/idp",
	}

	for name, issuer := range refused {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()

			parsed, err := in.Parse(json.RawMessage(
				`{"display_name":"n","issuer_url":"`+issuer+`","client_id":"c","redirect_uri":"https://app.example/cb"}`),
				map[string]string{"client_secret": "s"})
			require.NoError(t, err)
			require.Error(t, in.Validate(parsed), "issuer %q must be refused", issuer)
		})
	}

	// A public issuer is the normal case and must stay allowed -- including one
	// whose host merely looks internal without being an address.
	t.Run("allows a public issuer", func(t *testing.T) {
		t.Parallel()

		for _, issuer := range []string{"https://accounts.google.com", "https://idp.corp.example", "https://localhost.example.com"} {
			parsed, err := in.Parse(json.RawMessage(
				`{"display_name":"n","issuer_url":"`+issuer+`","client_id":"c","redirect_uri":"https://app.example/cb"}`),
				map[string]string{"client_secret": "s"})
			require.NoError(t, err)
			require.NoError(t, in.Validate(parsed), "issuer %q must be allowed", issuer)
		}
	})
}

// Google and Custom are the same implementation registered twice. The property
// worth pinning is that they are DISTINGUISHABLE -- each answers with its own
// name -- while parsing identically, because the registry keys on the name and
// a shared answer would make one of them unreachable.
func TestLoginEntries_ShareOneImplementationUnderTwoNames(t *testing.T) {
	t.Parallel()

	require.Equal(t, "google", integrationkinds.Google.Name())
	require.Equal(t, "custom", integrationkinds.Custom.Name())
	require.NotEqual(t, integrationkinds.Google.Name(), integrationkinds.Custom.Name(),
		"two entries answering the same name would collide in the registry")

	require.Equal(t, integrationkinds.Google.SecretKeys(), integrationkinds.Custom.SecretKeys())

	const config = `{"display_name":"n","issuer_url":"https://idp.example",` +
		`"client_id":"c","redirect_uri":"https://app.example/cb"}`

	for _, in := range []integrationkinds.Integration{integrationkinds.Google, integrationkinds.Custom} {
		parsed, err := in.Parse(json.RawMessage(config), map[string]string{"client_secret": "s"})
		require.NoErrorf(t, err, "%s parses", in.Name())
		require.IsTypef(t, integrationkinds.OIDCSettings{}, parsed, "%s returns OIDC settings", in.Name())
		require.NoErrorf(t, in.Validate(parsed), "%s validates", in.Name())
	}
}

// SecurityRelevant has to be INJECTIVE: two different configurations must never
// render the same string.
//
// Nothing compares the rendering today. It reaches aadBindingOf, which stores it
// on aadBinding.security, and checkAADBindingStable then compares only
// aadInputs() -- the issuer and the client id. The consumer that compared this
// string wholesale, and treated equality as "nothing security-relevant changed",
// no longer exists -- it went the way of the linked-account check create.go
// describes, which the closed set of login names made unnecessary.
//
// The property is pinned anyway, because the field is still rendered and still
// carried: whoever wires a comparison back onto it inherits a collision as a
// re-point waved through, and would have no reason to suspect the rendering.
//
// allowed_hosted_domains has no element-level validation, so a separator byte
// survives into the field -- which is exactly how a joined-with-a-separator
// rendering collides.
func TestOIDCSettings_SecurityRelevantIsInjective(t *testing.T) {
	t.Parallel()

	render := func(redirect string, domains ...string) string {
		s := integrationkinds.OIDCSettings{RedirectURI: redirect}
		s.JWTVerify.AllowedHostedDomains = domains

		return s.SecurityRelevant()
	}

	// Two domains, versus one domain carrying the separator: different
	// configurations, and the second admits a domain the first does not.
	require.NotEqual(t,
		render("https://app/cb", "corp.example", "evil.example"),
		render("https://app/cb", "corp.example\x00evil.example"),
		"a separator inside a domain must not fake the boundary between two")

	// The boundary between the redirect and the domain list must hold too.
	require.NotEqual(t,
		render("https://app/cb", "corp.example"),
		render("https://app/cb\x00corp.example"),
		"a separator inside the redirect must not fake the field boundary")

	// And the ordinary case still distinguishes what it should.
	require.NotEqual(t,
		render("https://app/cb", "corp.example"),
		render("https://app/cb", "other.example"))
	require.Equal(t,
		render("https://app/cb", "corp.example"),
		render("https://app/cb", "corp.example"),
		"the same configuration must render identically, or every update is a re-point")
}
