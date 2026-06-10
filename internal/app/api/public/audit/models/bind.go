package apiauthmodels

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

func ToAPIAuditLog(log *entity.AuditEntry) *AuditLog {
	return &AuditLog{
		ID:               log.ID,
		Action:           log.Action,
		Actor:            log.Actor,
		ActorID:          log.ActorID,
		ActorDisplayName: log.ActorDisplayName,
		EntityType:       log.EntityType,
		EntityID:         log.EntityID,
		TargetType:       log.TargetType,
		TargetID:         log.TargetID,
		Details:          log.Details,
		Metadata:         toAPIAuditLogMetadata(log.Metadata),
		CreatedAt:        log.CreatedAt,
	}
}

func toAPIAuditLogMetadata(m *entity.AuditMetadata) *AuditLogMetadata {
	if m == nil {
		return nil
	}
	return &AuditLogMetadata{
		IP:                m.IP,
		UserAgent:         m.UserAgent,
		SessionID:         m.SessionID,
		FailureReason:     m.FailureReason,
		LogoutKind:        m.LogoutKind,
		Roles:             m.Roles,
		RolesAdded:        m.RolesAdded,
		RolesRemoved:      m.RolesRemoved,
		TargetEmail:       m.TargetEmail,
		TargetDisplayName: m.TargetDisplayName,
	}
}
func ToAPIAuditLogResponse(logs []*entity.AuditEntry) *AuditLogResponse {
	return &AuditLogResponse{
		Logs: lo.Map(logs, func(log *entity.AuditEntry, _ int) *AuditLog {
			return ToAPIAuditLog(log)
		}),
	}
}
