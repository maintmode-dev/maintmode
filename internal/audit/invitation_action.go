package audit

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Invitation audited actions. Each records the invited address and the roles
// the invitation grants -- who may get in, and as what -- keyed by the
// invitation's id. Never the token or its hash: the link is a bearer secret.

// InvitationCreated records that Actor invited an address.
type InvitationCreated struct {
	Actor      *entity.User
	Invitation *entity.Invitation
}

func (InvitationCreated) auditAction() entity.AuditAction {
	return entity.AuditActionInvitationCreated
}

// InvitationRevoked records that Actor withdrew a pending invitation.
type InvitationRevoked struct {
	Actor      *entity.User
	Invitation *entity.Invitation
}

func (InvitationRevoked) auditAction() entity.AuditAction {
	return entity.AuditActionInvitationRevoked
}

// fillInvitationPayload renders an invitation action to its snapshot.
func fillInvitationPayload(payload *entity.ProcessorTaskPayloadAuditWrite, action Action) error {
	switch a := action.(type) {
	case InvitationCreated:
		fillInvitationAction(payload, a.Actor, a.Invitation, "created")
	case InvitationRevoked:
		fillInvitationAction(payload, a.Actor, a.Invitation, "revoked")
	default:
		return fmt.Errorf("%w: %T", apperr.ErrUnsupportedEvent, a)
	}

	return nil
}

func fillInvitationAction(
	payload *entity.ProcessorTaskPayloadAuditWrite,
	actor *entity.User,
	inv *entity.Invitation,
	verb string,
) {
	setActor(payload, actor)
	payload.Details = fmt.Sprintf("invitation for %s %s by %s", inv.Email, verb, actor.Email)
	payload.EntityType = entity.AuditEntityTypeInvitation
	payload.EntityID = inv.ID.String()
	payload.Metadata = &entity.AuditMetadata{
		Roles:       roleNames(inv.Roles),
		TargetEmail: inv.Email,
	}
}
