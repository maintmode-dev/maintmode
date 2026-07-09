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
	// kindTelegram is derived from the transport constant (see kindSlack).
	kindTelegram        = string(entity.NotifyTransportTelegram)
	tgSecretKeyBotToken = "bot_token"
)

// TelegramSettings is the parsed configuration for the Telegram integration.
// BotToken is merged in from decrypted secrets (json:"-"); Timeout is a string
// parsed at build time. The type intentionally has no Stringer/marshaler.
type TelegramSettings struct {
	BotToken string `json:"-"`
	APIURL   string `json:"api_url"`
	Timeout  string `json:"timeout"`
}

// Kind marks TelegramSettings as the "telegram" settings value (integrationkinds.Settings).
func (TelegramSettings) Kind() string { return kindTelegram }

// Telegram implements the integrationkinds.Integration contract for kind "telegram".
type telegram struct{}

func (telegram) Kind() string { return kindTelegram }

func (telegram) SecretKeys() []string { return []string{tgSecretKeyBotToken} }

func (telegram) Parse(config json.RawMessage, secrets map[string]string) (Settings, error) {
	var s TelegramSettings
	if err := unmarshalConfig(config, &s); err != nil {
		return nil, err
	}
	s.BotToken = secrets[tgSecretKeyBotToken]
	return s, nil
}

func (telegram) Validate(settings Settings) error {
	s, ok := settings.(TelegramSettings)
	if !ok {
		return fmt.Errorf("telegram: unexpected settings type %T", settings)
	}
	return validation.ValidateStruct(&s,
		validation.Field(&s.BotToken, validation.Required.Error("bot_token is required")),
		// Syntactic check only: where the URL points is the admin's call (a
		// self-hosted Bot API server is legitimate); this catches typos, not exfiltration.
		validation.Field(&s.APIURL, validation.When(s.APIURL != "", is.URL)),
		validation.Field(&s.Timeout, validation.When(s.Timeout != "", validation.WithContext(xvalidation.IsDuration))),
	)
}
