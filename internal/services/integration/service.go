package integration

import (
	"context"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
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

// IdentitiesStore removes the accounts that authenticate through a provider
// name. Declared consumer-side, and it has to be: the module boundaries forbid
// this module from importing the auth storages directly, so bootstrap injects
// the concrete store behind this one method.
//
// One method, and it used to be two: a count sat beside it, feeding guards that
// refused creating or deleting a name identities carried. The closed set
// retired them -- there are two login names, one of which cannot be aimed
// anywhere and the other of which is an admin's to re-point -- and what remains
// is the cascade.
//
// The argument is the instance NAME (google, keycloak), never the category --
// user_identities.provider holds names, so passing a category would delete
// nothing, or, worse, another provider's rows.
type IdentitiesStore interface {
	// DeleteByProvider removes every identity on a provider name, returning how
	// many went. Used by the delete cascade.
	DeleteByProvider(ctx context.Context, provider entity.AuthMethod) (int64, error)
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
	// identities counts accounts linked to a provider name. Nil means no auth
	// storage on this binary, and therefore no identities to protect, so the
	// guard passes rather than refusing everything.
	identities IdentitiesStore
	// loginPresets is the credential-free catalog of well-known providers,
	// copied into a row at create. Nil means every login name behaves like
	// "custom" -- a preset only ever removes fields an operator must supply.
	loginPresets config.LoginPresets
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

func NewService(
	txManager *dbtx.TxManager,
	store *integrationstore.Store,
	dekStore *datakeystore.Store,
	registry *Registry,
	kr keyring,
	c cipher,
	auditPublisher AuditPublisher,
) *Service {
	return &Service{
		txManager:      txManager,
		store:          store,
		dekStore:       dekStore,
		registry:       registry,
		keyring:        kr,
		cipher:         c,
		auditPublisher: auditPublisher,
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

// WithIdentities wires the linked-account guard. A setter rather than a
// constructor argument for the same reason WithDance is one: the auth storages
// are built after this service, and threading them through the constructor
// would invert that order.
func (s *Service) WithIdentities(identities IdentitiesStore) *Service {
	s.identities = identities

	return s
}
