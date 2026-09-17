package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

func presetSvc(t *testing.T) *Service {
	t.Helper()

	registry, err := NewRegistry(
		integrationkinds.Google, integrationkinds.Custom, integrationkinds.GitHub)
	require.NoError(t, err)

	return (&Service{registry: registry}).WithLoginPresets(config.LoginPresets{
		"google": {DisplayName: "Google", IssuerURL: "https://accounts.google.com"},
		"github": {
			DisplayName:  "GitHub",
			AuthorizeURL: "https://github.com/login/oauth/authorize",
			TokenURL:     "https://github.com/login/oauth/access_token",
			APIBaseURL:   "https://api.github.com",
		},
	})
}

// A preset supplies what is knowable in advance, and the operator supplies the
// rest. The value is COPIED INTO THE CONFIG here rather than read at use time:
// issuer_url is an AAD input, so a live read would mean editing the catalog
// silently strands every secret sealed under the old value.
func TestApplyPreset_CopiesKnownFieldsIntoTheRow(t *testing.T) {
	t.Parallel()

	got, err := presetSvc(t).applyPreset("google",
		json.RawMessage(`{"client_id":"c","redirect_uri":"https://app.example/cb"}`))
	require.NoError(t, err)

	var cfg map[string]any
	require.NoError(t, json.Unmarshal(got, &cfg))
	require.Equal(t, "https://accounts.google.com", cfg["issuer_url"],
		"the preset issuer must be written into the stored config")
	require.Equal(t, "Google", cfg["display_name"])
	require.Equal(t, "c", cfg["client_id"], "operator fields must survive")
}

// A preset field is a CONSTANT, not a default. Honoring a caller-supplied
// issuer under a preset name would let an operator create "google" pointed at
// an IdP they control, which mints whatever subject it likes against the
// user_identities rows already carrying provider='google'.
func TestApplyPreset_RefusesACallerSuppliedPresetField(t *testing.T) {
	t.Parallel()

	_, err := presetSvc(t).applyPreset("google",
		json.RawMessage(`{"issuer_url":"https://attacker.example","client_id":"c"}`))
	require.ErrorIs(t, err, apperr.ErrValidation)
	require.ErrorContains(t, err, "issuer_url")
}

// custom is the one entry with no catalog record, and the operator supplies
// everything -- including an arbitrary issuer, which is legal there because
// the name says so.
func TestApplyPreset_CustomPassesThroughUntouched(t *testing.T) {
	t.Parallel()

	in := json.RawMessage(`{"issuer_url":"https://idp.corp.example","client_id":"c"}`)
	got, err := presetSvc(t).applyPreset("custom", in)
	require.NoError(t, err)
	require.JSONEq(t, string(in), string(got))
}

// A preset NAME whose catalog entry is missing is a refusal, never a
// fallback to operator input: otherwise deleting one line of YAML reopens the
// takeover path above.
func TestApplyPreset_MissingCatalogEntryIsRefused(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(integrationkinds.Google, integrationkinds.Custom)
	require.NoError(t, err)
	svc := (&Service{registry: registry}).WithLoginPresets(config.LoginPresets{})

	_, err = svc.applyPreset("google", json.RawMessage(`{"client_id":"c"}`))
	require.ErrorIs(t, err, apperr.ErrValidation)
}

