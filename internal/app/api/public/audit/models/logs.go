package apiauthmodels

import (
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

type AuditLog struct {
	ID         uuid.UUID              `json:"id"`
	Action     entity.AuditAction     `json:"action"`
	Actor      string                 `json:"actor"`
	EntityType entity.AuditEntityType `json:"entity_type,omitempty"`
	EntityID   string                 `json:"entity_id,omitempty"`
	TargetType entity.AuditEntityType `json:"target_type,omitempty"`
	TargetID   string                 `json:"target_id,omitempty"`
	Details    string                 `json:"details,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

type AuditLogResponse struct {
	Logs []*AuditLog `json:"logs,omitempty"`
}
