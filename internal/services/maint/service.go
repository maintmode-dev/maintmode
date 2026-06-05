package maint

import (
	"context"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/services/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/services/maintnotify"
	"github.com/ruko1202/maintmode/internal/services/notifytargets"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// ApproverValidator checks whether a user is eligible to be assigned as a
// maintenance approver. Defined consumer-side (like usersummary.AuthUsersGateway)
// so tests can substitute a mock; backed by the auth gateway over S2S.
type ApproverValidator interface {
	IsEligibleApprover(ctx context.Context, id uuid.UUID) (bool, error)
}

type Service struct {
	txManager      *dbtx.TxManager
	maintStore     *maintenances.Store
	resourcesStore *resources.Store

	notifyTargets     *notifytargets.Service
	conflictsSrv      *conflicts.Service
	notifier          *maintnotify.Service
	deferred          *deferrednotifications.Service
	approverValidator ApproverValidator
}

func NewService(
	txManager *dbtx.TxManager,
	maintStore *maintenances.Store,
	resourcesStore *resources.Store,
	notifyTargets *notifytargets.Service,
	conflictsSrv *conflicts.Service,
	notifier *maintnotify.Service,
	deferred *deferrednotifications.Service,
	approverValidator ApproverValidator,
) *Service {
	return &Service{
		txManager:         txManager,
		maintStore:        maintStore,
		resourcesStore:    resourcesStore,
		notifyTargets:     notifyTargets,
		conflictsSrv:      conflictsSrv,
		notifier:          notifier,
		deferred:          deferred,
		approverValidator: approverValidator,
	}
}
