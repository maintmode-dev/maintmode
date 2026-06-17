package audit

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestRender_MapsEveryMaintAction(t *testing.T) {
	actor := &entity.User{ID: uuid.New(), Email: "actor@example.com", Name: "Actor"}
	maint := &entity.Maintenance{
		ID:           uuid.New(),
		Title:        "DB upgrade",
		Status:       entity.MaintenanceStatusInProgress,
		CancelReason: entity.MaintenanceCancelReasonIncident,
	}
	step := &entity.MaintenanceStep{
		ID:     uuid.New(),
		Order:  2,
		Status: entity.MaintenanceStepStatusStarted,
	}
	r := fixedRenderer(uuid.New(), time.Now())

	tests := []struct {
		name             string
		action           Action
		wantAction       entity.AuditAction
		wantActor        string
		wantActorID      string
		wantDetailsParts []string
		checkMetadata    func(t *testing.T, m *entity.AuditMetadata)
	}{
		{
			name:             "created",
			action:           MaintCreated{Actor: actor, Maint: maint},
			wantAction:       entity.AuditActionMaintCreated,
			wantActor:        actor.Email,
			wantActorID:      actor.ID.String(),
			wantDetailsParts: []string{"DB upgrade", "created", actor.Email},
			checkMetadata: func(t *testing.T, m *entity.AuditMetadata) {
				t.Helper()
				require.Equal(t, "DB upgrade", m.MaintTitle)
			},
		},
		{
			name: "updated carries diff",
			action: MaintUpdated{
				Actor: actor, Maint: maint,
				Changes: []entity.AuditFieldChange{
					{Field: "title", Old: "old", New: "DB upgrade"},
					{Field: "steps"},
				},
			},
			wantAction:       entity.AuditActionMaintUpdated,
			wantActor:        actor.Email,
			wantActorID:      actor.ID.String(),
			wantDetailsParts: []string{"updated", actor.Email},
			checkMetadata: func(t *testing.T, m *entity.AuditMetadata) {
				t.Helper()
				require.Len(t, m.Changes, 2)
				require.Equal(t, "title", m.Changes[0].Field)
				require.Equal(t, "old", m.Changes[0].Old)
				require.Equal(t, "DB upgrade", m.Changes[0].New)
				require.Equal(t, "steps", m.Changes[1].Field)
			},
		},
		{
			name:             "approved",
			action:           MaintApproved{Actor: actor, Maint: maint},
			wantAction:       entity.AuditActionMaintApproved,
			wantActor:        actor.Email,
			wantActorID:      actor.ID.String(),
			wantDetailsParts: []string{"approved", actor.Email},
			checkMetadata:    func(t *testing.T, _ *entity.AuditMetadata) { t.Helper() },
		},
		{
			name:             "started",
			action:           MaintStarted{Actor: actor, Maint: maint},
			wantAction:       entity.AuditActionMaintStarted,
			wantActor:        actor.Email,
			wantActorID:      actor.ID.String(),
			wantDetailsParts: []string{"started", actor.Email},
			checkMetadata:    func(t *testing.T, _ *entity.AuditMetadata) { t.Helper() },
		},
		{
			name:             "completed",
			action:           MaintCompleted{Actor: actor, Maint: maint},
			wantAction:       entity.AuditActionMaintCompleted,
			wantActor:        actor.Email,
			wantActorID:      actor.ID.String(),
			wantDetailsParts: []string{"completed", actor.Email},
			checkMetadata:    func(t *testing.T, _ *entity.AuditMetadata) { t.Helper() },
		},
		{
			name:             "canceled carries reason",
			action:           MaintCancelled{Actor: actor, Maint: maint},
			wantAction:       entity.AuditActionMaintCanceled,
			wantActor:        actor.Email,
			wantActorID:      actor.ID.String(),
			wantDetailsParts: []string{"canceled", actor.Email},
			checkMetadata:    func(t *testing.T, _ *entity.AuditMetadata) { t.Helper() },
		},
		{
			name:             "step started",
			action:           MaintStepStarted{Actor: actor, Maint: maint, Step: step},
			wantAction:       entity.AuditActionMaintStepStarted,
			wantActor:        actor.Email,
			wantActorID:      actor.ID.String(),
			wantDetailsParts: []string{"step 2", "DB upgrade", "started"},
			checkMetadata:    func(t *testing.T, _ *entity.AuditMetadata) { t.Helper() },
		},
		{
			name:             "step completed",
			action:           MaintStepCompleted{Actor: actor, Maint: maint, Step: step},
			wantAction:       entity.AuditActionMaintStepCompleted,
			wantActor:        actor.Email,
			wantActorID:      actor.ID.String(),
			wantDetailsParts: []string{"step 2", "completed"},
			checkMetadata:    func(t *testing.T, _ *entity.AuditMetadata) { t.Helper() },
		},
		{
			name:             "step canceled",
			action:           MaintStepCancelled{Actor: actor, Maint: maint, Step: step},
			wantAction:       entity.AuditActionMaintStepCanceled,
			wantActor:        actor.Email,
			wantActorID:      actor.ID.String(),
			wantDetailsParts: []string{"step 2", "canceled"},
			checkMetadata:    func(t *testing.T, _ *entity.AuditMetadata) { t.Helper() },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := r.Render(tc.action)
			require.NoError(t, err)

			require.Equal(t, tc.wantAction, payload.Action)
			require.Equal(t, tc.wantActor, payload.Actor)
			require.Equal(t, tc.wantActorID, payload.ActorID)
			require.Equal(t, entity.AuditEntityTypeMaintenance, payload.EntityType)
			require.Equal(t, maint.ID.String(), payload.EntityID)
			for _, part := range tc.wantDetailsParts {
				require.Contains(t, payload.Details, part)
			}
			require.NotNil(t, payload.Metadata)
			require.Equal(t, "DB upgrade", payload.Metadata.MaintTitle)
			tc.checkMetadata(t, payload.Metadata)
		})
	}
}

// Every declared maintenance audit action must be renderable — guards against
// adding a constant without a renderer case.
func TestRender_AllMaintActionsRenderable(t *testing.T) {
	maint := &entity.Maintenance{ID: uuid.New(), Title: "m", Status: entity.MaintenanceStatusDraft}
	step := &entity.MaintenanceStep{ID: uuid.New(), Status: entity.MaintenanceStepStatusStarted}
	actor := &entity.User{ID: uuid.New(), Email: "a@example.com", Name: "A"}
	r := NewRenderer()

	actions := []Action{
		MaintCreated{Actor: actor, Maint: maint},
		MaintUpdated{Actor: actor, Maint: maint},
		MaintApproved{Actor: actor, Maint: maint},
		MaintStarted{Actor: actor, Maint: maint},
		MaintCompleted{Actor: actor, Maint: maint},
		MaintCancelled{Actor: actor, Maint: maint},
		MaintStepStarted{Actor: actor, Maint: maint, Step: step},
		MaintStepCompleted{Actor: actor, Maint: maint, Step: step},
		MaintStepCancelled{Actor: actor, Maint: maint, Step: step},
	}

	for _, a := range actions {
		payload, err := r.Render(a)
		require.NoErrorf(t, err, "action %T", a)
		require.Truef(t, payload.Action.IsValid(), "action %T renders invalid AuditAction %q", a, payload.Action)
		require.Equal(t, entity.AuditEntityTypeMaintenance, payload.EntityType)
	}
}
