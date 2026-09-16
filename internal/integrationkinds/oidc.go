package integrationkinds

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"

	"github.com/ruko1202/maintmode/internal/utils/xvalidation"
)

const (
	// The two names the OIDC implementation is registered under. They are NOT
	// derived from a transport constant, unlike slack/telegram/email: a login
	// provider delivers nothing, so it has no NotifyTransport to take its name
	// from, and it must never appear in the resolver's builder map.
	//
	// One implementation, two entries. They differ only in the preset they
	// carry: "google" knows its issuer in advance, "custom" knows nothing and
	// asks the operator for everything. Grafana draws the same line between its
	// named providers and generic_oauth.
	nameGoogle = "google"
	nameCustom = "custom"
	// oidcSecretKeyClientSecret is the confidential half of the OAuth client.
	// The verification half (checking an ID token against the provider's JWKS)
	// needs only client_id, which is why this is the sole secret here.
	oidcSecretKeyClientSecret = "client_secret"
)

// OIDCSettings is the parsed configuration for one OIDC login provider.
//
// The field names are the config file's names verbatim, nesting included, so an
// operator moving a provider out of YAML sees the same words in the same shape.
// ClientSecret is merged in from the decrypted secrets (json:"-"). The type
// intentionally has no Stringer/marshaler, so it cannot be logged wholesale.
type OIDCSettings struct {
	DisplayName  string `json:"display_name"`
	IssuerURL    string `json:"issuer_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"-"`
	RedirectURI  string `json:"redirect_uri"`
	// Scopes defaults to openid/email/profile at build time when empty; an
	// explicit list replaces rather than extends it.
	Scopes []string `json:"scopes"`
	// JWTVerify carries the one provider-facing knob the OIDC path actually
	// reads. It stays nested under "jwtverifier" to match the config file: the
	// field inside it is an authorization control, and an operator copying their
	// YAML must not have to notice that it moved.
	JWTVerify OIDCJWTVerify `json:"jwtverifier"`
}

// OIDCJWTVerify is the subset of the config's jwtverifier block that applies to
// an upstream provider. The other three fields there (jwt_issuer, jwks_url,
// jwt_leeway) concern this backend's OWN tokens and are deliberately absent:
// exposing inert knobs in an admin form invites misconfiguration.
type OIDCJWTVerify struct {
	// AllowedHostedDomains restricts sign-in to accounts whose Google `hd`
	// claim matches one of these domains.
	//
	// EMPTY MEANS NO RESTRICTION, which is the fail-open direction and is
	// deliberate: `hd` is Google-specific, and every other issuer (Keycloak,
	// Okta, Authentik, Azure AD) mints no such claim. Requiring a non-empty
	// list would make the restriction mandatory for providers that cannot
	// satisfy it, refusing every sign-in through them.
	AllowedHostedDomains []string `json:"allowed_hosted_domains"`
}

// AADBinding implements ClientBound: an OIDC secret is sealed against the issuer
// it was minted for and the client it belongs to.
func (s OIDCSettings) AADBinding() (issuerURL, clientID string) {
	return s.IssuerURL, s.ClientID
}

// SecurityRelevant covers the fields an operator can change to alter who gets
// in, on top of the two the AAD already binds.
//
// allowed_hosted_domains is the sharp one: emptying it is a one-field edit that
// turns a Workspace-restricted provider into one admitting every account the
// IdP will vouch for, and it moves no AAD input, so the binding check alone
// never sees it. redirect_uri is here because it names where an authorization
// code is delivered -- the IdP's own registration is the real control, but a
// change to it on a live provider is not routine maintenance either.
// LENGTH-PREFIXED rather than joined by a separator, and that is the whole
// correctness argument. The caller compares two renderings wholesale and reads
// equality as "nothing security-relevant changed", so a collision means a real
// re-point is waved through on a provider accounts already sign in with.
// allowed_hosted_domains has no element-level validation, so a separator byte
// survives into the field -- and a joined rendering cannot then tell
// ["a", "b"] from ["a<sep>b"]. Same reasoning, and the same encoding, as
// secrets.encodeAAD.
func (s OIDCSettings) SecurityRelevant() string {
	parts := append([]string{s.RedirectURI}, s.JWTVerify.AllowedHostedDomains...)

	var buf []byte
	for _, part := range parts {
		buf = binary.AppendUvarint(buf, uint64(len(part)))
		buf = append(buf, part...)
	}

	return string(buf)
}

// oidc implements the Integration contract for kind "oidc". Several rows of
// this kind coexist on purpose -- a corporate IdP alongside Google -- which is
// what (kind, name) addressing is for.
type oidc struct {
	// name is the registry key this entry answers to. The struct is otherwise
	// stateless: two values of it ARE the two entries.
	name string
	// presetKey is the catalog key its defaults are filed under. Normally the
	// same as name; they differ only where a test registers this entry under a
	// unique name to keep parallel runs apart.
	presetKey string
}

func (o oidc) Name() string { return o.name }

// Category is login for both entries: google and custom are the same
// implementation, and the preset is the only thing separating them.
func (oidc) Category() string { return CategoryLogin }

// PresetKey implements Preseted for the entries whose defaults the catalog
// supplies. `custom` returns "" and is therefore NOT preset-backed: it is the
// one entry where the operator supplies every field, which is what the name
// announces.
//
// Returning the catalog key rather than a bool is what lets the integration
// tests register this implementation under a per-test name while still hitting
// the real preset path -- the key travels with the entry instead of being
// re-derived from the row's name somewhere else.
func (o oidc) PresetKey() string {
	if o.name == nameCustom {
		return ""
	}

	return o.presetKey
}

func (oidc) SecretKeys() []string { return []string{oidcSecretKeyClientSecret} }

func (oidc) Parse(config json.RawMessage, secrets map[string]string) (Settings, error) {
	var s OIDCSettings
	if err := unmarshalConfig(config, &s); err != nil {
		return nil, err
	}
	s.ClientSecret = secrets[oidcSecretKeyClientSecret]

	return s, nil
}

func (o oidc) Validate(settings Settings) error {
	s, ok := settings.(OIDCSettings)
	if !ok {
		return fmt.Errorf("%s: unexpected settings type %T", o.name, settings)
	}

	// client_secret and redirect_uri are both REQUIRED here, unlike in the
	// config file where they are all-or-nothing: a provider configured through
	// the registry exists to sign people in, so the half-configured
	// verification-only state is rejected at the edge rather than discovered
	// when someone tries to log in.
	return validation.ValidateStruct(&s,
		validation.Field(&s.DisplayName, validation.Required.Error("display_name is required")),
		validation.Field(&s.IssuerURL,
			validation.Required.Error("issuer_url is required"),
			is.URL,
			// No escape hatch for loopback, deliberately: this is the field the
			// client secret is bound to and sent to, and an issuer reached over
			// plain http can be rewritten in flight.
			validation.By(xvalidation.HTTPSURL),
		),
		validation.Field(&s.ClientID, validation.Required.Error("client_id is required")),
		validation.Field(&s.ClientSecret, validation.Required.Error("client_secret is required")),
		validation.Field(&s.RedirectURI,
			validation.Required.Error("redirect_uri is required"),
			is.URL,
		),
	)
}
