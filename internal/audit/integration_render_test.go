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
		name       string
		action     Action
		wantAction entity.AuditAction
	}{
		{"created", IntegrationCreated{Actor: actor, Kind: "slack", Enabled: true}, entity.AuditActionIntegrationCreated},
		{"updated", IntegrationUpdated{Actor: actor, Kind: "slack", Enabled: false}, entity.AuditActionIntegrationUpdated},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload, err := r.Render(tc.action)
			require.NoError(t, err)

			require.Equal(t, tc.wantAction, payload.Action)
			require.Equal(t, entity.AuditEntityTypeIntegration, payload.EntityType)
			require.Equal(t, "slack", payload.EntityID)
			require.Equal(t, actor.Email, payload.Actor)
			require.Contains(t, payload.Details, "slack")
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
