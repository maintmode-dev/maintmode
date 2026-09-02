package audit

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Integration registry audited actions. Each records that Actor created
// or updated an integration of a given kind. The snapshot deliberately carries
// only the kind and enabled flag — never config values and never secret values —
// so a credential can't leak into the durable audit trail.

// IntegrationCreated records that Actor created an integration.
type IntegrationCreated struct {
	Actor   *entity.User
	Kind    string
	Enabled bool
}

func (IntegrationCreated) auditAction() entity.AuditAction {
	return entity.AuditActionIntegrationCreated
}

// IntegrationUpdated records that Actor updated (or toggled) an integration.
type IntegrationUpdated struct {
	Actor   *entity.User
	Kind    string
	Enabled bool
}

func (IntegrationUpdated) auditAction() entity.AuditAction {
	return entity.AuditActionIntegrationUpdated
}

// fillIntegrationPayload renders an integration action to its snapshot. Only the
// kind, enabled flag, and actor are recorded — no config, no secrets — so the
// durable audit trail can never carry a credential.
func fillIntegrationPayload(payload *entity.ProcessorTaskPayloadAuditWrite, action Action) error {
	switch a := action.(type) {
	case IntegrationCreated:
		fillIntegrationAction(payload, a.Actor, a.Kind, a.Enabled, "created")
	case IntegrationUpdated:
		fillIntegrationAction(payload, a.Actor, a.Kind, a.Enabled, "updated")
	default:
		return fmt.Errorf("%w: %T", apperr.ErrUnsupportedEvent, a)
	}
	return nil
}

func fillIntegrationAction(
	payload *entity.ProcessorTaskPayloadAuditWrite,
	actor *entity.User,
	kind string,
	enabled bool,
	verb string,
) {
	setActor(payload, actor)
	payload.Details = fmt.Sprintf("integration %q %s (enabled=%t) by %s", kind, verb, enabled, actor.Email)
	payload.EntityType = entity.AuditEntityTypeIntegration
	payload.EntityID = kind
}
