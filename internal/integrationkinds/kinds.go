// Package integrationkinds is the NEUTRAL vocabulary of integration kinds: the settings
// contract (Integration), the per-kind typed Settings with Parse/Validate, and
// shared config vocabulary (TLS policies, timeout format). It is owned by
// neither service: the integration registry consumes the contract to validate
// and store settings; the transport resolver consumes the concrete Settings
// types to build clients. Both depend downward on this package and never on
// each other. This package must not import gateways or either service.
package integrationkinds

import (
	"encoding/json"
	"fmt"
)

var (
	Slack    Integration = slack{}
	Email    Integration = email{}
	Telegram Integration = telegram{}
	// Login providers. They deliver nothing, so they have no transport builder
	// and must never join the resolver's builder map -- see
	// TestBuilders_AlignWithIntegrationKinds.
	//
	// Google and Custom are the SAME implementation registered under two names.
	// What separates them is the preset: Google's issuer is known in advance and
	// is refused as operator input, while Custom asks for everything. Registering
	// one object twice is impossible -- NewRegistry keys on Name() and rejects a
	// duplicate -- so each entry is its own value.
	Google Integration = oidc{name: nameGoogle, presetKey: nameGoogle}
	Custom Integration = oidc{name: nameCustom}
)

// unmarshalConfig decodes the raw config JSON into a kind's typed Settings. An
// empty/nil config is treated as an empty object so all fields default to their
// zero value; a malformed JSON body is an error so a bad config fails loudly.
func unmarshalConfig(config json.RawMessage, dst any) error {
	if len(config) == 0 {
		return nil
	}
	if err := json.Unmarshal(config, dst); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// Categories an integration_settings row can belong to. They are what the
// .kind column holds after the registry moved to keying by system name: a row
// answers "which half of the product is this" with the category, and "which
// system" with the name.
//
// Declared as constants rather than derived from an implementation because the
// category is a property of the ROW, not of the code that parses it: no
// Integration implements "notify".
const (
	// CategoryNotify is a delivery integration -- Slack, Telegram, SMTP.
	CategoryNotify = "notify"
	// CategoryLogin is a sign-in provider. It is the category the
	// linked-account guard, the login reloader and the admin health field key
	// on; before the rename each of them compared against the "oidc" kind,
	// which stopped meaning "a login provider" the moment login rows could
	// carry more than one system name.
	CategoryLogin = "login"
)
