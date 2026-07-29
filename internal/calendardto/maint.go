package calendardto

import (
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

type MaintenanceResource struct {
	ID   uuid.UUID
	Name string
}

// MaintenanceNotifyTarget is one notify channel of the maintenance view: the
// catalog channel's id + name with its transport, resolved from the channel
// catalog (targets reference channels by FK).
type MaintenanceNotifyTarget struct {
	ID        uuid.UUID
	Name      string
	Transport entity.NotifyTransport
}

// MaintenanceDeferredNotification is one scheduled reminder of the maintenance
// view: when it fires and whether it has been enqueued yet (false until the
// maintenance is approved). The goque task id itself stays in the domain — the
// view only needs the resolved flag.
type MaintenanceDeferredNotification struct {
	ID        uuid.UUID
	FireAt    time.Time
	Scheduled bool
}

type MaintenanceStep struct {
	ID                  uuid.UUID
	Order               int32
	Description         string
	RollbackDescription string
	DurationMinutes     int64
	Status              entity.MaintenanceStepStatus
}

type Maintenance struct {
	ID                    uuid.UUID
	Title                 string
	Description           string
	PlannedPeriod         entity.Period
	ActualPeriod          *entity.Period
	Resources             []*MaintenanceResource
	Scope                 entity.MaintenanceScope
	Impact                entity.MaintenanceImpact
	Status                entity.MaintenanceStatus
	CancelReason          entity.MaintenanceCancelReason
	CancelReasonComment   string
	CreatedAt             time.Time
	UpdatedAt             *time.Time
	Revision              int64
	CreatedByUserID       uuid.UUID
	ApproverUserID        uuid.UUID
	Steps                 []*MaintenanceStep
	NotifyTargets         []*MaintenanceNotifyTarget
	DeferredNotifications []*MaintenanceDeferredNotification
	// Mentions holds the raw ids of the mentioned users. Names are resolved in
	// the API handler, which already batches a user-service call; the calendar
	// service deliberately keeps no resolver of its own.
	Mentions []uuid.UUID
}

type MaintenancesMeta struct {
	Count     int64
	Truncated bool
}
