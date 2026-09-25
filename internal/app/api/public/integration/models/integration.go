// Package apimodels holds the request/response DTOs for the integration registry
// admin API. The response never carries secret values: secrets are exposed only
// as a key->is-set map, and the plaintext/ciphertext stays server-side.
package apimodels

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// UserSummary is the privacy-safe author/editor view, mirroring the shape used
// across the other admin APIs. Nil serializes as null.
type UserSummary struct {
	ID          uuid.UUID `json:"id" format:"uuid"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

// Integration is the read-safe API view. Config is the non-secret settings shown
// verbatim; SecretsSet reports which secret keys are configured without ever
// exposing their values.
type Integration struct {
	ID uuid.UUID `json:"id" format:"uuid"`
	// Kind is the CATEGORY the row belongs to: "notify" or "login".
	Kind string `json:"kind" example:"notify"`
	// Name is the SYSTEM it connects to: slack, telegram, email, google or
	// custom. Together with Kind it is the row's identity AND the path that
	// addresses it: clients build /api/v1/integrations/{kind}/{name} from these
	// two fields rather than assembling an identifier of their own.
	Name    string          `json:"name" example:"slack"`
	Enabled bool            `json:"enabled"`
	Config  json.RawMessage `json:"config" swaggertype:"object"`
	// Health is present for login providers only, and reports whether the
	// provider can actually be used: "ok", "unresolved" (discovery has not
	// answered, so it is listed but refuses sign-in), "disabled", or
	// "unreadable" (its stored secret will not decrypt).
	//
	// The sign-in page deliberately does NOT use this -- it reports what is
	// configured, not what is reachable, and probing every IdP to render a login
	// form would be a self-inflicted outage. This is the admin-facing answer to
	// "I saved it, why does it not work".
	Health     string          `json:"health,omitempty" example:"ok"`
	SecretsSet map[string]bool `json:"secrets_set"`
	CreatedAt  time.Time       `json:"created_at" format:"date-time"`
	CreatedBy  *UserSummary    `json:"created_by"`
	UpdatedAt  time.Time       `json:"updated_at" format:"date-time"`
	UpdatedBy  *UserSummary    `json:"updated_by"`
}

// ListIntegrationsResponse is the list envelope.
type ListIntegrationsResponse struct {
	Integrations []*Integration `json:"integrations"`
}

// CreateIntegrationRequest creates an integration. Secrets are the plaintext
// secret values keyed by the system's secret keys; the server encrypts them
// before persisting and never echoes them back.
//
// (Kind, Name) must be one of the pairs the registry knows -- (notify, slack),
// (notify, telegram), (notify, email), (login, google), (login, custom).
// Anything else is refused: the name decides which implementation parses the
// row, so it is a closed set rather than free text.
//
// For a preset system the server supplies what it already knows -- Google's
// issuer and display name come from the deployment's catalog -- and sending
// either of those fields is REFUSED rather than ignored. "custom" is the one
// entry where the operator supplies everything, issuer included.
type CreateIntegrationRequest struct {
	// Kind is the category: "notify" or "login".
	Kind string `json:"kind" example:"notify"`
	// Name is the system. REQUIRED. For a login provider it is the identity its
	// users authenticate against, so it cannot be changed afterwards.
	Name    string          `json:"name" example:"slack"`
	Enabled *bool           `json:"enabled"`
	Config  json.RawMessage `json:"config" swaggertype:"object"`
	Secrets json.RawMessage `json:"secrets" swaggertype:"object"`
}

// UpdateIntegrationRequest patches an integration: every omitted field keeps
// its stored value (PATCH semantics). enabled: omitted keeps the current flag.
// config: omitted keeps the stored config; an explicit object (including {})
// replaces it wholesale. secrets, per key: omitted keeps the stored value, a
// non-empty string replaces it, null clears it.
// Name is deliberately absent: the instance name is immutable, because it is an
// input to the AAD of a login provider's client_secret and renaming would make
// that secret undecryptable. Linked accounts survive a rename -- they reference
// the row by ID -- so they are no longer what forbids it.
type UpdateIntegrationRequest struct {
	// Omitted → keep the current flag; true/false → set it.
	Enabled *bool `json:"enabled"`
	// Omitted → keep the stored config; an explicit object (including {})
	// replaces it wholesale.
	Config json.RawMessage `json:"config" swaggertype:"object"`
	// Per-key intent: key absent → keep the stored secret; non-empty string →
	// replace it; null → clear it. Values are write-only and never returned by
	// any read endpoint (see Integration.secrets_set).
	Secrets json.RawMessage `json:"secrets" swaggertype:"object"`
}

// TestIntegrationRequest asks the server to try the given settings for real and
// report what happened. Nothing is saved and nothing about the outcome is
// recorded: this tests what the admin is looking at, which may never have been
// stored.
//
// Secrets is a plain map here, NOT the three-state json.RawMessage the create
// and update requests use. There is no stored row to merge with, so an omitted
// key means "no such secret", never "keep the saved one" -- a server that
// paired a stored password with a caller-named host would be handing the
// credential to whatever server the request named. A write-only secret
// therefore has to be re-entered to be tested.
type TestIntegrationRequest struct {
	// Config is the kind's non-secret settings, same shape as on create/update.
	Config json.RawMessage `json:"config" swaggertype:"object"`
	// Secrets are plaintext secret values keyed by the kind's secret keys. A key
	// the kind does not declare is an error rather than being ignored: silently
	// dropping it would test an anonymous relay and report success.
	Secrets map[string]string `json:"secrets"`
	// To is the address the test message is sent to. Required -- the server does
	// not infer a recipient from the caller's token.
	To string `json:"to"`
}

// ToggleIntegrationRequest flips the enabled flag.
type ToggleIntegrationRequest struct {
	Enabled *bool `json:"enabled"`
}
