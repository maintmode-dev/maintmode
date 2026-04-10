package apiauthmodels

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

func ToAPIAuditLog(log *entity.AuditEntry) *AuditLog {
	return &AuditLog{
		ID:         log.ID,
		Action:     log.Action,
		Actor:      log.Actor,
		EntityType: log.EntityType,
		EntityID:   log.EntityID,
		TargetType: log.TargetType,
		TargetID:   log.TargetID,
		Details:    log.Details,
		CreatedAt:  log.CreatedAt,
	}
}
func ToAPIAuditLogResponse(logs []*entity.AuditEntry) *AuditLogResponse {
	return &AuditLogResponse{
		Logs: lo.Map(logs, func(log *entity.AuditEntry, _ int) *AuditLog {
			return ToAPIAuditLog(log)
		}),
	}
}
