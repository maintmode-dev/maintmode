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
	// ProcessorTaskAuditWrite is the goque task type produced when a service
	// publishes an audited domain event via auditpublisher.Publish (RUK-179). Its
	// processor decodes the rendered audit snapshot and writes it to audit_log
	// after commit, outside any tx. It may be *enqueued* on any binary (auth
	// today; maintmode for maint audit, RUK-182) but is *drained* only on the
	// auth binary, which owns the audit store.
	//
	// The payload carries the fully rendered AuditEntry snapshot — a deliberate
	// exception to the project's "payload = id, not content" rule, because audit
	// must be an immutable point-in-time record (actor_display_name is captured
	// at event time, never resolved on read).
	ProcessorTaskAuditWrite = "audit.write"
)

// ProcessorBinary identifies which application binary owns (drains) a goque task
// type. The goque_task table is shared across binaries (see the task-type
// isolation note), so every task type must be drained by exactly one binary;
// otherwise an enqueued task lingers forever and its work — e.g. an audit record
// — is silently lost. ProcessorTaskOwner is the single source of truth, asserted
// at startup and in a CI test.
type ProcessorBinary string

const (
	ProcessorBinaryMaintmode ProcessorBinary = "maintmode"
	ProcessorBinaryAuth      ProcessorBinary = "auth"
)

// ProcessorTaskOwner maps every goque task type to the binary that drains it.
// Adding a task type without adding it here fails the coverage test; registering
// it on the wrong binary fails the startup guard. Cron task types
// (*.cron) are produced by RegisterPeriodicJob on their owning binary and listed
// here too so the coverage check stays exhaustive.
var ProcessorTaskOwner = map[string]ProcessorBinary{
	ProcessorTaskMessagingSend:       ProcessorBinaryMaintmode,
	ProcessorTaskMaintReminder:       ProcessorBinaryMaintmode,
	ProcessorTaskMaintAutoCancel:     ProcessorBinaryMaintmode,
	ProcessorTaskMaintAutoCancelCron: ProcessorBinaryMaintmode,
	ProcessorTaskInvitationEmailSend: ProcessorBinaryAuth,
	ProcessorTaskAuditWrite:          ProcessorBinaryAuth,
	ProcessorTaskAuditPrune:          ProcessorBinaryAuth,
	ProcessorTaskAuditPruneCron:      ProcessorBinaryAuth,
}

// ProcessorTaskTypesFor returns the task types the given binary must register a
// processor (or periodic job) for.
func ProcessorTaskTypesFor(binary ProcessorBinary) []string {
	var types []string
	for taskType, owner := range ProcessorTaskOwner {
		if owner == binary {
			types = append(types, taskType)
		}
	}
	return types
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
