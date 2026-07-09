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
