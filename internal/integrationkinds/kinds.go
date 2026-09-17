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
	// GitHub is the first login entry that is not OIDC: it speaks plain OAuth
	// 2.0, which has no discovery document, so its endpoints are configuration
	// and its settings are a different shape.
	//
	// One implementation, one entry per vendor -- the same arrangement google
	// and custom have. A second vendor is another value here plus a catalog
	// entry, not another type.
	//
	// It carries no presetKey beside its name because every oauth2 entry is
	// preset-backed: PresetKey returns the name itself. There is no counterpart
	// to `custom` here -- a plain OAuth 2.0 provider IS its endpoints, so an
	// entry an operator could point anywhere would be one whose identity nobody
	// vouches for.
	GitHub Integration = oauth2{name: nameGithub}
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
	// CategoryLogin is a sign-in provider. It is the category the delete
	// cascade, the login reloader and the admin health field key on; before the
	// rename each of them compared against the "oidc" kind, which stopped
	// meaning "a login provider" the moment login rows could carry more than
	// one system name.
	//
	// The category does NOT imply OIDC, and no longer only in principle: github
	// speaks plain OAuth 2.0, has no discovery document, and belongs here on the
	// same footing as google and custom. What the category promises is "this row
	// signs people in" and nothing about how.
	//
	// The one place that knows a shape is the reloader's buildOne, which
	// switches on the settings TYPE. Everything downstream of it holds an
	// AuthMethod and a Gateway, both interfaces, so a further shape costs one
	// arm there and nothing here.
	CategoryLogin = "login"
)
