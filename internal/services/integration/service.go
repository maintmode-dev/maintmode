package integration

import (
	"context"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/config"
	datakeystore "github.com/ruko1202/maintmode/internal/storages/datakey"
	integrationstore "github.com/ruko1202/maintmode/internal/storages/integration"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// AuditPublisher enqueues an audited action to the durable outbox. Defined
// consumer-side so the service depends only on the publish capability and can be
// faked in tests; backed by auditpublisher.Publisher.
type AuditPublisher interface {
	Publish(ctx context.Context, action audit.Action) error
}

// IdentitiesStore removes the accounts that authenticate through a provider.
// Declared consumer-side, and it has to be: the module boundaries forbid this
// module from importing the auth storages directly, so bootstrap injects the
// concrete store behind this one method.
//
// The argument is the registry row's ID. It used to be the instance NAME, and
// the change is the point of RUK-303 rather than a refactor: a name can be
// deleted and created again against a different IdP, so identities addressed by
// one could outlive the provider they were written for. An id cannot be reused.
type IdentitiesStore interface {
	// DeleteByIntegrationID removes every identity authenticating through one
	// registry row, returning how many went. Used by the delete cascade.
	DeleteByIntegrationID(ctx context.Context, integrationID uuid.UUID) (int64, error)
}

// cipher is the subset of secrets.SecretCipher the service uses to seal/open
// secret values under a DEK.
type cipher interface {
	Encrypt(dek, plaintext, aad []byte) ([]byte, error)
	Decrypt(dek, envelope, aad []byte) ([]byte, error)
}

// keyring wraps/unwraps the DEK that protects a kind's secrets.
type keyring interface {
	WrapDEK(dek []byte) (wrapped []byte, kekID string, err error)
	UnwrapDEK(wrapped []byte, kekID string) ([]byte, error)
}

// Service owns integration CRUD: it validates per-kind, encrypts secrets on write
// with a DEK wrapped by the active KEK, masks secrets on read, and keeps every
// mutation transactional. Secrets are decrypted only transiently, deep inside the
// service; plaintext never reaches a read/response path.
type Service struct {
	txManager      *dbtx.TxManager
	store          *integrationstore.Store
	dekStore       *datakeystore.Store
	registry       *Registry
	keyring        keyring
	cipher         cipher
	auditPublisher AuditPublisher
	// identities removes the accounts linked to a provider when it is deleted,
	// addressed by the registry row's id. Under ON DELETE RESTRICT the cascade
	// is load-bearing: skipping it makes the delete fail on the children it
	// left behind.
	identities IdentitiesStore
	// loginProviders is the config file's provider sections. Their facts are
	// the catalog an API create copies into a row (see presets.go: with no
	// entry, only `custom` stays creatable); the managed_by: config entries are
	// the providers Provision validates, writes and serves -- ONE source for all
	// three, so they cannot drift.
	loginProviders config.LoginProviders
	// provisioned is the managed_by: config entries after validation, keyed by
	// name: what openProvider serves for a provisioned row, secret included --
	// the secret is not in the database, so this is the only place it lives.
	// Set once by Provision at startup, before the reloader's first build reads
	// it, and never written again -- which is why it needs no lock.
	provisioned map[string]provisionedProvider
	// onChange are notified (synchronously, post-commit) with the identity of
	// every mutated integration. Bootstrap wires the transport resolver's cache
	// invalidation and the login reloader; an empty list means nobody is
	// listening. These are the registry's ONLY links outward, and they are
	// outgoing callbacks — the registry imports nothing from either consumer.
	//
	// The name travels with the kind because a kind alone stopped identifying a
	// row: a listener serving several instances of one kind cannot otherwise
	// tell which one changed.
	onChange []func(kind, name string)
}

// NewService builds the integration registry service.
//
// loginProviders may be empty, and that is a valid catalog rather than a missing
// one: only `custom` can then be created, and every preset-backed name is
// refused (see presetAndConfig).
func NewService(
	txManager *dbtx.TxManager,
	store *integrationstore.Store,
	dekStore *datakeystore.Store,
	registry *Registry,
	kr keyring,
	c cipher,
	auditPublisher AuditPublisher,
	identities IdentitiesStore,
	loginProviders config.LoginProviders,
) *Service {
	return &Service{
		txManager:      txManager,
		store:          store,
		dekStore:       dekStore,
		registry:       registry,
		keyring:        kr,
		cipher:         c,
		auditPublisher: auditPublisher,
		identities:     identities,
		// The managed entries are validated and applied by Provision, not
		// here: validation needs the registry's rules and the rows need a
		// transaction.
		loginProviders: loginProviders,
	}
}

// AddOnChange registers a post-commit change listener (see Service.onChange).
//
// Called from bootstrap while wiring, before the service serves anything; not
// safe for concurrent use with in-flight mutations. A list rather than a single
// slot because there are now two consumers, and the second one -- the login
// reloader -- must not be installable only where the first happens to be.
func (s *Service) AddOnChange(fn func(kind, name string)) { s.onChange = append(s.onChange, fn) }

// notifyChanged tells the listener (if any) that kind's stored state changed.
// Deliberately after commit, not inside the tx: invalidating inside the tx
// could evict-then-repopulate from another connection's pre-commit read.
// Cross-replica correctness rests on the resolver's cache TTL, not on this call.
func (s *Service) notifyChanged(kind, name string) {
	for _, fn := range s.onChange {
		fn(kind, name)
	}
}
