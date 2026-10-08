package audit

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestRender_IntegrationActions(t *testing.T) {
	t.Parallel()
	actor := &entity.User{ID: uuid.New(), Email: "admin@example.com", Name: "Admin"}
	r := fixedRenderer(uuid.New(), time.Now())

	tests := []struct {
		name         string
		action       Action
		wantAction   entity.AuditAction
		wantEntityID string
		wantInDetail string
	}{
		{
			"created", IntegrationCreated{Actor: actor, Kind: "slack", Name: "default", Enabled: true},
			entity.AuditActionIntegrationCreated, "slack/default", "slack",
		},
		{
			"updated", IntegrationUpdated{Actor: actor, Kind: "slack", Name: "default", Enabled: false},
			entity.AuditActionIntegrationUpdated, "slack/default", "slack",
		},
		{
			"deleted", IntegrationDeleted{Actor: actor, Kind: "slack", Name: "default"},
			entity.AuditActionIntegrationDeleted, "slack/default", "slack",
		},
		// Two instances of one kind must be distinguishable in the trail; before
		// the name was recorded, both of these rendered identically.
		{
			"named instance", IntegrationUpdated{Actor: actor, Kind: "oidc", Name: "keycloak", Enabled: true},
			entity.AuditActionIntegrationUpdated, "oidc/keycloak", "keycloak",
		},
		// Entries written before instances existed keep the id they have, so a
		// trail spanning the change stays comparable.
		{
			"absent name keeps the bare kind", IntegrationUpdated{Actor: actor, Kind: "slack", Enabled: true},
			entity.AuditActionIntegrationUpdated, "slack", "slack",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload, err := r.Render(tc.action)
			require.NoError(t, err)

			require.Equal(t, tc.wantAction, payload.Action)
			require.Equal(t, entity.AuditEntityTypeIntegration, payload.EntityType)
			require.Equal(t, tc.wantEntityID, payload.EntityID)
			require.Equal(t, actor.Email, payload.Actor)
			require.Contains(t, payload.Details, tc.wantInDetail)
		})
	}
}

// TestRender_IntegrationPayloadHasNoSecret guards against future drift: even if a
// field is added to the integration action or its metadata, the *rendered*
// payload must never carry a secret value.
func TestRender_IntegrationPayloadHasNoSecret(t *testing.T) {
	t.Parallel()
	actor := &entity.User{ID: uuid.New(), Email: "admin@example.com", Name: "Admin"}
	r := fixedRenderer(uuid.New(), time.Now())

	payload, err := r.Render(IntegrationCreated{Actor: actor, Kind: "slack", Enabled: true})
	require.NoError(t, err)

	// The action carries only kind/enabled/actor; assert the whole rendered
	// payload (details + metadata) contains no secret-looking material.
	rendered := fmt.Sprintf("%+v", payload)
	require.NotContains(t, rendered, "xoxb-")
	require.NotContains(t, rendered, "bot_token")
}

// An update's changes reach the record's metadata, so the trail can answer
// "who moved this integration, from where, to where".
func TestRender_IntegrationUpdatedCarriesChanges(t *testing.T) {
	t.Parallel()
	actor := &entity.User{ID: uuid.New(), Email: "admin@example.com", Name: "Admin"}
	r := fixedRenderer(uuid.New(), time.Now())

	changes := []entity.AuditFieldChange{
		{Field: "api_url", Old: "https://a.test", New: "https://b.test"},
		{Field: "secrets.bot_token"},
	}
	payload, err := r.Render(IntegrationUpdated{Actor: actor, Kind: "notify", Name: "slack", Enabled: true, Changes: changes})
	require.NoError(t, err)
	require.NotNil(t, payload.Metadata)
	require.Equal(t, changes, payload.Metadata.Changes)

	// A toggle carries no changes and no metadata, as before.
	toggled, err := r.Render(IntegrationUpdated{Actor: actor, Kind: "notify", Name: "slack", Enabled: false})
	require.NoError(t, err)
	require.Nil(t, toggled.Metadata)
}
