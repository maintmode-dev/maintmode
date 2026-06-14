package audit

import (
	"context"
	"encoding/json"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

const emptyMetadataJSON = "{}"

func toDBEntry(ctx context.Context, e *entity.AuditEntry) *model.AuditLog {
	return &model.AuditLog{
		Action:           string(e.Action),
		Actor:            e.Actor,
		ActorID:          e.ActorID,
		ActorDisplayName: e.ActorDisplayName,
		EntityType:       string(e.EntityType),
		EntityID:         e.EntityID,
		Details:          e.Details,
		Metadata:         metadataToJSON(ctx, e.Metadata),
	}
}

func fromDBEntry(ctx context.Context, e *model.AuditLog) *entity.AuditEntry {
	return &entity.AuditEntry{
		ID:               e.ID,
		Action:           entity.AuditAction(e.Action),
		Actor:            e.Actor,
		ActorID:          e.ActorID,
		ActorDisplayName: e.ActorDisplayName,
		EntityType:       entity.AuditEntityType(e.EntityType),
		EntityID:         e.EntityID,
		Details:          e.Details,
		Metadata:         metadataFromJSON(ctx, e.Metadata),
		CreatedAt:        e.CreatedAt,
	}
}

// metadataToJSON сериализует whitelist-метаданные в JSONB-колонку. Ошибка
// сериализации не валит запись аудита — метаданные деградируют до "{}".
func metadataToJSON(ctx context.Context, m *entity.AuditMetadata) string {
	if m == nil {
		return emptyMetadataJSON
	}
	raw, err := json.Marshal(m)
	if err != nil {
		xlog.Error(ctx, "failed to marshal audit metadata", xfield.Error(err))
		return emptyMetadataJSON
	}
	return string(raw)
}

// metadataFromJSON парсит JSONB-колонку; пустой объект и битый JSON дают nil,
// чтобы API мог опустить поле целиком.
func metadataFromJSON(ctx context.Context, raw string) *entity.AuditMetadata {
	if raw == "" || raw == emptyMetadataJSON {
		return nil
	}
	m := new(entity.AuditMetadata)
	if err := json.Unmarshal([]byte(raw), m); err != nil {
		xlog.Error(ctx, "failed to unmarshal audit metadata", xfield.Error(err))
		return nil
	}
	return m
}
