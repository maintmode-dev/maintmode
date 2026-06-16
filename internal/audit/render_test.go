package audit

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// fixedRenderer is a Renderer with a deterministic clock and id source so the
// rendered occurred_at / event_id are assertable.
func fixedRenderer(id uuid.UUID, now time.Time) *Renderer {
	return &Renderer{
		now:   func() time.Time { return now },
		newID: func() uuid.UUID { return id },
	}
}

func TestRender_StampsEventIDAndOccurredAt(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	r := fixedRenderer(id, now)

	actor := &entity.User{ID: uuid.New(), Email: "a@example.com", Name: "Alice"}

	payload, err := r.Render(LoginSuccess{User: actor})
	require.NoError(t, err)

	require.Equal(t, id, payload.EventID)
	require.Equal(t, now, payload.OccurredAt)
}

func TestRender_MapsEveryAction(t *testing.T) {
	actor := &entity.User{ID: uuid.New(), Email: "actor@example.com", Name: "Actor"}
	target := &entity.User{ID: uuid.New(), Email: "target@example.com", Name: "Target"}
	r := fixedRenderer(uuid.New(), time.Now())

	tests := []struct {
		name             string
		action           Action
		wantAction       entity.AuditAction
		wantActor        string
		wantEntityID     string
		wantDetailsParts []string
		checkMetadata    func(t *testing.T, m *entity.AuditMetadata)
	}{
		{
			name:             "login success",
			action:           LoginSuccess{User: actor, Meta: &entity.AuditMetadata{IP: "1.2.3.4"}},
			wantAction:       entity.AuditActionLoginSuccess,
			wantActor:        actor.Email,
			wantEntityID:     actor.ID.String(),
			wantDetailsParts: []string{"login success", actor.Email},
			checkMetadata: func(t *testing.T, m *entity.AuditMetadata) {
				t.Helper()
				require.Equal(t, "1.2.3.4", m.IP)
			},
		},
		{
			name:             "login failed",
			action:           LoginFailed{User: actor, Meta: &entity.AuditMetadata{FailureReason: entity.AuditFailureTokenIssuance}},
			wantAction:       entity.AuditActionLoginFailed,
			wantActor:        actor.Email,
			wantEntityID:     actor.ID.String(),
			wantDetailsParts: []string{"login failed", actor.Email},
			checkMetadata: func(t *testing.T, m *entity.AuditMetadata) {
				t.Helper()
				require.Equal(t, entity.AuditFailureTokenIssuance, m.FailureReason)
			},
		},
		{
			name:             "logout success",
			action:           LogoutSuccess{User: actor, SessionID: "sess-1"},
			wantAction:       entity.AuditActionLogoutSuccess,
			wantActor:        actor.Email,
			wantEntityID:     actor.ID.String(),
			wantDetailsParts: []string{"logout success", actor.Email},
			checkMetadata: func(t *testing.T, m *entity.AuditMetadata) {
				t.Helper()
				require.Equal(t, "sess-1", m.SessionID)
				require.EqualValues(t, entity.AuditLogoutKindManual, m.LogoutKind)
			},
		},
		{
			name: "roles changed",
			action: RolesChanged{
				Actor: actor, Target: target, Kind: RolesAssigned,
				Change: RolesChange{Roles: []entity.Role{entity.RoleAdmin}},
			},
			wantAction:       entity.AuditActionRolesChanged,
			wantActor:        actor.Email,
			wantEntityID:     target.ID.String(),
			wantDetailsParts: []string{target.Email},
			checkMetadata: func(t *testing.T, m *entity.AuditMetadata) {
				t.Helper()
				require.Equal(t, []string{string(entity.RoleAdmin)}, m.Roles)
				require.Equal(t, target.Email, m.TargetEmail)
			},
		},
		{
			name:             "user blocked",
			action:           UserBlocked{Actor: actor, Target: target},
			wantAction:       entity.AuditActionUserBlocked,
			wantActor:        actor.Email,
			wantEntityID:     target.ID.String(),
			wantDetailsParts: []string{target.Email, "blocked", actor.Email},
			checkMetadata: func(t *testing.T, m *entity.AuditMetadata) {
				t.Helper()
				require.Equal(t, target.Email, m.TargetEmail)
			},
		},
		{
			name:             "user unblocked",
			action:           UserUnblocked{Actor: actor, Target: target},
			wantAction:       entity.AuditActionUserUnblocked,
			wantActor:        actor.Email,
			wantEntityID:     target.ID.String(),
			wantDetailsParts: []string{target.Email, "unblocked", actor.Email},
			checkMetadata: func(t *testing.T, m *entity.AuditMetadata) {
				t.Helper()
				require.Equal(t, target.Email, m.TargetEmail)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := r.Render(tc.action)
			require.NoError(t, err)

			require.Equal(t, tc.wantAction, payload.Action)
			require.Equal(t, tc.wantActor, payload.Actor)
			require.Equal(t, entity.AuditEntityTypeUser, payload.EntityType)
			require.Equal(t, tc.wantEntityID, payload.EntityID)
			for _, part := range tc.wantDetailsParts {
				require.Contains(t, payload.Details, part)
			}
			require.NotNil(t, payload.Metadata)
			tc.checkMetadata(t, payload.Metadata)
		})
	}
}

func TestRender_UnsupportedAction(t *testing.T) {
	_, err := NewRenderer().Render(unknownAction{})
	require.ErrorIs(t, err, apperr.ErrUnsupportedEvent)
}

func TestRender_TruncatesUserAgent(t *testing.T) {
	long := strings.Repeat("x", maxUserAgentLen+50)

	payload, err := NewRenderer().Render(LoginSuccess{
		User: &entity.User{ID: uuid.New(), Email: "a@example.com"},
		Meta: &entity.AuditMetadata{UserAgent: long},
	})
	require.NoError(t, err)

	require.Len(t, []rune(payload.Metadata.UserAgent), maxUserAgentLen)
}

type unknownAction struct{}

func (unknownAction) auditAction() entity.AuditAction { return "unknown" }
