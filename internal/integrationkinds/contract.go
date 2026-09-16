package integrationkinds

import "encoding/json"

// Settings is a kind's parsed, decrypted configuration — the value that flows
// from the integration registry (which produces it via Parse) to the transport
// resolver (whose per-kind builder asserts it back to the kind's concrete
// type). Only the same kind consumes what it produced, so the value is opaque
// to everything in between.
//
// It carries no methods. It used to declare Kind() string, mirroring
// Integration.Kind on the settings value, and that method had no caller
// anywhere -- not in the code, not in a test. Two methods answering "which kind
// is this?" could only agree redundantly or disagree silently, and the registry
// keys rows by what Integration reports, so the copy on the settings value was
// the one with nothing keeping it honest.
//
// Deliberately NOT closed with an unexported marker method. Closing it would
// stop a test in another package from writing a fake settings type, and two
// packages do exactly that to cover the AAD binding and the transport seam --
// coverage of the cryptographic path is worth more than a compile-time lid over
// a seam whose two ends are written together.
type Settings interface{}

// Preseted is implemented by a login entry whose well-known values come from
// the deployment's catalog rather than from the operator.
//
// It is a separate, optional interface for the same reason ClientBound is: the
// question is about one entry, not about every integration, and asking it by
// type assertion keeps the answer with the entry that knows it. `custom` does
// not implement it; `google` does. Nothing else in the codebase then has to
// carry a list of which login names are preset-backed -- a list that would have
// to be kept in step with the registry by hand.
type Preseted interface {
	// PresetKey is the catalog key this entry's defaults are filed under.
	PresetKey() string
}

// ClientBound is implemented by kinds whose secrets are issued by an external
// OAuth client and must be bound to it cryptographically — the login providers.
// A kind that does not implement it gets the plain (kind, key) AAD.
//
// It is implemented by a kind's SETTINGS, not by the kind, and the distinction
// is the whole point: AADBinding returns the issuer and client id of ONE ROW,
// which only a parsed value can supply. It answers "what is this row bound to",
// never "which category is this kind" -- Integration.Category answers that, and
// asking this interface instead meant parsing an empty config purely to inspect
// the result's type.
//
// Optional rather than part of Integration for the reason that keeps that
// contract small: every non-login kind would otherwise carry a method returning
// nothing. Type-asserting for it is how the secret path chooses the AAD, so a
// new login kind opts in by implementing it and cannot silently get the weaker
// binding.
type ClientBound interface {
	// AADBinding returns the identifiers a secret of this kind is bound to:
	// the OAuth issuer URL (empty for kinds with no issuer, such as GitHub,
	// whose endpoints are fixed) and the client id.
	//
	// Both are read from the settings the caller already parsed, so the binding
	// always matches the config being written rather than whatever is stored.
	AADBinding() (issuerURL, clientID string)

	// SecurityRelevant returns the fields whose change alters who can sign in
	// or where credentials travel.
	//
	// It is deliberately WIDER than AADBinding. The binding answers "would this
	// edit strand the stored secret"; this answers "would this edit change the
	// security posture of a provider people already authenticate through", and
	// the two are not the same set. A domain restriction dropped to empty
	// admits every account of the provider and touches no AAD input at all.
	//
	// Returned as a comparable string so callers can diff two parsed configs
	// without knowing any kind's field names.
	SecurityRelevant() string
}

// Integration is the per-kind SETTINGS contract: parsing and validating the
// kind's config+secrets. It deliberately knows nothing about delivery — the
// kind's transport builder lives in services/transportresolver (grafana-shape
// split: the registry stores, the resolver builds). A new integration type
// implements this and registers itself — no infrastructure code, no DDL.
type Integration interface {
	// Name is the registry key and the SYSTEM this integration talks to:
	// "slack", "telegram", "email", "google", "custom". It is stored in
	// integration_settings.name, while the .kind column now holds the category
	// ("notify" or "login") that the row belongs to.
	//
	// For delivery integrations this value is unchanged from what Kind()
	// returned before the rename, and it must stay that way: it is an input to
	// every stored secret's AAD (see services/integration/secrets.go) and the
	// key of the transport builder map. Returning the category here would
	// strand every bot_token and SMTP password in the database.
	Name() string
	// Category is the half of the product this integration belongs to:
	// CategoryNotify for one that delivers messages, CategoryLogin for one
	// people sign in through. It is stored in integration_settings.kind, and
	// together with Name it forms the closed set of admissible pairs.
	//
	// Answered by the ENTRY rather than derived from a parsed Settings value.
	// The category is a property of the implementation -- it is the same for
	// every row of a kind, and knowable without a row at all -- so deriving it
	// meant parsing an empty config purely to type-assert the result, which
	// reads as an accident and fails for any kind whose Parse ever stops
	// accepting nil. Two implementations that differ only in their preset
	// (google and custom) answer the same category here, which is correct and
	// says so.
	Category() string
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
