package integrationkinds

import "encoding/json"

// Settings is a kind's parsed, decrypted configuration — the typed value that
// flows from the integration registry (which produces it via Parse) to the
// transport resolver (whose per-kind builder asserts it back to the kind's
// concrete type). The interface is a typed lid over that seam: only a kind's
// own settings type can travel through it, never an arbitrary value.
type Settings interface {
	// Kind names the integration kind this settings value belongs to,
	// matching its Integration.Kind.
	Kind() string
}

// Integration is the per-kind SETTINGS contract: parsing and validating the
// kind's config+secrets. It deliberately knows nothing about delivery — the
// kind's transport builder lives in services/transportresolver (grafana-shape
// split: the registry stores, the resolver builds). A new integration type
// implements this and registers itself — no infrastructure code, no DDL.
type Integration interface {
	// Kind is the registry key (e.g. "slack"), matching integration_settings.kind.
	Kind() string
	// SecretKeys lists the config keys whose values are secret and must be
	// encrypted (e.g. "bot_token", "password"). It is the single source of truth
	// for splitting secret fields from plaintext config.
	SecretKeys() []string
	// Parse unmarshals the raw non-secret config JSON and merges the decrypted
	// secrets into the kind's OWN concrete Settings type, or errors if the JSON
	// is malformed. Only the same kind consumes the value (Validate here, its
	// transport builder in services/transportresolver), so the type assertion
	// back is a within-kind contract — the value is opaque to everything
	// between. Semantic validation (required fields, formats) is done in
	// Validate.
	Parse(config json.RawMessage, secrets map[string]string) (Settings, error)
	// Validate checks a parsed Settings for semantic correctness. Secrets must
	// be judged by PRESENCE (empty vs non-empty), never by format/content: during
	// an update's re-validation an unchanged secret is represented by an opaque
	// non-empty sentinel rather than its plaintext (so it need not be decrypted),
	// and a format check would falsely reject it.
	Validate(settings Settings) error
}
