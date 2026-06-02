package entity

import (
	"time"

	"github.com/google/uuid"
)

// NotifyEventKind identifies a lifecycle transition. Strings are stable: they
// appear in config (routes) and in idempotency keys.
type NotifyEventKind string

const (
	NotifyEventMaintStarted   NotifyEventKind = "maint.started"
	NotifyEventMaintCompleted NotifyEventKind = "maint.completed"
	NotifyEventMaintCancelled NotifyEventKind = "maint.cancelled"
	NotifyEventMaintReminder  NotifyEventKind = "maint.reminder"
	NotifyEventStepStarted    NotifyEventKind = "step.started"
	NotifyEventStepCompleted  NotifyEventKind = "step.completed"
	NotifyEventStepCancelled  NotifyEventKind = "step.cancelled"
)

var eventNames = map[NotifyEventKind]string{
	NotifyEventMaintStarted:   "Maintenance started",
	NotifyEventMaintCompleted: "Maintenance completed",
	NotifyEventMaintCancelled: "Maintenance canceled",
	NotifyEventMaintReminder:  "Maintenance reminder",
	NotifyEventStepStarted:    "Step started",
	NotifyEventStepCompleted:  "Step completed",
	NotifyEventStepCancelled:  "Step canceled",
}

func (k NotifyEventKind) Subject() string {
	n, ok := eventNames[k]
	if !ok {
		return string(k)
	}

	return n
}

func (k NotifyEventKind) IsValid() bool {
	switch k {
	case NotifyEventMaintStarted,
		NotifyEventMaintCompleted,
		NotifyEventMaintCancelled,
		NotifyEventMaintReminder,
		NotifyEventStepStarted,
		NotifyEventStepCompleted,
		NotifyEventStepCancelled:
		return true
	default:
		return false
	}
}

func (k NotifyEventKind) IsStep() bool {
	switch k {
	case NotifyEventStepStarted, NotifyEventStepCompleted, NotifyEventStepCancelled:
		return true
	default:
		return false
	}
}

// NotifyEvent is the input to the template renderer
// Fields are flattened primitives, so the renderer doesn't import entity.
type NotifyEvent struct {
	Kind        NotifyEventKind
	OccurredAt  time.Time
	FrontendURL string

	MaintID      uuid.UUID
	MaintTitle   string
	PlannedStart time.Time

	StepID          uuid.UUID
	StepOrder       int32
	StepDescription string

	CancelReason        MaintenanceCancelReason
	CancelReasonComment string
}

type NotifyTarget struct {
	ID        uuid.UUID
	MaintID   uuid.UUID
	Transport NotifyTransport
	ChannelID string
	CreatedAt time.Time
}

// NotifyMessage is a rendered notification ready for delivery.
type NotifyMessage struct {
	Subject string
	Body    string
}
