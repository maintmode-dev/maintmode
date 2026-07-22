package entity

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInvitationRotationTaskTypes_RegisteredInGuard(t *testing.T) {
	// The startup coverage guard reads ActiveProcessorTaskTypes: every rotation
	// task type (task + its .cron pair) must be present, or the periodic job it
	// backs is registered without the guard expecting it and startup panics.
	for _, taskType := range []string{
		ProcessorTaskInvitationRotate,
		ProcessorTaskInvitationRotateCron,
		ProcessorTaskInvitationPrune,
		ProcessorTaskInvitationPruneCron,
	} {
		_, ok := ActiveProcessorTaskTypes[taskType]
		require.Truef(t, ok, "task type %q must be in ActiveProcessorTaskTypes", taskType)
	}
}

func TestProcessorTaskPayloadInvitationPrune_JSONRoundTrip(t *testing.T) {
	want := ProcessorTaskPayloadInvitationPrune{
		Retention:  8760 * time.Hour,
		BatchLimit: 1000,
	}

	raw, err := json.Marshal(want)
	require.NoError(t, err)

	var got ProcessorTaskPayloadInvitationPrune
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, want, got)
}

func TestProcessorTaskPayloadInvitationRotate_JSONRoundTrip(t *testing.T) {
	want := ProcessorTaskPayloadInvitationRotate{BatchLimit: 500}

	raw, err := json.Marshal(want)
	require.NoError(t, err)

	var got ProcessorTaskPayloadInvitationRotate
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, want, got)
}

func TestProcessorTaskPayloadAuditWrite_JSONRoundTrip(t *testing.T) {
	eventID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	occurred := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	want := ProcessorTaskPayloadAuditWrite{
		EventID:          eventID,
		OccurredAt:       occurred,
		Action:           AuditActionRolesChanged,
		Actor:            "admin@example.com",
		ActorID:          "actor-1",
		ActorDisplayName: "Admin",
		EntityID:         "target-1",
		EntityType:       AuditEntityTypeUser,
		Details:          "roles changed",
		Metadata: &AuditMetadata{
			Roles:             []string{"admin"},
			TargetEmail:       "target@example.com",
			TargetDisplayName: "Target",
		},
	}

	raw, err := json.Marshal(want)
	require.NoError(t, err)

	var got ProcessorTaskPayloadAuditWrite
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, want, got)
}

func TestProcessorTaskPayloadAuditWrite_ToAuditEntry(t *testing.T) {
	eventID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	occurred := time.Date(2026, 6, 15, 12, 30, 0, 0, time.UTC)

	payload := ProcessorTaskPayloadAuditWrite{
		EventID:          eventID,
		OccurredAt:       occurred,
		Action:           AuditActionLoginSuccess,
		Actor:            "user@example.com",
		ActorID:          "user-1",
		ActorDisplayName: "User",
		EntityID:         "user-1",
		EntityType:       AuditEntityTypeUser,
		Details:          "login success",
		Metadata:         &AuditMetadata{IP: "1.2.3.4"},
	}

	entry := payload.ToAuditEntry()

	// ID and CreatedAt are reproduced from the idempotency key and event time
	// so the record is identical across processor retries.
	require.Equal(t, eventID, entry.ID)
	require.Equal(t, eventID, entry.EventID)
	require.Equal(t, occurred, entry.CreatedAt)
	require.Equal(t, payload.Action, entry.Action)
	require.Equal(t, payload.Actor, entry.Actor)
	require.Equal(t, payload.ActorID, entry.ActorID)
	require.Equal(t, payload.ActorDisplayName, entry.ActorDisplayName)
	require.Equal(t, payload.EntityID, entry.EntityID)
	require.Equal(t, payload.EntityType, entry.EntityType)
	require.Equal(t, payload.Details, entry.Details)
	require.Equal(t, payload.Metadata, entry.Metadata)
}
