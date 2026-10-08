package resources

import (
	"context"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// AuditPublisher enqueues an audited action to the durable outbox. Defined
// consumer-side so tests can record what was published; satisfied by
// *auditpublisher.Publisher.
type AuditPublisher interface {
	Publish(ctx context.Context, action audit.Action) error
}

type Service struct {
	txManager      *dbtx.TxManager
	store          *resources.Store
	auditPublisher AuditPublisher
}

func NewService(txManager *dbtx.TxManager, store *resources.Store, auditPublisher AuditPublisher) *Service {
	return &Service{
		txManager:      txManager,
		store:          store,
		auditPublisher: auditPublisher,
	}
}
