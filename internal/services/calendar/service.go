package calendar

import (
	"context"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
)

// NotifyTargetsReader reads notify targets of a maintenance. Defined
// consumer-side (like maint.ApproverValidator) so calendar depends only on the
// read capability, not on the notificator module's storage internals — that
// cross-module storage import is forbidden by the depguard storage-fortress rule
// (RUK-193). Backed by notifytargets.Store.
type NotifyTargetsReader interface {
	ListByMaint(ctx context.Context, maintID uuid.UUID) ([]*entity.NotifyTarget, error)
}

type Service struct {
	maintStore     *maintenances.Store
	resourcesStore *resources.Store
	notifyTargets  NotifyTargetsReader
	conflictsSrv   *conflicts.Service
}

func NewService(
	maintStore *maintenances.Store,
	resourcesStore *resources.Store,
	notifyTargets NotifyTargetsReader,
	conflictsSrv *conflicts.Service,
) *Service {
	return &Service{
		maintStore:     maintStore,
		resourcesStore: resourcesStore,
		notifyTargets:  notifyTargets,
		conflictsSrv:   conflictsSrv,
	}
}
