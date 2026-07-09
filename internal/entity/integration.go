package entity

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// IntegrationSetting is a stored connection to an external system (Slack,
// Telegram, SMTP, ...), identified by Kind. Config holds non-secret fields as
// plaintext; Secrets holds each secret value as base64(envelope) encrypted with
// the DEK at DEKID. The plaintext secret is never held here — it is decrypted
// only transiently inside the integration service.
type IntegrationSetting struct {
	ID      uuid.UUID
	Kind    string
	Enabled bool
	// Config is the non-secret settings as a raw JSON object, opaque to the
	// service: each kind unmarshals it into its own typed Settings. It is stored
	// and returned verbatim — secrets belong in Secrets, never here.
	Config json.RawMessage
	// Secrets maps a secret key to its base64(envelope) ciphertext as stored —
	// the typed form, because every consumer (mask, merge, decrypt) addresses
	// values per key; (un)marshaling to the jsonb column happens once, in the
	// store mapper. On read paths this is masked; never surfaced as plaintext.
	Secrets map[string]string
	DEKID   uuid.UUID

	CreatedAt       time.Time
	CreatedByUserID *uuid.UUID
	UpdatedAt       time.Time
	UpdatedByUserID *uuid.UUID
}

// CreateIntegrationCmd creates a new integration of a kind. Config carries the
// non-secret fields; Secrets carries plaintext secret values keyed by the kind's
// SecretKeys — the service encrypts them before persisting. CreatedByUserID is
// the actor from the access token.
type CreateIntegrationCmd struct {
	Kind string
	// Enabled is a pointer so the service can tell "explicitly on/off" from
	// "omitted": a nil Enabled is rejected rather than silently defaulting to
	// false, since an integration's on/off state must be a deliberate choice.
	Enabled *bool
	Config  json.RawMessage
	Secrets json.RawMessage
	// Actor is the authenticated user performing the create, resolved at the API
	// boundary. Its id is persisted as created_by; the full user is used only for
	// the audit snapshot (email/name).
	Actor *User
}

// UpdateIntegrationCmd replaces the config and (optionally) secrets of an
// existing integration. Secrets uses a *string value to carry three intents per
// key: key absent -> keep the stored secret (an edit need not resend unchanged
// secrets); a non-empty value -> replace it; a nil or empty value -> clear it
// (e.g. dropping SMTP auth to switch to an open relay). UpdatedByUserID is the
// actor from the access token.
type UpdateIntegrationCmd struct {
	Kind    string
	Enabled *bool
	Config  json.RawMessage
	Secrets json.RawMessage
	Actor   *User
}

// ToggleIntegrationCmd flips the enabled flag of an integration at runtime.
type ToggleIntegrationCmd struct {
	Kind    string
	Enabled *bool
	Actor   *User
}

// MaskedIntegration is the read-safe view of an integration for API/diagnostics.
// Config is shown verbatim (non-secret); SecretsSet maps each secret key to a
// bool "is set" flag — the ciphertext and plaintext both stay out of this view so
// a read can never leak a credential.
type MaskedIntegration struct {
	ID         uuid.UUID
	Kind       string
	Enabled    bool
	Config     json.RawMessage
	SecretsSet map[string]bool

	CreatedAt       time.Time
	CreatedByUserID *uuid.UUID
	UpdatedAt       time.Time
	UpdatedByUserID *uuid.UUID
}

// Mask projects a stored setting to its read-safe view: each key present in the
// stored secrets map becomes SecretsSet[key]=true, dropping the ciphertext.
func (s *IntegrationSetting) Mask() *MaskedIntegration {
	set := make(map[string]bool, len(s.Secrets))
	for k := range s.Secrets {
		set[k] = true
	}
	return &MaskedIntegration{
		ID:              s.ID,
		Kind:            s.Kind,
		Enabled:         s.Enabled,
		Config:          s.Config,
		SecretsSet:      set,
		CreatedAt:       s.CreatedAt,
		CreatedByUserID: s.CreatedByUserID,
		UpdatedAt:       s.UpdatedAt,
		UpdatedByUserID: s.UpdatedByUserID,
	}
}

// PlainSecrets decodes the caller-supplied secrets object into plaintext values
// keyed by the kind's secret keys. An omitted field (nil/empty/JSON null)
// decodes to an empty map — "no secrets supplied", not an error.
func (c *CreateIntegrationCmd) PlainSecrets() (map[string]string, error) {
	return decodeSecretsJSON[string](c.Secrets)
}

// SecretIntents decodes the caller-supplied secrets object preserving the
// three-way intent per key: key absent -> keep the stored secret; a string ->
// replace it; JSON null -> clear it. The *string value is what keeps null
// distinct from absent through the decode.
func (c *UpdateIntegrationCmd) SecretIntents() (map[string]*string, error) {
	return decodeSecretsJSON[*string](c.Secrets)
}

// decodeSecretsJSON decodes a JSON object of secret values. nil, empty, and the
// JSON null literal all decode to an empty map; anything that is not an object
// of the expected value shape is an error for the caller to classify (user
// input -> validation; stored data -> corruption).
func decodeSecretsJSON[V any](raw json.RawMessage) (map[string]V, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]V{}, nil
	}
	var m map[string]V
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("secrets must be a JSON object: %w", err)
	}
	if m == nil {
		m = map[string]V{}
	}
	return m, nil
}
