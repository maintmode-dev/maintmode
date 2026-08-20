package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
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
		CreatedAt:        e.CreatedAt,
		EventID:          eventIDToDB(e.EventID),
	}
}

// eventIDToDB maps the idempotency key to a nullable column: uuid.Nil (a
// legacy or non-outbox write) becomes NULL so it never participates in the
// unique-event_id conflict check.
func eventIDToDB(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

// eventIDFromDB maps the nullable column back: NULL (legacy rows) becomes
// uuid.Nil.
func eventIDFromDB(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func fromDBEntry(ctx context.Context, e *model.AuditLog) *entity.AuditEntry {
	return &entity.AuditEntry{
		ID:               e.ID,
		EventID:          eventIDFromDB(e.EventID),
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

// metadataToJSON serializes the whitelist metadata into the JSONB column. A
// serialization error does not fail the audit record — the metadata degrades to "{}".
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

// metadataFromJSON parses the JSONB column; an empty object and broken JSON both
// yield nil so the API can omit the field entirely.
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
