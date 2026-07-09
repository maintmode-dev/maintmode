package integrationkinds

import (
	"encoding/json"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/utils/xvalidation"
)

// TLS policy vocabulary of the email config contract. The kind owns the
// admin-facing vocabulary; the transport maps these strings (and safely
// defaults anything unknown to mandatory STARTTLS). String equality with the
// transport's mapping is pinned by a test in services/transportresolver — the
// one module that legitimately sees both halves.
const (
	TLSPolicyNone          = "none"
	TLSPolicyOpportunistic = "opportunistic"
	TLSPolicyMandatory     = "mandatory"
)

const (
	// kindEmail is derived from the transport constant (see kindSlack).
	kindEmail              = string(entity.NotifyTransportEmail)
	emailSecretKeyPassword = "password"
)

// EmailSettings is the parsed configuration for the email/SMTP integration.
// Config fields are populated by json.Unmarshal; Password is merged in from the
// decrypted secrets (json:"-"). Timeout is a string parsed at build time. The
// type intentionally has no Stringer/marshaler.
type EmailSettings struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"-"`
	From      string `json:"from"`
	ReplyTo   string `json:"reply_to"`
	TLSPolicy string `json:"tls_policy"`
	Timeout   string `json:"timeout"`
}

// Kind marks EmailSettings as the "email" settings value (integrationkinds.Settings).
func (EmailSettings) Kind() string { return kindEmail }

// Email implements the integrationkinds.Integration contract for kind "email". The kind matches
// entity.NotifyTransportEmail so the runtime resolver can find it by the
// channel's transport ("email"); the spec's "smtp" wording refers to the
// underlying protocol, not the registry key.
type email struct{}

func (email) Kind() string { return kindEmail }

func (email) SecretKeys() []string { return []string{emailSecretKeyPassword} }

func (email) Parse(config json.RawMessage, secrets map[string]string) (Settings, error) {
	var s EmailSettings
	if err := unmarshalConfig(config, &s); err != nil {
		return nil, err
	}
	// The password is only meaningful with a username (SMTP auth is opt-in); a
	// missing password is fine for an unauthenticated relay, so it is merged as-is
	// here and enforced against username in Validate.
	s.Password = secrets[emailSecretKeyPassword]
	return s, nil
}

func (email) Validate(settings Settings) error {
	s, ok := settings.(EmailSettings)
	if !ok {
		return fmt.Errorf("email: unexpected settings type %T", settings)
	}

	return validation.ValidateStruct(&s,
		validation.Field(&s.Host, validation.Required),
		validation.Field(&s.From, validation.Required),
		// Port is optional: the transport defaults it (587). SMTP auth is opt-in —
		// an unauthenticated relay is valid (this is also what lets an update CLEAR
		// the password) — but a half credential is a config mistake: username and
		// password only together.
		validation.Field(&s.Username,
			validation.Required.When(s.Password != "").Error("username and password must be set together")),
		validation.Field(&s.Password,
			validation.Required.When(s.Username != "").Error("username and password must be set together")),
		// The vocabulary is the transport's (email/client.go tlsPolicy): empty is
		// allowed and defaults to mandatory STARTTLS there.
		validation.Field(&s.TLSPolicy, validation.In(
			TLSPolicyNone,
			TLSPolicyOpportunistic,
			TLSPolicyMandatory,
		)),
		validation.Field(&s.Timeout, validation.When(s.Timeout != "", validation.WithContext(xvalidation.IsDuration))),
	)
}
