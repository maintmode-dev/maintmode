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
// cross-module storage import is forbidden by the depguard storage-fortress
// rule. Backed by notifytargets.Store.
type NotifyTargetsReader interface {
	ListByMaint(ctx context.Context, maintID uuid.UUID) ([]*entity.NotifyTarget, error)
}

// DeferredNotificationsReader reads the deferred-notification schedule of a
// maintenance. Defined consumer-side (like NotifyTargetsReader) so calendar does
// not import another module's storage directly — that cross-module storage
// import is forbidden by the depguard storage-fortress rule. Backed by
// deferrednotifications.Store.
type DeferredNotificationsReader interface {
	ListByMaint(ctx context.Context, maintID uuid.UUID) ([]*entity.DeferredNotification, error)
}

type Service struct {
	maintStore     *maintenances.Store
	resourcesStore *resources.Store
	notifyTargets  NotifyTargetsReader
	deferred       DeferredNotificationsReader
	conflictsSrv   *conflicts.Service
}

func NewService(
	maintStore *maintenances.Store,
	resourcesStore *resources.Store,
	notifyTargets NotifyTargetsReader,
	deferred DeferredNotificationsReader,
	conflictsSrv *conflicts.Service,
) *Service {
	return &Service{
		maintStore:     maintStore,
		resourcesStore: resourcesStore,
		notifyTargets:  notifyTargets,
		deferred:       deferred,
		conflictsSrv:   conflictsSrv,
	}
}
