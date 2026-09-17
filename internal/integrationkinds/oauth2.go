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
	// nameGithub is the registry key of the first OAuth 2.0 entry, and it is
	// also what reaches user_identities.provider for every account linked
	// through it. Renaming it would orphan those rows, which is why the name is
	// fixed here rather than derived from anything an operator can edit.
	nameGithub = "github"
	// oauth2SecretKeyClientSecret is the confidential half of the OAuth app.
	//
	// Unlike OIDC, where the verification half needs only a client_id, EVERY
	// plain OAuth 2.0 sign-in spends this secret: an access token is opaque, so
	// the only way to learn who it belongs to is to ask the vendor with it, and
	// the only way to obtain it is the exchange this secret authorizes.
	oauth2SecretKeyClientSecret = "client_secret"
)

// OAuth2Settings is the parsed configuration for one plain OAuth 2.0 login
// provider.
//
// It is the SIBLING of OIDCSettings, and the pair names two PROTOCOLS rather
// than a protocol and a vendor. An OIDC provider is fully described by its
// issuer: discovery supplies every endpoint, which is why OIDCSettings serves
// google and custom alike. Plain OAuth 2.0 publishes no discovery document, so
// there is no issuer to describe it with and the endpoints are configuration in
// its place -- which is equally true of every vendor that speaks it, not of
// GitHub in particular.
//
// Nothing here is GitHub's. Every field is the authorization-code grant's:
// which client, where the browser consents, where the code is exchanged, and
// which API answers afterwards. A second vendor -- gitlab, bitbucket -- is a
// new registry NAME parsed by this same type, with its endpoints supplied by
// its own preset, and no new settings shape at all.
//
// A self-hosted deployment is likewise a different registry name rather than an
// override: blank endpoint fields would have to mean "use some default", and
// there is no default to use once the type stops being one vendor's.
//
// ClientSecret is merged in from the decrypted secrets (json:"-"). The type
// intentionally has no Stringer/marshaler, so it cannot be logged wholesale.
type OAuth2Settings struct {
	DisplayName string `json:"display_name"`
	ClientID    string `json:"client_id"`
	// ClientSecret authenticates this app at GitHub's token endpoint.
	ClientSecret string `json:"-"`
	// RedirectURI is this instance's EXTERNAL callback URL, and it must be the
	// external form -- see config.OIDCProvider.RedirectURI, which the OIDC path
	// shares verbatim.
	RedirectURI string `json:"redirect_uri"`
	// AuthorizeURL, TokenURL and APIBaseURL are supplied by the preset, not by
	// the operator: applyPreset refuses a create that sets them and
	// enforcePreset refuses an update that changes them. They are stored on the
	// row rather than read from the catalog at use time, for the reason
	// LoginPreset gives -- a value read live would change under providers
	// already using it.
	//
	// They are required, and validated as URLs, because nothing else supplies
	// them: with no discovery document there is no second source to fall back
	// on, so a row missing one could be created and would fail at sign-in.
	AuthorizeURL string `json:"authorize_url"`
	TokenURL     string `json:"token_url"`
	APIBaseURL   string `json:"api_base_url"`
}

// AADBinding implements ClientBound.
//
// The issuer is EMPTY, and that is the case the interface documents rather than
// a gap: GitHub has no issuer URL to bind to, because its endpoints are fixed
// and cannot be re-pointed by editing a row. The client id still binds, so a
// secret sealed for one OAuth app cannot be opened under another.
func (s OAuth2Settings) AADBinding() (issuerURL, clientID string) {
	return "", s.ClientID
}

// SecurityRelevant covers the fields an operator can change to alter who gets
// in or where credentials travel, on top of the client id the AAD already
// binds.
//
// The three endpoints are here because they are where the client secret and the
// authorization code travel. The preset already refuses a change to them, but
// this is the check that does not depend on the row still being preset-backed.
//
// redirect_uri is the one that matters here: it names where an authorization
// code is delivered. GitHub's own app registration is the real control, but a
// change on this side is worth re-consenting to.
//
// There is no hosted-domain equivalent to guard: that restriction is a Google
// claim, and GitHub mints nothing like it. What GitHub does enforce -- that the
// address be primary and verified -- is not configurable, so it cannot be
// weakened by an edit.
//
// Length-prefixed rather than joined, for the reason OIDCSettings gives: the
// caller compares two renderings wholesale, so a separator surviving into a
// scope would let ["a", "b"] and ["a<sep>b"] collide and wave a real change
// through.
func (s OAuth2Settings) SecurityRelevant() string {
	parts := []string{s.RedirectURI, s.AuthorizeURL, s.TokenURL, s.APIBaseURL}

	var buf []byte
	for _, part := range parts {
		buf = binary.AppendUvarint(buf, uint64(len(part)))
		buf = append(buf, part...)
	}

	return string(buf)
}

