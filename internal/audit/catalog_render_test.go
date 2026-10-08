package audit

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestRender_InvitationActions(t *testing.T) {
	t.Parallel()
	actor := &entity.User{ID: uuid.New(), Email: "admin@example.com", Name: "Admin"}
	inv := &entity.Invitation{
		ID:        uuid.New(),
		Email:     "new.hire@example.com",
		Roles:     []entity.Role{"editor"},
		TokenHash: "hash-that-must-not-be-recorded",
	}
	r := fixedRenderer(uuid.New(), time.Now())

	for _, tc := range []struct {
		action Action
		want   entity.AuditAction
		verb   string
	}{
		{InvitationCreated{Actor: actor, Invitation: inv}, "invitation.created", "created"},
		{InvitationRevoked{Actor: actor, Invitation: inv}, "invitation.revoked", "revoked"},
	} {
		t.Run(string(tc.want), func(t *testing.T) {
			t.Parallel()
			payload, err := r.Render(tc.action)
			require.NoError(t, err)

			require.Equal(t, tc.want, payload.Action)
			require.Equal(t, entity.AuditEntityType("invitation"), payload.EntityType)
			require.Equal(t, inv.ID.String(), payload.EntityID)
			require.Equal(t, actor.Email, payload.Actor)
			require.Equal(t, actor.ID.String(), payload.ActorID)
			require.Equal(t, "invitation for new.hire@example.com "+tc.verb+" by admin@example.com", payload.Details)
			require.Equal(t, &entity.AuditMetadata{
				Roles:       []string{"editor"},
				TargetEmail: "new.hire@example.com",
			}, payload.Metadata)
			require.NotContains(t, fmt.Sprintf("%+v", payload), inv.TokenHash, "the link token must never reach the trail")
		})
	}
}

func TestRender_ResourceActions(t *testing.T) {
	t.Parallel()
	actor := &entity.User{ID: uuid.New(), Email: "admin@example.com", Name: "Admin"}
	resource := &entity.ResourceDetails{ID: uuid.New(), Name: "payments-db"}
	changes := []entity.AuditFieldChange{{Field: "name", Old: "pay-db", New: "payments-db"}}
	r := fixedRenderer(uuid.New(), time.Now())

	for _, tc := range []struct {
		action      Action
		want        entity.AuditAction
		verb        string
		wantChanges []entity.AuditFieldChange
	}{
		{ResourceCreated{Actor: actor, Resource: resource}, "resource.created", "created", nil},
		{ResourceUpdated{Actor: actor, Resource: resource, Changes: changes}, "resource.updated", "updated", changes},
		{ResourceArchived{Actor: actor, Resource: resource}, "resource.archived", "archived", nil},
		{ResourceUnarchived{Actor: actor, Resource: resource}, "resource.unarchived", "unarchived", nil},
	} {
		t.Run(string(tc.want), func(t *testing.T) {
			t.Parallel()
			payload, err := r.Render(tc.action)
			require.NoError(t, err)

			require.Equal(t, tc.want, payload.Action)
			require.Equal(t, entity.AuditEntityType("resource"), payload.EntityType)
			require.Equal(t, resource.ID.String(), payload.EntityID)
			require.Equal(t, actor.Email, payload.Actor)
			require.Equal(t, `resource "payments-db" `+tc.verb+" by admin@example.com", payload.Details)
			require.Equal(t, &entity.AuditMetadata{TargetDisplayName: "payments-db", Changes: tc.wantChanges}, payload.Metadata)
		})
	}
}

func TestRender_NotifyChannelActions(t *testing.T) {
	t.Parallel()
	actor := &entity.User{ID: uuid.New(), Email: "admin@example.com", Name: "Admin"}
	channel := &entity.NotifyChannel{ID: uuid.New(), Transport: "slack", Name: "#ops", TransportChannelID: "C123"}
	changes := []entity.AuditFieldChange{{Field: "transport_channel_id", Old: "C000", New: "C123"}}
	r := fixedRenderer(uuid.New(), time.Now())

	for _, tc := range []struct {
		action      Action
		want        entity.AuditAction
		verb        string
		wantChanges []entity.AuditFieldChange
	}{
		{NotifyChannelCreated{Actor: actor, Channel: channel}, "notify_channel.created", "created", nil},
		{NotifyChannelUpdated{Actor: actor, Channel: channel, Changes: changes}, "notify_channel.updated", "updated", changes},
		{NotifyChannelArchived{Actor: actor, Channel: channel}, "notify_channel.archived", "archived", nil},
		{NotifyChannelUnarchived{Actor: actor, Channel: channel}, "notify_channel.unarchived", "unarchived", nil},
	} {
		t.Run(string(tc.want), func(t *testing.T) {
			t.Parallel()
			payload, err := r.Render(tc.action)
			require.NoError(t, err)

			require.Equal(t, tc.want, payload.Action)
			require.Equal(t, entity.AuditEntityType("notify_channel"), payload.EntityType)
			require.Equal(t, channel.ID.String(), payload.EntityID)
			require.Equal(t, actor.Email, payload.Actor)
			require.Equal(t, `slack channel "#ops" `+tc.verb+" by admin@example.com", payload.Details)
			require.Equal(t, &entity.AuditMetadata{TargetDisplayName: "#ops", Changes: tc.wantChanges}, payload.Metadata)
		})
	}
}
