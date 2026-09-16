package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

// clientBoundSettings stands in for a login provider: its secrets are sealed
// against identifiers that live in the config, so moving either one strands any
// ciphertext carried forward unchanged.
type clientBoundSettings struct {
	Issuer         string   `json:"issuer_url"`
	ClientID       string   `json:"client_id"`
	RedirectURI    string   `json:"redirect_uri"`
	AllowedDomains []string `json:"allowed_domains"`
}

func (c clientBoundSettings) AADBinding() (issuerURL, clientID string) { return c.Issuer, c.ClientID }

// SecurityRelevant is what the linked-account guard compares, and it is wider
// than the AAD on purpose: these fields change who gets in without moving
// anything the secret is sealed under.
func (c clientBoundSettings) SecurityRelevant() string {
	return strings.Join(append([]string{c.RedirectURI, "|"}, c.AllowedDomains...), "\x00")
}

type clientBoundKind struct{}

func (clientBoundKind) Name() string         { return "fake-login" }
func (clientBoundKind) Category() string     { return integrationkinds.CategoryLogin }
func (clientBoundKind) SecretKeys() []string { return []string{"client_secret"} }

func (clientBoundKind) Parse(config json.RawMessage, _ map[string]string) (integrationkinds.Settings, error) {
	var s clientBoundSettings
	if len(config) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(config, &s); err != nil {
		return nil, err
	}

	return s, nil
}

func (clientBoundKind) Validate(integrationkinds.Settings) error { return nil }

// plainKind is any kind whose secrets are bound by (kind, key) alone -- every
// delivery integration that exists today.
type plainKind struct{}

func (plainKind) Name() string         { return "slack" }
func (plainKind) Category() string     { return integrationkinds.CategoryNotify }
func (plainKind) SecretKeys() []string { return []string{"bot_token"} }

func (plainKind) Parse(json.RawMessage, map[string]string) (integrationkinds.Settings, error) {
	return plainSettings{}, nil
}
func (plainKind) Validate(integrationkinds.Settings) error { return nil }

type plainSettings struct{}

// storedRow is the row as persisted before the update under test: one login
// provider with a secret already sealed against the config it carries.
func storedRow(t *testing.T) *entity.IntegrationSetting {
	t.Helper()

	return &entity.IntegrationSetting{
		Kind: "fake-login",
		Name: "corporate",
		Config: json.RawMessage(
			`{"issuer_url":"https://corp.example","client_id":"abc","allowed_domains":["corp.example"]}`),
		Secrets: map[string]string{"client_secret": "stored-ciphertext"},
	}
}

// The invariant is about the AAD inputs moving, not about any one field being
// named in the request: mergeSecrets carries an unchanged ciphertext forward
// without re-encrypting, so an absent key is exactly the dangerous case.
func TestCheckAADBindingStable(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	in := clientBoundKind{}
	actor := &entity.User{}

	t.Run("issuer change without the secret is refused", func(t *testing.T) {
		t.Parallel()

		err := svc.checkAADBindingStable(in, storedRow(t), &entity.UpdateIntegrationCmd{
			Kind:   "fake-login",
			Config: json.RawMessage(`{"issuer_url":"https://other.example","client_id":"abc","allowed_domains":["corp.example"]}`),
			Actor:  actor,
		}, nil)

		require.ErrorIs(t, err, apperr.ErrValidation)
		require.Contains(t, err.Error(), "client_secret",
			"the message must name the secret the operator has to resupply")
	})

	t.Run("client id change without the secret is refused", func(t *testing.T) {
		t.Parallel()

		err := svc.checkAADBindingStable(in, storedRow(t), &entity.UpdateIntegrationCmd{
			Kind:   "fake-login",
			Config: json.RawMessage(`{"issuer_url":"https://corp.example","client_id":"xyz","allowed_domains":["corp.example"]}`),
			Actor:  actor,
		}, nil)

		require.ErrorIs(t, err, apperr.ErrValidation)
	})

	t.Run("issuer change with the secret resupplied is allowed", func(t *testing.T) {
		t.Parallel()

		secret := "new-secret"
		err := svc.checkAADBindingStable(in, storedRow(t), &entity.UpdateIntegrationCmd{
			Kind:   "fake-login",
			Config: json.RawMessage(`{"issuer_url":"https://other.example","client_id":"abc","allowed_domains":["corp.example"]}`),
			Actor:  actor,
		}, map[string]*string{"client_secret": &secret})

		require.NoError(t, err)
	})

	// An unrelated config edit must stay cheap: the operator changing a display
	// name should not be told to re-enter credentials.
	t.Run("an edit that leaves the binding alone is allowed", func(t *testing.T) {
		t.Parallel()

		err := svc.checkAADBindingStable(in, storedRow(t), &entity.UpdateIntegrationCmd{
			Kind:   "fake-login",
			Config: json.RawMessage(`{"issuer_url":"https://corp.example","client_id":"abc","allowed_domains":["corp.example"],"scopes":["openid"]}`),
			Actor:  actor,
		}, nil)

		require.NoError(t, err)
	})

	// Normalization is the difference between a guard that cries wolf and one
	// that misses a real change: discovery trims the trailing slash and the host
	// is case-insensitive, so neither is a new issuer. Whitespace on either side
	// of that slash counts too -- it is the same IdP typed untidily.
	t.Run("trailing slash and host case are not a change", func(t *testing.T) {
		t.Parallel()

		err := svc.checkAADBindingStable(in, storedRow(t), &entity.UpdateIntegrationCmd{
			Kind:   "fake-login",
			Config: json.RawMessage(`{"issuer_url":" https://CORP.example/ ","client_id":"abc","allowed_domains":["corp.example"]}`),
			Actor:  actor,
		}, nil)

		require.NoError(t, err)
	})

	// Nothing stored under the key means nothing can be stranded.
	t.Run("no stored secret means no obligation", func(t *testing.T) {
		t.Parallel()

		row := storedRow(t)
		row.Secrets = nil

		err := svc.checkAADBindingStable(in, row, &entity.UpdateIntegrationCmd{
			Kind:   "fake-login",
			Config: json.RawMessage(`{"issuer_url":"https://other.example","client_id":"abc","allowed_domains":["corp.example"]}`),
			Actor:  actor,
		}, nil)

		require.NoError(t, err)
	})

	// A kind whose secrets are not client-bound carries nothing in its AAD that
	// an edit can move, so it must never be asked to resupply.
	t.Run("a non-client-bound kind is unaffected", func(t *testing.T) {
		t.Parallel()

		err := svc.checkAADBindingStable(plainKind{}, storedRow(t),
			&entity.UpdateIntegrationCmd{
				Kind:   "slack",
				Config: json.RawMessage(`{"api_url":"https://new.example"}`),
				Actor:  actor,
			}, nil)

		require.NoError(t, err)
	})
}
