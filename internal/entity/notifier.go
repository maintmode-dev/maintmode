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
	// Mentions is filled by dispatch after the owner is filtered out and
	// blocked users are dropped, before any target is contacted — the same
	// once-per-event point as OwnerMention. Empty means there is no mention
	// line to render.
	Mentions []*UserMention

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

	// RootRef points at the "maintenance started" message delivered to this
	// channel; subsequent lifecycle events reply to it. Nil until the
	// maintenance starts, and for maintenances that started before threads
	// existed — those live out their lifetime flat.
	RootRef *MessageRef
	// RootChannel is the delivery address the root was actually sent to, copied
	// from the catalog at start time. Comparing it to the channel's CURRENT
	// address detects a re-pointed channel, whose root must not be replayed into
	// the new chat.
	//
	// The comparison has to be address-to-address. The canonical id a messenger
	// API answers with ("-1001234567890", "C0DEADBEEF") is written by a
	// different system than TransportChannelID, which is free-form operator text
	// ("@ops_alerts", "#ops"); the two legitimately differ, so matching one
	// against the other would report drift for every channel not addressed by
	// its exact canonical id.
	RootChannel string
}

// MessageRef identifies a delivered message.
//
// MessageID is a string on purpose: Slack ts is "1503435956.000247", and a trip
// through a numeric type silently breaks threading (the API answers ok:true and
// posts the message outside the thread).
//
// The conversation the message landed in is deliberately absent. A reply is
// addressed by MessageID alone — the destination comes from the catalog at send
// time — and the re-pointed-channel guard compares NotifyTarget.RootChannel.
// The canonical chat id is logged at delivery instead of stored, since nothing
// reads it back.
type MessageRef struct {
	// MessageID is what a reply is threaded under. A ref without it is not a ref.
	MessageID string
}

// SendResult reports what happened to a delivery.
type SendResult struct {
	// Ref points at the message just delivered. Nil for transports without
	// addressable messages (email, stub).
	Ref *MessageRef
	// RootReplaced is set when a reply was requested but the transport had to
	// fall back to a top-level message because the root was unusable. The
	// transport knows this as a fact; it makes Ref the message later events
	// should thread under, replacing the root that was refused.
	//
	// Callers must NEVER try to infer it by comparing ids. A successful
	// threaded reply is a new message with its OWN id, always different from
	// the root's — so "returned ref != requested replyTo" is true on success
	// too, and using it would re-seed the root on every correctly threaded
	// reply, leaving every thread exactly one message long.
	RootReplaced bool
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
