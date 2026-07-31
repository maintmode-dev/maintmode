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

type ConflictQueryCmd entity.ConflictQueryCmd

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
