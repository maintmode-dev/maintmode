package calendardto

import (
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

type GetMaintsFilter struct {
	PeriodFrom  time.Time
	PeriodTo    time.Time
	Statuses    []entity.MaintenanceStatus
	ResourceIDs []uuid.UUID
	ChannelIDs  []uuid.UUID
}

// ConflictQueryCmd asks for the conflicts of one maintenance. It carries both
// periods and the status because the answer depends on where the maintenance is
// in its lifecycle: a live one is compared by plan, a finished one by fact. See
// calendar.resolveConflicts for the mapping.
//
// It deliberately diverges from entity.ConflictQueryCmd rather than aliasing
// it: that type is also what the approve gate passes to the live query, so
// keeping it untouched keeps the gate out of this read path.
//
// ActualPeriod is nil for a maintenance that never started. That is the signal,
// not an omission: it routes the read to the approval snapshot.
type ConflictQueryCmd struct {
	MaintID       uuid.UUID
	Status        entity.MaintenanceStatus
	PlannedPeriod entity.Period
	ActualPeriod  *entity.Period
	Scope         entity.MaintenanceScope
	ResourceIDs   []uuid.UUID
}

// PendingApprovalsFilter selects the draft maintenances awaiting one specific
// approver. ApproverUserID is mandatory: a zero value is an internal invariant
// violation, not "no filter" — see the store's guard.
type PendingApprovalsFilter struct {
	ApproverUserID uuid.UUID
	Limit          int64
	Offset         int64
}

// PendingApprovalsResult carries one page plus the total count matching the
// same filter. Maintenances are already mapped to the DTO layer by the service;
// the store returns entities.
type PendingApprovalsResult struct {
	Maintenances []*Maintenance
	Total        int64
}
