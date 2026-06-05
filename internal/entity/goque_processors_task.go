package entity

import "github.com/google/uuid"

const (
	// ProcessorTaskMessagingSend is the goque task type for sending messages.
	ProcessorTaskMessagingSend = "messaging.send"
	// ProcessorTaskInvitationEmailSend is the goque task type for invitation
	// emails. It is distinct from ProcessorTaskMessagingSend so that only the auth
	// binary (which owns invitations and the email transport) drains it — the
	// maintmode binary, which also registers a messaging.send processor against the
	// shared goque_task table, must never pick up an invitation email and route it
	// through its own differently-configured registry.
	ProcessorTaskInvitationEmailSend = "invitation.email"
	// ProcessorTaskMaintReminder is the goque task type for deferred reminders.
	// Its string value coincides with NotifyEventMaintReminder but they are
	// independent concepts (queue task type vs notify event kind) — keep both in
	// sync if either is renamed.
	ProcessorTaskMaintReminder = "maint.reminder"
)

// ProcessorTaskPayloadEventNotify is the typed payload stored in each goque task.
type ProcessorTaskPayloadEventNotify struct {
	TransportName NotifyTransport `json:"transport"`
	Target        string          `json:"target"`
	Subject       string          `json:"subject"`
	Body          string          `json:"body"`
	MessageMIME   MessageMIME     `json:"mime"`
}

// ProcessorTaskPayloadMaintReminder is the payload of a deferred-reminder task.
// It carries only the maintenance id; the processor resolves the maintenance's
// current notify targets and renders the maint.reminder template at fire time.
type ProcessorTaskPayloadMaintReminder struct {
	MaintID    uuid.UUID `json:"maint_id"`
	DeferredID uuid.UUID `json:"deferred_id"`
}
