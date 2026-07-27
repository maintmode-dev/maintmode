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

	// CreatedByUserID is the maintenance owner — the person the notification
	// mentions. Maintenance lifecycle paths fill it from the maintenance row
	// (where the column is NOT NULL); step paths deliberately leave it zero, so
	// the zero value is the mechanism that keeps step events out of the owner
	// resolve, not a defensive guard.
	CreatedByUserID uuid.UUID
	// OwnerMention is filled by dispatch, once per event, before any target is
	// contacted; nil means "no owner to mention" (step events). The renderer
	// picks the handle matching the target transport and falls back to Name.
	OwnerMention *UserMention

	StepID          uuid.UUID
	StepOrder       int32
	StepDescription string

	CancelReason        MaintenanceCancelReason
	CancelReasonComment string
}

// NotifyTarget is a maintenance's subscription to a catalog notify channel.
// The row persists only the channel reference (ChannelID, an FK to
// messenger_channels); Transport, TransportChannelID and ChannelName are
// resolved from the catalog on read (store ListByMaint joins it), so the
// delivery address always reflects the channel's current state — editing a
// channel in the catalog redirects notifications of every subscribed
// maintenance. Archiving a channel does not unsubscribe: the subscription
// stays live and keeps delivering.
type NotifyTarget struct {
	ID        uuid.UUID
	MaintID   uuid.UUID
	ChannelID uuid.UUID
	CreatedAt time.Time

	// Resolved from the catalog on read paths; zero-valued on instances that
	// only carry the persisted columns (e.g. CreateMany results).
	Transport          NotifyTransport
	TransportChannelID string
	ChannelName        string
}

type MessageMIME string

const (
	HTMLMessageMIME MessageMIME = "text/html"
	TextMessageMIME MessageMIME = "text/plain"
)

// NotifyMessage is a rendered notification ready for delivery.
type NotifyMessage struct {
	Subject     string
	Body        string
	MessageMIME MessageMIME
}
