package integration

import (
	"context"

	"github.com/ruko1202/maintmode/internal/audit"
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
	// onChange is notified (synchronously, post-commit) with the kind of every
	// mutated integration. Bootstrap wires it to the transport resolver's cache
	// invalidation; nil means nobody is listening. This is the registry's ONLY
	// link toward the delivery side, and it is an outgoing callback — the
	// registry itself imports nothing transport-related.
	onChange func(kind string)
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

// SetOnChange registers the post-commit change listener (see Service.onChange).
// Called once from bootstrap after the delivery-side resolver is built; not
// safe for concurrent use with in-flight mutations.
func (s *Service) SetOnChange(fn func(kind string)) { s.onChange = fn }

// notifyChanged tells the listener (if any) that kind's stored state changed.
// Deliberately after commit, not inside the tx: invalidating inside the tx
// could evict-then-repopulate from another connection's pre-commit read.
// Cross-replica correctness rests on the resolver's cache TTL, not on this call.
func (s *Service) notifyChanged(kind string) {
	if s.onChange != nil {
		s.onChange(kind)
	}
}
