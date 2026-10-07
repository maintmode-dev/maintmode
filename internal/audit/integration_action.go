package audit

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Integration registry audited actions. Each records that Actor created,
// updated or deleted one integration: its identity (kind and name) and the
// enabled flag, and for an update what changed. Never a secret value -- a
// changed secret appears by name only -- so a credential can't leak into the
// durable audit trail. Config values may appear: a config holds no secret, and
// GET already returns it verbatim to the same admins. The name is an
// identifier, already visible in every URL that addresses the row, and without
// it two providers of one kind are indistinguishable here.

// IntegrationCreated records that Actor created an integration.
type IntegrationCreated struct {
	Actor   *entity.User
	Kind    string
	Name    string
	Enabled bool
}

func (IntegrationCreated) auditAction() entity.AuditAction {
	return entity.AuditActionIntegrationCreated
}

// IntegrationUpdated records that Actor updated (or toggled) an integration.
// Changes is what the update moved: config fields before and after, the enabled
// flag, and each secret replaced or cleared, by name only. Empty for a toggle,
// whose whole change is the flag in Enabled.
type IntegrationUpdated struct {
	Actor   *entity.User
	Kind    string
	Name    string
	Enabled bool
	Changes []entity.AuditFieldChange
}

func (IntegrationUpdated) auditAction() entity.AuditAction {
	return entity.AuditActionIntegrationUpdated
}

// IntegrationDeleted records that Actor removed an integration. Enabled is
// absent: a deleted row has no flag left to report, and the delete itself is
// the fact worth keeping.
type IntegrationDeleted struct {
	Actor *entity.User
	Kind  string
	Name  string
}

func (IntegrationDeleted) auditAction() entity.AuditAction {
	return entity.AuditActionIntegrationDeleted
}

// fillIntegrationPayload renders an integration action to its snapshot: the
// identity, enabled flag and actor, plus an update's changes. No secret value
// is ever recorded, so the durable audit trail can never carry a credential.
func fillIntegrationPayload(payload *entity.ProcessorTaskPayloadAuditWrite, action Action) error {
	switch a := action.(type) {
	case IntegrationCreated:
		fillIntegrationAction(payload, a.Actor, a.Kind, a.Name, a.Enabled, "created")
	case IntegrationUpdated:
		fillIntegrationAction(payload, a.Actor, a.Kind, a.Name, a.Enabled, "updated")
		if len(a.Changes) > 0 {
			payload.Metadata = &entity.AuditMetadata{Changes: a.Changes}
		}
	case IntegrationDeleted:
		fillIntegrationDeleteAction(payload, a.Actor, a.Kind, a.Name)
	default:
		return fmt.Errorf("%w: %T", apperr.ErrUnsupportedEvent, a)
	}
	return nil
}

func fillIntegrationAction(
	payload *entity.ProcessorTaskPayloadAuditWrite,
	actor *entity.User,
	kind, name string,
	enabled bool,
	verb string,
) {
	setActor(payload, actor)
	payload.Details = fmt.Sprintf("integration %s %s (enabled=%t) by %s",
		integrationRef(kind, name), verb, enabled, actor.Email)
	payload.EntityType = entity.AuditEntityTypeIntegration
	payload.EntityID = auditEntityID(kind, name)
}

func fillIntegrationDeleteAction(
	payload *entity.ProcessorTaskPayloadAuditWrite,
	actor *entity.User,
	kind, name string,
) {
	setActor(payload, actor)
	payload.Details = fmt.Sprintf("integration %s deleted by %s", integrationRef(kind, name), actor.Email)
	payload.EntityType = entity.AuditEntityTypeIntegration
	payload.EntityID = auditEntityID(kind, name)
}

// integrationRef renders the identity for a human reading the trail. Both
// halves, because a kind alone stopped identifying a row once several instances
// of one kind could coexist -- two OIDC providers would otherwise produce
// entries nobody can tell apart.
func integrationRef(kind, name string) string {
	if name == "" {
		return fmt.Sprintf("%q", kind)
	}

	return fmt.Sprintf("%q/%q", kind, name)
}

// auditEntityID keys the trail by the same identity the REST path uses, so an
// entry can be traced back to the row it describes.
//
// The name is safe here under this file's own rule: it is an identifier, not a
// credential, and it is already visible in every URL that addresses the row.
//
// An absent name yields the bare kind rather than a trailing slash, so entries
// written before instances existed keep the id they already have and stay
// comparable with the ones written after.
func auditEntityID(kind, name string) string {
	if name == "" {
		return kind
	}

	return kind + "/" + name
}
