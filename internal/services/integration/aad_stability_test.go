package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
)

// systemNamed is a delivery integration reporting the given system name. It
// wraps a real kind so the settings side is unchanged and only the registry key
// varies -- which is the axis this test is about.
type systemNamed string

func (s systemNamed) Name() string       { return string(s) }
func (systemNamed) Category() string     { return integrationkinds.CategoryNotify }
func (systemNamed) SecretKeys() []string { return integrationkinds.Slack.SecretKeys() }

func (systemNamed) Parse(c json.RawMessage, sec map[string]string) (integrationkinds.Settings, error) {
	return integrationkinds.Slack.Parse(c, sec)
}
func (systemNamed) Validate(s integrationkinds.Settings) error {
	return integrationkinds.Slack.Validate(s)
}

// TestSecretAAD_DeliveryBindingIsTheSystemName is a regression lock on the one
// value in this change that must NOT move.
//
// Every delivery secret stored today -- every bot_token, every SMTP password --
// was sealed with secrets.SecretAAD(<system name>, key). This change renames the
// registry's key method and turns integration_settings.kind into a category, so
// there is an obvious-looking edit that makes secretAAD feed the CATEGORY into
// the AAD instead. Nothing would fail at compile time and no existing test would
// go red: the damage shows up later as ErrIntegrationUnreadable on a read nobody
// connects to this change.
//
// So the expected AADs are spelled out as literals rather than derived from the
// registry. A test that computed them the same way the code does would agree
// with the code however wrong the code became.
func TestSecretAAD_DeliveryBindingIsTheSystemName(t *testing.T) {
	t.Parallel()

	// What the stored ciphertext is bound to, written out by hand.
	for _, tc := range []struct{ system, key string }{
		{"slack", "bot_token"},
		{"telegram", "bot_token"},
		{"email", "password"},
	} {
		require.Equal(t,
			secrets.SecretAAD(tc.system, tc.key),
			secretAAD(systemNamed(tc.system), nil, tc.key),
			"the AAD of a %q secret must stay bound to the system name", tc.system)

		require.NotEqual(t,
			secrets.SecretAAD("notify", tc.key),
			secretAAD(systemNamed(tc.system), nil, tc.key),
			"binding a delivery secret to the category would strand every stored %q secret", tc.system)
	}
}
