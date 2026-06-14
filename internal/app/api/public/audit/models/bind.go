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
		FailureReason:     string(m.FailureReason),
		LogoutKind:        string(m.LogoutKind),
		Roles:             m.Roles,
		RolesAdded:        m.RolesAdded,
		RolesRemoved:      m.RolesRemoved,
		TargetEmail:       m.TargetEmail,
		TargetDisplayName: m.TargetDisplayName,
	}
}
func ToAPIAuditLogResponse(page *entity.AuditLogsPage) *AuditLogResponse {
	return &AuditLogResponse{
		Logs: lo.Map(page.Logs, func(log *entity.AuditEntry, _ int) *AuditLog {
			return ToAPIAuditLog(log)
		}),
		Total: page.Total,
		Facets: AuditFacets{
			All:   page.Facets.All,
			Auth:  page.Facets.Auth,
			Roles: page.Facets.Roles,
			Block: page.Facets.Block,
		},
	}
}
