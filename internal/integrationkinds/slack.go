// slack and telegram are intentionally parallel bot-token kinds: the symmetry
// is the design (same config shape), and they diverge independently.
//
//nolint:dupl
package integrationkinds

import (
	"encoding/json"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/utils/xvalidation"
)

const (
	// kindSlack is DERIVED from the transport constant: a delivery-capable
	// kind and its channel transport are one name by construction, not two
	// strings glued by a test.
	kindSlack              = string(entity.NotifyTransportSlack)
	slackSecretKeyBotToken = "bot_token"
)

// SlackSettings is the parsed configuration for the Slack integration. Config
// fields are populated by json.Unmarshal from the stored config; BotToken is
// merged in from the decrypted secrets, never from config JSON. Timeout is a
// string (e.g. "10s") parsed at build time, since JSON has no duration literal.
// The type has no Stringer/marshaler: BotToken must never be logged by accident.
type SlackSettings struct {
	BotToken string `json:"-"`
	APIURL   string `json:"api_url"`
	Timeout  string `json:"timeout"`
}

// Kind marks SlackSettings as the "slack" settings value (integrationkinds.Settings).
func (SlackSettings) Kind() string { return kindSlack }

// Slack implements the integrationkinds.Integration contract for kind "slack".
type slack struct{}

func (slack) Kind() string { return kindSlack }

// SecretKeys returns a fresh slice each call so a caller cannot mutate an
// internal source of truth.
func (slack) SecretKeys() []string { return []string{slackSecretKeyBotToken} }

func (slack) Parse(config json.RawMessage, secrets map[string]string) (Settings, error) {
	var s SlackSettings
	if err := unmarshalConfig(config, &s); err != nil {
		return nil, err
	}
	s.BotToken = secrets[slackSecretKeyBotToken]
	return s, nil
}

func (slack) Validate(settings Settings) error {
	s, ok := settings.(SlackSettings)
	if !ok {
		return fmt.Errorf("slack: unexpected settings type %T", settings)
	}
	return validation.ValidateStruct(&s,
		validation.Field(&s.BotToken, validation.Required.Error("bot_token is required")),
		// Syntactic check only: where the URL points is the admin's call (self-hosted
		// proxies/gateways are legitimate); this catches typos, not exfiltration.
		validation.Field(&s.APIURL, validation.When(s.APIURL != "", is.URL)),
		validation.Field(&s.Timeout, validation.When(s.Timeout != "", validation.WithContext(xvalidation.IsDuration))),
	)
}
