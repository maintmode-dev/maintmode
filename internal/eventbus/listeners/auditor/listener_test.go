package auditor

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/eventbus/events"
)

// fakeWriter captures the AuditEntry Handle produces, without touching the DB.
type fakeWriter struct {
	entry *entity.AuditEntry
}

func (f *fakeWriter) AddLog(_ context.Context, entry *entity.AuditEntry) error {
	f.entry = entry
	return nil
}

func handle(t *testing.T, event interface {
	EventType() events.EventType
},
) *entity.AuditEntry {
	t.Helper()
	w := &fakeWriter{}
	require.NoError(t, NewListener(w).Handle(context.Background(), event))
	require.NotNil(t, w.entry)
	return w.entry
}

func TestHandle_LoginSuccess_TruncatesUserAgent(t *testing.T) {
	t.Parallel()

	longUA := strings.Repeat("x", maxUserAgentLen+50)
	user := &entity.User{ID: uuid.New(), Email: "u@example.com", Name: "User"}

	entry := handle(t, events.UserLoginSuccess{
		User: user,
		Meta: &entity.AuditMetadata{UserAgent: longUA},
	})

	require.Equal(t, entity.AuditActionLoginSuccess, entry.Action)
	require.Equal(t, user.Email, entry.Actor)
	require.Equal(t, user.ID.String(), entry.ActorID)
	require.Equal(t, entity.AuditEntityTypeUser, entry.EntityType)
	require.Equal(t, user.ID.String(), entry.EntityID)
	require.Len(t, []rune(entry.Metadata.UserAgent), maxUserAgentLen)
}

func TestHandle_LoginFailed_PseudoUserHasNilActorID(t *testing.T) {
	t.Parallel()

	// login failed builds a pseudo-user with only Email/Name (no ID), so
	// ActorID is the nil-UUID string.
	entry := handle(t, events.UserLoginFailed{
		User: &entity.User{Email: "ghost@example.com"},
		Meta: &entity.AuditMetadata{FailureReason: entity.AuditFailureTokenIssuance},
	})

	require.Equal(t, entity.AuditActionLoginFailed, entry.Action)
	require.Equal(t, "ghost@example.com", entry.Actor)
	require.Equal(t, uuid.Nil.String(), entry.ActorID)
	require.Equal(t, entity.AuditFailureTokenIssuance, entry.Metadata.FailureReason)
}

func TestHandle_LogoutSuccess(t *testing.T) {
	t.Parallel()

	user := &entity.User{ID: uuid.New(), Email: "u@example.com"}

	entry := handle(t, events.UserLogoutSuccess{User: user, SessionID: "session-123"})

	require.Equal(t, entity.AuditActionLogoutSuccess, entry.Action)
	require.Equal(t, entity.AuditEntityTypeUser, entry.EntityType)
	require.Equal(t, user.ID.String(), entry.EntityID)
	require.Equal(t, "session-123", entry.Metadata.SessionID)
	require.Equal(t, entity.AuditLogoutKind(entity.AuditLogoutKindManual), entry.Metadata.LogoutKind)
}

func TestHandle_RolesChanged(t *testing.T) {
	t.Parallel()

	actor := &entity.User{ID: uuid.New(), Email: "admin@example.com", Name: "Admin"}
	target := &entity.User{ID: uuid.New(), Email: "target@example.com", Name: "Target"}

	entry := handle(t, events.UserRolesChanged{
		Actor:  actor,
		Target: target,
		Kind:   events.RolesReplaced,
		Change: events.AuditRolesChange{
			Roles:   []entity.Role{entity.RoleEditor},
			Added:   []entity.Role{entity.RoleEditor},
			Removed: []entity.Role{entity.RoleAdmin},
		},
	})

	require.Equal(t, entity.AuditActionRolesChanged, entry.Action)
	require.Equal(t, actor.ID.String(), entry.ActorID)
	require.Equal(t, entity.AuditEntityTypeUser, entry.EntityType)
	require.Equal(t, target.ID.String(), entry.EntityID)
	require.Equal(t, []string{string(entity.RoleEditor)}, entry.Metadata.Roles)
	require.Equal(t, []string{string(entity.RoleEditor)}, entry.Metadata.RolesAdded)
	require.Equal(t, []string{string(entity.RoleAdmin)}, entry.Metadata.RolesRemoved)
	require.Equal(t, target.Email, entry.Metadata.TargetEmail)
}

func TestHandle_Blocked(t *testing.T) {
	t.Parallel()

	actor := &entity.User{ID: uuid.New(), Email: "admin@example.com", Name: "Admin"}
	target := &entity.User{ID: uuid.New(), Email: "target@example.com", Name: "Target"}

	entry := handle(t, events.UserBlocked{Actor: actor, Target: target})

	require.Equal(t, entity.AuditActionUserBlocked, entry.Action)
	require.Equal(t, actor.Email, entry.Actor)
	require.Equal(t, entity.AuditEntityTypeUser, entry.EntityType)
	require.Equal(t, target.ID.String(), entry.EntityID)
	require.Equal(t, target.Email, entry.Metadata.TargetEmail)
}
