package audit

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Catalog audited actions: the resource and notify-channel catalogs that
// maintenances draw from. Each records the row's name at event time (the row
// may be renamed later, and the trail must say what it was called then), keyed
// by the row's id, plus what an update moved.

// ResourceCreated records that Actor added a resource to the catalog.
type ResourceCreated struct {
	Actor    *entity.User
	Resource *entity.ResourceDetails
}

func (ResourceCreated) auditAction() entity.AuditAction { return entity.AuditActionResourceCreated }

// ResourceUpdated records that Actor edited a resource. Changes holds the
// fields that moved; Resource is the row after the edit.
type ResourceUpdated struct {
	Actor    *entity.User
	Resource *entity.ResourceDetails
	Changes  []entity.AuditFieldChange
}

func (ResourceUpdated) auditAction() entity.AuditAction { return entity.AuditActionResourceUpdated }

// ResourceArchived records that Actor archived an active resource.
type ResourceArchived struct {
	Actor    *entity.User
	Resource *entity.ResourceDetails
}

func (ResourceArchived) auditAction() entity.AuditAction { return entity.AuditActionResourceArchived }

// ResourceUnarchived records that Actor restored an archived resource.
type ResourceUnarchived struct {
	Actor    *entity.User
	Resource *entity.ResourceDetails
}

func (ResourceUnarchived) auditAction() entity.AuditAction {
	return entity.AuditActionResourceUnarchived
}

// NotifyChannelCreated records that Actor added a channel to the catalog.
type NotifyChannelCreated struct {
	Actor   *entity.User
	Channel *entity.NotifyChannel
}

func (NotifyChannelCreated) auditAction() entity.AuditAction {
	return entity.AuditActionNotifyChannelCreated
}

// NotifyChannelUpdated records that Actor edited a channel. Changes holds the
// fields that moved; Channel is the row after the edit.
type NotifyChannelUpdated struct {
	Actor   *entity.User
	Channel *entity.NotifyChannel
	Changes []entity.AuditFieldChange
}

func (NotifyChannelUpdated) auditAction() entity.AuditAction {
	return entity.AuditActionNotifyChannelUpdated
}

// NotifyChannelArchived records that Actor archived an active channel.
type NotifyChannelArchived struct {
	Actor   *entity.User
	Channel *entity.NotifyChannel
}

func (NotifyChannelArchived) auditAction() entity.AuditAction {
	return entity.AuditActionNotifyChannelArchived
}

// NotifyChannelUnarchived records that Actor restored an archived channel.
type NotifyChannelUnarchived struct {
	Actor   *entity.User
	Channel *entity.NotifyChannel
}

func (NotifyChannelUnarchived) auditAction() entity.AuditAction {
	return entity.AuditActionNotifyChannelUnarchived
}

// fillCatalogPayload renders a resource or notify-channel action.
func fillCatalogPayload(payload *entity.ProcessorTaskPayloadAuditWrite, action Action) error {
	switch a := action.(type) {
	case ResourceCreated:
		fillResourceAction(payload, a.Actor, a.Resource, "created", nil)
	case ResourceUpdated:
		fillResourceAction(payload, a.Actor, a.Resource, "updated", a.Changes)
	case ResourceArchived:
		fillResourceAction(payload, a.Actor, a.Resource, "archived", nil)
	case ResourceUnarchived:
		fillResourceAction(payload, a.Actor, a.Resource, "unarchived", nil)
	case NotifyChannelCreated:
		fillChannelAction(payload, a.Actor, a.Channel, "created", nil)
	case NotifyChannelUpdated:
		fillChannelAction(payload, a.Actor, a.Channel, "updated", a.Changes)
	case NotifyChannelArchived:
		fillChannelAction(payload, a.Actor, a.Channel, "archived", nil)
	case NotifyChannelUnarchived:
		fillChannelAction(payload, a.Actor, a.Channel, "unarchived", nil)
	default:
		return fmt.Errorf("%w: %T", apperr.ErrUnsupportedEvent, a)
	}

	return nil
}

func fillResourceAction(
	payload *entity.ProcessorTaskPayloadAuditWrite,
	actor *entity.User,
	resource *entity.ResourceDetails,
	verb string,
	changes []entity.AuditFieldChange,
) {
	setActor(payload, actor)
	payload.Details = fmt.Sprintf("resource %q %s by %s", resource.Name, verb, actor.Email)
	payload.EntityType = entity.AuditEntityTypeResource
	payload.EntityID = resource.ID.String()
	payload.Metadata = &entity.AuditMetadata{
		TargetDisplayName: resource.Name,
		Changes:           changes,
	}
}

func fillChannelAction(
	payload *entity.ProcessorTaskPayloadAuditWrite,
	actor *entity.User,
	channel *entity.NotifyChannel,
	verb string,
	changes []entity.AuditFieldChange,
) {
	setActor(payload, actor)
	payload.Details = fmt.Sprintf("%s channel %q %s by %s", channel.Transport, channel.Name, verb, actor.Email)
	payload.EntityType = entity.AuditEntityTypeNotifyChannel
	payload.EntityID = channel.ID.String()
	payload.Metadata = &entity.AuditMetadata{
		TargetDisplayName: channel.Name,
		Changes:           changes,
	}
}
