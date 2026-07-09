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
	ID         uuid.UUID       `json:"id" format:"uuid"`
	Kind       string          `json:"kind" example:"slack"`
	Enabled    bool            `json:"enabled"`
	Config     json.RawMessage `json:"config" swaggertype:"object"`
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

// CreateIntegrationRequest creates an integration of a kind. Secrets are the
// plaintext secret values keyed by the kind's secret keys; the server encrypts
// them before persisting and never echoes them back.
type CreateIntegrationRequest struct {
	Kind    string          `json:"kind"`
	Enabled *bool           `json:"enabled"`
	Config  json.RawMessage `json:"config" swaggertype:"object"`
	Secrets json.RawMessage `json:"secrets" swaggertype:"object"`
}

// UpdateIntegrationRequest patches an integration: every omitted field keeps
// its stored value (PATCH semantics). enabled: omitted keeps the current flag.
// config: omitted keeps the stored config; an explicit object (including {})
// replaces it wholesale. secrets, per key: omitted keeps the stored value, a
// non-empty string replaces it, null clears it.
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

// ToggleIntegrationRequest flips the enabled flag.
type ToggleIntegrationRequest struct {
	Enabled *bool `json:"enabled"`
}