// oauth2 implements the Integration contract for a plain OAuth 2.0 login entry.
//
// Parameterised by name, exactly as oidc is: one implementation, one entry per
// vendor. What separates two entries is their PRESET -- each vendor's endpoints
// are its own -- not their settings shape, which is the grant's and therefore
// shared.
//
// Adding gitlab is a value here and a catalog entry beside google's, with no
// new type and no new arm in the reloader.
type oauth2 struct {
	// name is the registry key this entry answers to. The struct is otherwise
	// stateless: one value of it IS one entry.
	name string
}

func (o oauth2) Name() string { return o.name }

// PresetKey implements Preseted: the endpoints come from the catalog, not from
// the operator.
//
// That is what makes them unforgeable rather than merely defaulted. applyPreset
// refuses a create that supplies one and enforcePreset refuses an update that
// changes one, so nobody can point a name accounts already use at a host they
// control and mint whatever subject they like against the user_identities rows
// carrying it. A missing catalog entry is a refusal, not "the operator supplies
// everything".
//
// Every entry of this kind is preset-backed, unlike oidc's `custom`: there is no
// generic plain-OAuth-2.0 provider an operator could point anywhere, because
// the endpoints ARE the provider.
func (o oauth2) PresetKey() string { return o.name }

// Category is login: this row signs people in. It deliberately does NOT imply
// OIDC -- see the comment on CategoryLogin, which names this entry as the case
// the category was widened for.
func (oauth2) Category() string { return CategoryLogin }

// SecretKeys is what the service encrypts at rest and masks on read.
func (oauth2) SecretKeys() []string { return []string{oauth2SecretKeyClientSecret} }

func (oauth2) Parse(config json.RawMessage, secrets map[string]string) (Settings, error) {
	var s OAuth2Settings
	if err := unmarshalConfig(config, &s); err != nil {
		return nil, err
	}
	s.ClientSecret = secrets[oauth2SecretKeyClientSecret]

	return s, nil
}

// Validate refuses a half-configured provider at the edge.
//
// client_secret is UNCONDITIONALLY required, which is where this diverges from
// what the OIDC path once allowed in a config file. OIDC has a mode that needs
// no secret: the frontend runs the dance and posts an id_token the backend
// verifies offline. Plain OAuth 2.0 has no such mode -- there is no id_token to
// post, and the identity can only be fetched with a token this secret buys. An
// instance carrying only a client_id could therefore be created, listed, and
// draw a button that refuses every sign-in.
//
// There is no issuer_url: GitHub has no issuer. The three endpoints below take
// its place as the addresses a secret is spent against, so they carry the same
// HTTPSURL rule an issuer would -- an endpoint reached over plain http can be
// rewritten in flight, and the client secret travels to the token one.
func (o oauth2) Validate(settings Settings) error {
	s, ok := settings.(OAuth2Settings)
	if !ok {
		return fmt.Errorf("%s: unexpected settings type %T", o.Name(), settings)
	}

	return validation.ValidateStruct(&s,
		validation.Field(&s.DisplayName, validation.Required.Error("display_name is required")),
		validation.Field(&s.ClientID, validation.Required.Error("client_id is required")),
		validation.Field(&s.ClientSecret, validation.Required.Error("client_secret is required")),
		validation.Field(&s.RedirectURI,
			validation.Required.Error("redirect_uri is required"),
			is.URL,
		),
		// The preset fills these, so a failure here means the catalog entry is
		// incomplete rather than that an operator typed something wrong -- which
		// is why the messages name the field rather than the form.
		validation.Field(&s.AuthorizeURL,
			validation.Required.Error("authorize_url is required"),
			is.URL,
			validation.By(xvalidation.HTTPSURL),
		),
		validation.Field(&s.TokenURL,
			validation.Required.Error("token_url is required"),
			is.URL,
			validation.By(xvalidation.HTTPSURL),
		),
		validation.Field(&s.APIBaseURL,
			validation.Required.Error("api_base_url is required"),
			is.URL,
			validation.By(xvalidation.HTTPSURL),
		),
	)
}
