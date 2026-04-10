package audit

import (
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func toDBEntry(e *entity.AuditEntry) *model.AuditLog {
	return &model.AuditLog{
		Action:     string(e.Action),
		Actor:      e.Actor,
		EntityType: string(e.EntityType),
		EntityID:   e.ID.String(),
		TargetType: string(e.TargetType),
		Details:    e.Details,
	}
}

func fromDBEntry(e *model.AuditLog) *entity.AuditEntry {
	return &entity.AuditEntry{
		ID:         e.ID,
		Action:     entity.AuditAction(e.Action),
		Actor:      e.Actor,
		EntityType: entity.AuditEntityType(e.EntityType),
		EntityID:   e.ID.String(),
		TargetType: entity.AuditEntityType(e.TargetType),
		Details:    e.Details,
		CreatedAt:  e.CreatedAt,
	}
}
