package maint

import (
	"context"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/services/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/services/maintnotify"
	"github.com/ruko1202/maintmode/internal/services/notifytargets"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// UserLister is the subset of the auth user service the maint service depends
// on for approver-eligibility checks. Declared consumer-side so tests can
// substitute a mock; satisfied by *user.Service. The "eligible approver"
// composition (role mapping, active-only filter, non-empty check) lives in
// validate_approver.go — the user service knows nothing about "approver".
type UserLister interface {
	ListUsers(ctx context.Context, cmd *entity.ListUsersCmd) (*entity.ListUsersResult, error)
}

// AuditPublisher enqueues an audited action to the durable outbox.
// Defined consumer-side so the maint service depends only on the publish
// capability and can be tested with a fake. Backed by auditpublisher.Publisher;
// the audit.write task it enqueues is drained on the auth binary.
type AuditPublisher interface {
	Publish(ctx context.Context, action audit.Action) error
}

type Service struct {
	txManager      *dbtx.TxManager
	maintStore     *maintenances.Store
	resourcesStore *resources.Store

	notifyTargets  *notifytargets.Service
	conflictsSrv   *conflicts.Service
	notifier       *maintnotify.Service
	deferred       *deferrednotifications.Service
	users          UserLister
	auditPublisher AuditPublisher
}

func NewService(
	txManager *dbtx.TxManager,
	maintStore *maintenances.Store,
	resourcesStore *resources.Store,
	notifyTargets *notifytargets.Service,
	conflictsSrv *conflicts.Service,
	notifier *maintnotify.Service,
	deferred *deferrednotifications.Service,
	users UserLister,
	auditPublisher AuditPublisher,
) *Service {
	return &Service{
		txManager:      txManager,
		maintStore:     maintStore,
		resourcesStore: resourcesStore,
		notifyTargets:  notifyTargets,
		conflictsSrv:   conflictsSrv,
		notifier:       notifier,
		deferred:       deferred,
		users:          users,
		auditPublisher: auditPublisher,
	}
}
