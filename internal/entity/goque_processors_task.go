package entity

import (
	"time"

	"github.com/google/uuid"
)

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
	// ProcessorTaskMaintAutoCancel is the goque task type produced by the
	// every-minute periodic job. Its processor sweeps not-started maintenances
	// (draft or planned) whose planned start has passed the grace window without
	// being started and cancels them. The payload is empty: the sweep is computed at
	// fire time, not snapshotted at enqueue time. maintmode-only — not registered by
	// the auth binary.
	ProcessorTaskMaintAutoCancel     = "maint.auto.cancel"
	ProcessorTaskMaintAutoCancelCron = "maint.auto.cancel.cron"
	// ProcessorTaskAuditPrune is the goque task type produced by the audit-retention
	// periodic job. Its processor deletes audit_log rows older than the retention
	// window in bounded batches. The payload carries the retention window and batch
	// limit (from config). auth-binary-only: the auth binary owns the audit store
	// (read + write), so it also owns the retention prune — not registered by
	// maintmode.
	ProcessorTaskAuditPrune     = "audit.prune"
	ProcessorTaskAuditPruneCron = "audit.prune.cron"
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

// ProcessorTaskPayloadMaintAutoCancel is the payload of an auto-cancel sweep task.
// The cron job stamps the tunables (from config) into each task so the processor
// stays config-free: Threshold is the grace window after planned start, Limit
// bounds how many maintenances one sweep cancels.
type ProcessorTaskPayloadMaintAutoCancel struct {
	Threshold time.Duration `json:"threshold"`
	Limit     int64         `json:"limit"`
}

// ProcessorTaskPayloadAuditPrune is the payload of an audit-retention sweep task.
// The cron job stamps the tunables (from config) into each task so the processor
// stays config-free: Retention is the age threshold past which a row is deleted,
// BatchLimit bounds how many rows one DELETE statement removes.
type ProcessorTaskPayloadAuditPrune struct {
	Retention  time.Duration `json:"retention"`
	BatchLimit int64         `json:"batch_limit"`
}
