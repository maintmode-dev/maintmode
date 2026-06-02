package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

// DeferredNotification is a reminder scheduled to fire at FireAt relative to a
// maintenance. It stores only the schedule: recipients are the maintenance's
// notify targets (resolved at send time) and the text is rendered from the
// maint.reminder template, so neither is stored on the reminder.
//
// GoqueTaskID is nil until the reminder is enqueued (on approve); it holds the
// goque task id so the pending task can be canceled.
type DeferredNotification struct {
	ID          uuid.UUID
	MaintID     uuid.UUID
	FireAt      time.Time
	GoqueTaskID *uuid.UUID
}

// IsScheduled returns true if the reminder is scheduled to fire.
func (n *DeferredNotification) IsScheduled() bool {
	return n.GoqueTaskID != nil && lo.FromPtr(n.GoqueTaskID) != uuid.Nil
}
