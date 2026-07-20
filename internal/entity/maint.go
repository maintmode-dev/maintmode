package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

type MaintenanceStatus string

const (
	MaintenanceStatusDraft      MaintenanceStatus = "draft"
	MaintenanceStatusPlanned    MaintenanceStatus = "planned"
	MaintenanceStatusInProgress MaintenanceStatus = "in_progress"
	MaintenanceStatusCancelled  MaintenanceStatus = "canceled"
	MaintenanceStatusCompleted  MaintenanceStatus = "completed"
)

var allowedMaintStatusTransitions = map[MaintenanceStatus]map[MaintenanceStatus]struct{}{
	MaintenanceStatusDraft: {
		MaintenanceStatusPlanned:   {}, // approve
		MaintenanceStatusCancelled: {}, // cancel
	},

	MaintenanceStatusPlanned: {
		MaintenanceStatusInProgress: {}, // start
		MaintenanceStatusCancelled:  {}, // cancel
	},

	MaintenanceStatusInProgress: {
		MaintenanceStatusCompleted: {}, // complete
		MaintenanceStatusCancelled: {}, // cancel
	},

	// final states
	MaintenanceStatusCancelled: {},
	MaintenanceStatusCompleted: {},
}

func CanMaintTransition(from, to MaintenanceStatus) bool {
	_, ok := allowedMaintStatusTransitions[from][to]
	return ok
}

type MaintenanceScope string

const (
	MaintenanceScopeGlobal    MaintenanceScope = "global"
	MaintenanceScopeResources MaintenanceScope = "resource"
)

type MaintenanceImpact string

const (
	MaintenanceImpactNone    MaintenanceImpact = "none"
	MaintenanceImpactPartial MaintenanceImpact = "partial_outage"
	MaintenanceImpactFull    MaintenanceImpact = "full_outage"
)

type MaintenanceCancelReason string

const (
	MaintenanceCancelReasonConflict         MaintenanceCancelReason = "conflict"
	MaintenanceCancelReasonIncident         MaintenanceCancelReason = "incident"
	MaintenanceCancelReasonBusinessDecision MaintenanceCancelReason = "business_decision"
	MaintenanceCancelReasonRescheduled      MaintenanceCancelReason = "rescheduled"
	MaintenanceCancelReasonMistake          MaintenanceCancelReason = "mistake"
	// MaintenanceCancelReasonNotStarted is set by the automatic overdue-cancel
	// job when a planned maintenance is not started within the grace window after
	// its planned start. It is system-only: it is never accepted on the public
	// cancel endpoint (see FromAPIMaintenanceCancelReason), only surfaced on read.
	MaintenanceCancelReasonNotStarted MaintenanceCancelReason = "not_started"
)

// Maintenance is a planned work window over one or more resources (or the whole
// system when Scope is global).
//
// Conflict semantics:
//
// Two maintenances over the same resource_id in overlapping time always
// conflict, regardless of Impact (none / partial_outage / full_outage). A
// maintenance is a coordination tool — even non-blocking work (read-only
// vacuum, monitoring exporter rollout) should surface as a conflict so on-call
// sees both windows at once and the teams can coordinate.
//
// If a real product case for "co-existing maintenances on the same resource"
// appears (e.g., explicitly read-only maintenance must run in parallel with a
// deploy), introduce a dedicated flag — do not overload Impact for that.
type Maintenance struct {
	ID                  uuid.UUID
	Title               string
	Description         string
	PlannedPeriod       Period
	ActualPeriod        *Period
	Resources           []uuid.UUID
	Scope               MaintenanceScope
	Impact              MaintenanceImpact
	Status              MaintenanceStatus
	CancelReason        MaintenanceCancelReason
	CancelReasonComment string
	// CreatedByUserID is the author of the maintenance: the authenticated user
	// who created the draft. Always set — the create path requires an
	// authenticated user and the column is NOT NULL.
	CreatedByUserID       uuid.UUID
	CreatedAt             time.Time
	UpdatedAt             *time.Time
	Steps                 []*MaintenanceStep
	NotifyTargets         []*NotifyTarget
	DeferredNotifications []*DeferredNotification
	ApproverUserID        uuid.UUID
}

// Normalize enforces the Scope ↔ Resources invariant: a global-scoped
// maintenance has no associated resources. Idempotent — safe to call multiple
// times. Callers that mutate Scope or Resources should invoke Normalize before
// persisting.
func (m *Maintenance) Normalize() {
	if m.Scope == MaintenanceScopeGlobal {
		m.Resources = nil
	}
}

// CloseActualPeriod closes the actual period at end. The DB constraint requires
// actual_period to be NULL or strictly ordered (lower < upper), and the shared
// database can hold drifted rows whose recorded actual start is still in the
// future — closing such a period naively would write an inverted range and fail
// the whole update. A maintenance whose actual work has not begun by end never
// actually ran, so its actual period is dropped instead.
func (m *Maintenance) CloseActualPeriod(end time.Time) {
	if m.ActualPeriod == nil {
		return
	}
	if !m.ActualPeriod.Start.Before(end) {
		m.ActualPeriod = nil
		return
	}
	m.ActualPeriod.End = lo.ToPtr(end)
}

func (m *Maintenance) Revision() int64 {
	if m.UpdatedAt != nil {
		return m.UpdatedAt.UnixMicro()
	}
	return m.CreatedAt.UnixMicro()
}

func (m *Maintenance) Clone() *Maintenance {
	return lo.ToPtr(lo.FromPtr(m))
}
