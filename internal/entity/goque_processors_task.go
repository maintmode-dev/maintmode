package entity

import (
	"maps"
	"time"

	"github.com/google/uuid"
)

const (
	// ProcessorTaskMessagingSend was the goque task type for queue-backed
	// notification delivery. It is retired: maintenance notifications are sent
	// inline by the notifier, so nothing enqueues this type and no processor is
	// registered for it. The const stays declared, and the type is listed in
	// disabledTaskTypes rather than ActiveProcessorTaskTypes, so the string cannot
	// be recycled for unrelated work while the startup guard stops expecting a
	// drainer for it.
	ProcessorTaskMessagingSend = "messaging.send"
	// ProcessorTaskInvitationEmailSend is the goque task type for invitation
	// emails. It is deliberately its own type rather than a generic "send a
	// message" one: task types share the goque_task table, and routing invitations
	// through a dedicated type keeps them bound to the invitation email transport
	// instead of whichever registry a general-purpose sender happens to carry.
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
	// ProcessorTaskLicenseHeartbeat is the goque task type produced once per cron
	// tick by the license client (SaaS mode only). Its processor collects
	// the seat/activity report at process time (payload is empty — a stale
	// enqueued snapshot must never reach Console), sends the heartbeat and caches
	// the returned license. Registered only when config license mode is enabled;
	// see ExpectedProcessorTaskTypes.
	ProcessorTaskLicenseHeartbeat     = "license.heartbeat"
	ProcessorTaskLicenseHeartbeatCron = "license.heartbeat.cron"
	// ProcessorTaskInvitationRotate is the goque task type produced by the
	// invitation-rotation periodic job. Its processor flips pending invitations
	// whose expires_at has passed to the persisted 'expired' status in bounded
	// batches, so the stored status matches reality (until now 'expired' was only
	// derived on read). A rotated row leaves the partial-unique pending index,
	// freeing the email slot for a fresh invite. The payload carries the batch
	// limit (from config); the expiry boundary is process time, not snapshotted.
	ProcessorTaskInvitationRotate     = "invitation.rotate"
	ProcessorTaskInvitationRotateCron = "invitation.rotate.cron"
	// ProcessorTaskInvitationPrune is the goque task type produced by the
	// invitation-retention periodic job. Its processor deletes terminal
	// invitations (expired/accepted/revoked) whose created_at is older than the
	// retention window in bounded batches. pending rows are never pruned (they
	// leave via rotation first). The payload carries the retention window and
	// batch limit (from config).
	ProcessorTaskInvitationPrune     = "invitation.prune"
	ProcessorTaskInvitationPruneCron = "invitation.prune.cron"
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
	ProcessorTaskMaintReminder:        {},
	ProcessorTaskMaintAutoCancel:      {},
	ProcessorTaskMaintAutoCancelCron:  {},
	ProcessorTaskInvitationEmailSend:  {},
	ProcessorTaskAuditWrite:           {},
	ProcessorTaskAuditPrune:           {},
	ProcessorTaskAuditPruneCron:       {},
	ProcessorTaskInvitationRotate:     {},
	ProcessorTaskInvitationRotateCron: {},
	ProcessorTaskInvitationPrune:      {},
	ProcessorTaskInvitationPruneCron:  {},
}

// ExpectedProcessorTaskTypes returns the exact task-type set the process must
// register given its feature toggles: the always-on baseline plus the license
// client pair when SaaS mode is enabled. The coverage guard verifies
// against this so a self-hosted process is not forced to register license
// processors, and a license-enabled one cannot forget them.
func ExpectedProcessorTaskTypes(licenseEnabled bool) map[string]struct{} {
	expected := maps.Clone(ActiveProcessorTaskTypes)
	if licenseEnabled {
		expected[ProcessorTaskLicenseHeartbeat] = struct{}{}
		expected[ProcessorTaskLicenseHeartbeatCron] = struct{}{}
	}
	return expected
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

// ProcessorTaskPayloadInvitationRotate is the payload of an invitation-rotation
// sweep task. The cron job stamps the batch limit (from config) so the processor
// stays config-free. There is no retention field: rotation flips pending rows
// whose expires_at is past process time, so the boundary is "now", computed at
// fire time.
type ProcessorTaskPayloadInvitationRotate struct {
	BatchLimit int64 `json:"batch_limit"`
}

// ProcessorTaskPayloadInvitationPrune is the payload of an invitation-retention
// sweep task. The cron job stamps the tunables (from config) so the processor
// stays config-free: Retention is the age (by created_at) past which a terminal
// invitation is deleted, BatchLimit bounds how many rows one DELETE removes.
type ProcessorTaskPayloadInvitationPrune struct {
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
