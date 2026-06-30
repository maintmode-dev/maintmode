package entity

import (
	"time"

	"github.com/google/uuid"
)

const (
	// ProcessorTaskMessagingSend is the goque task type for sending messages.
	ProcessorTaskMessagingSend = "messaging.send"
	// ProcessorTaskInvitationEmailSend is the goque task type for invitation
	// emails. It is distinct from ProcessorTaskMessagingSend so the invitation path
	// drains through the invitation email transport, not the generic messaging
	// registry: both task types share the goque_task table, and a misrouted
	// invitation must never be picked up by the messaging.send processor and sent
	// through its differently-configured registry.
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
	// fire time, not snapshotted at enqueue time.
	ProcessorTaskMaintAutoCancel     = "maint.auto.cancel"
	ProcessorTaskMaintAutoCancelCron = "maint.auto.cancel.cron"
	// ProcessorTaskAuditPrune is the goque task type produced by the audit-retention
	// periodic job. Its processor deletes audit_log rows older than the retention
	// window in bounded batches. The payload carries the retention window and batch
	// limit (from config).
	ProcessorTaskAuditPrune     = "audit.prune"
	ProcessorTaskAuditPruneCron = "audit.prune.cron"
	// ProcessorTaskAuditWrite is the goque task type produced when a service
	// publishes an audited domain event via auditpublisher.Publish (RUK-179). Its
	// processor decodes the rendered audit snapshot and writes it to audit_log
	// after commit, outside any tx.
	//
	// The payload carries the fully rendered AuditEntry snapshot — a deliberate
	// exception to the project's "payload = id, not content" rule, because audit
	// must be an immutable point-in-time record (actor_display_name is captured
	// at event time, never resolved on read).
	ProcessorTaskAuditWrite = "audit.write"
)

// ActiveProcessorTaskTypes is the set of goque task types the process must
// currently register a processor (or periodic job) for. It is the single source
// of truth for the startup coverage guard: every type here must have a
// registered processor (else an enqueued task lingers in the goque_task table
// forever and its work is silently lost), and no processor may be registered for
// a type absent from this set. Cron task types (*.cron) are listed here too so
// the check stays exhaustive.
//
// To disable a processor without freeing its type string for reuse, remove the
// type from this set AND stop registering its processor — but keep its
// ProcessorTask* const declared. The string then stays reserved (no accidental
// re-use) while the guard no longer expects it to be drained.
var ActiveProcessorTaskTypes = map[string]struct{}{
	ProcessorTaskMessagingSend:       {},
	ProcessorTaskMaintReminder:       {},
	ProcessorTaskMaintAutoCancel:     {},
	ProcessorTaskMaintAutoCancelCron: {},
	ProcessorTaskInvitationEmailSend: {},
	ProcessorTaskAuditWrite:          {},
	ProcessorTaskAuditPrune:          {},
	ProcessorTaskAuditPruneCron:      {},
}

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

// ProcessorTaskPayloadAuditWrite is the payload of an audit-write task
// (RUK-179). It is the rendered, point-in-time snapshot of one audit event:
// the publisher fills every persisted field at dispatch time and the processor
// writes it verbatim. Unlike the other payloads it carries content, not an id
// — audit records must not be re-resolved on read.
//
// EventID is the per-event idempotency key: it is the goque task external_id
// (at-most-once enqueue) and audit_log.event_id (ON CONFLICT DO NOTHING guards
// against processor re-delivery). OccurredAt is stamped at dispatch time and
// becomes audit_log.created_at so ordering reflects event time, not insert time.
type ProcessorTaskPayloadAuditWrite struct {
	EventID          uuid.UUID       `json:"event_id"`
	OccurredAt       time.Time       `json:"occurred_at"`
	Action           AuditAction     `json:"action"`
	Actor            string          `json:"actor"`
	ActorID          string          `json:"actor_id"`
	ActorDisplayName string          `json:"actor_display_name"`
	EntityID         string          `json:"entity_id"`
	EntityType       AuditEntityType `json:"entity_type"`
	Details          string          `json:"details"`
	Metadata         *AuditMetadata  `json:"metadata,omitempty"`
}

// ToAuditEntry rebuilds the persisted AuditEntry from the snapshot. ID and
// CreatedAt are taken from the idempotency key and event time so the record is
// reproducible across processor retries.
func (p ProcessorTaskPayloadAuditWrite) ToAuditEntry() *AuditEntry {
	return &AuditEntry{
		ID:               p.EventID,
		EventID:          p.EventID,
		Action:           p.Action,
		Actor:            p.Actor,
		ActorID:          p.ActorID,
		ActorDisplayName: p.ActorDisplayName,
		EntityID:         p.EntityID,
		EntityType:       p.EntityType,
		Details:          p.Details,
		Metadata:         p.Metadata,
		CreatedAt:        p.OccurredAt,
	}
}