// The preset invariant has to hold on UPDATE, not only on create.
//
// Update replaces Config wholesale, so without a check here an operator creates
// `google` correctly and then rewrites its issuer in a second call -- ending up
// with a provider named `google`, trusted by every user_identities row already
// carrying provider='google', pointed at an IdP they run. The create-time
// refusal buys nothing if the field is writable one request later.
//
// Nothing else closes this. The linked-account check that once refused a name
// identities already carried is gone -- create.go says why, and it would not
// have helped regardless: it fired only once an account was linked, and the
// window before the first sign-in is exactly when an operator is setting the
// provider up.
func TestEnforcePreset_RefusesAnUpdateThatRewritesAPresetField(t *testing.T) {
	t.Parallel()

	svc := presetSvc(t)

	// Rewritten to an issuer the operator controls. display_name is left at the
	// catalog value on purpose, so exactly one field disagrees -- enforcePreset
	// iterates a map, and a config with two wrong fields would name whichever
	// one Go's random map order reached first, making the assertion flaky.
	err := svc.enforcePreset("google", json.RawMessage(
		`{"issuer_url":"https://attacker.example","display_name":"Google","client_id":"c"}`))
	require.ErrorIs(t, err, apperr.ErrValidation)
	require.ErrorContains(t, err, "issuer_url")

	// Dropped entirely -- the quieter half of the same bug, which would leave a
	// preset provider with no issuer at all.
	err = svc.enforcePreset("google", json.RawMessage(`{"client_id":"c"}`))
	require.ErrorIs(t, err, apperr.ErrValidation)

	// The catalog's own values are what a legitimate update carries, because
	// create wrote them into the row.
	require.NoError(t, svc.enforcePreset("google", json.RawMessage(
		`{"issuer_url":"https://accounts.google.com","display_name":"Google","client_id":"c"}`)))

	// custom owns its issuer, so any value is legal there.
	require.NoError(t, svc.enforcePreset("custom",
		json.RawMessage(`{"issuer_url":"https://idp.corp.example","client_id":"c"}`)))
}

// A preset name whose catalog entry is MISSING is refused on update too.
//
// applyPreset has this covered on create; enforcePreset did not, and the two
// close the same hole. Without it, deleting one line of YAML stops an update to
// the `google` row being compared against the catalog at all -- issuer_url
// included, which is the field the whole rule exists for.
func TestEnforcePreset_MissingCatalogEntryIsRefused(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(integrationkinds.Google, integrationkinds.Custom)
	require.NoError(t, err)

	// A deployment whose catalog lost the entry -- or never had it.
	svc := (&Service{registry: registry}).WithLoginPresets(config.LoginPresets{})

	err = svc.enforcePreset("google", json.RawMessage(
		`{"issuer_url":"https://attacker.example","client_id":"c"}`))
	require.ErrorIs(t, err, apperr.ErrValidation,
		"a preset name with no catalog entry must be refused, not waved through")
}

// The plain OAuth 2.0 entries are preset-backed for a sharper reason than the
// OIDC ones, and this is the test that holds it.
//
// An OIDC row pointed at a hostile issuer still has to produce an id_token that
// verifies against that issuer's JWKS. A plain OAuth 2.0 row has no such check:
// whoever owns the endpoints owns the identity, because the identity is
// whatever the API answers. So the three URLs must be the catalog's, and an
// entry that quietly stopped being preset-backed -- PresetKey returning "" --
// would hand them to the operator with nothing failing.
func TestApplyPreset_OAuth2EndpointsAreTheCatalogs(t *testing.T) {
	t.Parallel()

	got, err := presetSvc(t).applyPreset("github",
		json.RawMessage(`{"client_id":"c","redirect_uri":"https://app.example/cb"}`))
	require.NoError(t, err)

	var cfg map[string]any
	require.NoError(t, json.Unmarshal(got, &cfg))
	require.Equal(t, "https://github.com/login/oauth/authorize", cfg["authorize_url"])
	require.Equal(t, "https://github.com/login/oauth/access_token", cfg["token_url"])
	require.Equal(t, "https://api.github.com", cfg["api_base_url"])
	require.Equal(t, "c", cfg["client_id"], "operator fields must survive")
}

// The attack the entry above exists to refuse: an operator creating `github`
// with a token endpoint they control would receive this app's client_secret at
// it, and could then mint any subject against the user_identities rows already
// carrying provider='github'.
func TestApplyPreset_RefusesACallerSuppliedOAuth2Endpoint(t *testing.T) {
	t.Parallel()

	_, err := presetSvc(t).applyPreset("github",
		json.RawMessage(`{"token_url":"https://attacker.example/token","client_id":"c"}`))
	require.ErrorIs(t, err, apperr.ErrValidation)
	require.ErrorContains(t, err, "token_url")
}
