// Package authsettings owns which built-in sign-in methods an instance offers.
//
// Its whole job is one flag per method. Login providers are not here: they live
// in the integration registry with their own enabled flag, and a second switch
// over one provider would mean every read had to answer which of them wins.
//
// There is deliberately no guard against turning every method off. An instance
// that offers no built-in sign-in is a configuration an admin may legitimately
// want -- it is what running purely on corporate SSO looks like -- and the same
// state is reachable unguarded through the registry anyway, by disabling the
// last provider. It is also not a lockout: disabling a method does not end
// existing sessions, so the admin who did it keeps the session they did it
// from, and break-glass is there if that session is gone too.
package authsettings

import (
	"context"

	"github.com/ruko1202/maintmode/internal/audit"
	authsettingsstore "github.com/ruko1202/maintmode/internal/storages/authsettings"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// AuditPublisher enqueues an audited action to the durable outbox. Declared
// consumer-side so the service depends only on the publish capability; backed
// by auditpublisher.Publisher.
type AuditPublisher interface {
	Publish(ctx context.Context, action audit.Action) error
}

// Service reads and writes the built-in method flags.
type Service struct {
	txManager      *dbtx.TxManager
	store          *authsettingsstore.Store
	auditPublisher AuditPublisher
}

func NewService(
	txManager *dbtx.TxManager,
	store *authsettingsstore.Store,
	auditPublisher AuditPublisher,
) *Service {
	return &Service{
		txManager:      txManager,
		store:          store,
		auditPublisher: auditPublisher,
	}
}
